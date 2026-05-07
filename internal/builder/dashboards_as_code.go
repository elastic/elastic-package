// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package builder

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Masterminds/semver/v3"

	"github.com/elastic/elastic-package/internal/export"
	"github.com/elastic/elastic-package/internal/kibana"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/packages"
)

const dashboardsAsCodeDir = "_dev/build/dashboards_as_code"

// minDashboardsAsCodeKibanaVersion is the first Kibana version that supports
// the dashboards-as-code import API (POST /api/dashboards).
var minDashboardsAsCodeKibanaVersion = semver.MustParse("9.4.0")

// compileDashboardsAsCode compiles each *.json file under
// <sourcePackageRoot>/_dev/dashboards_as_code/ into a saved-object dashboard
// under <sourcePackageRoot>/kibana/dashboard/. Each source file is imported
// into the connected Kibana via POST /api/dashboards, the resulting dashboard
// is exported back through the standard dashboards export pipeline, and the
// imported saved object is then deleted from Kibana.
//
// If the source directory does not exist or contains no JSON files, this
// function is a no-op and no Kibana connection is attempted. When source
// files are present and kibanaClient is nil, returns an error.
func compileDashboardsAsCode(ctx context.Context, kibanaClient *kibana.Client, sourcePackageRoot string) error {
	sourceDir := filepath.Join(sourcePackageRoot, dashboardsAsCodeDir)
	files, err := filepath.Glob(filepath.Join(sourceDir, "*.json"))
	if err != nil {
		return fmt.Errorf("listing dashboards-as-code sources failed: %w", err)
	}
	if len(files) == 0 {
		return nil
	}

	if kibanaClient == nil {
		return fmt.Errorf("package contains %s but no Kibana client is configured; "+
			"set ELASTIC_PACKAGE_KIBANA_HOST or run 'elastic-package stack up' first", dashboardsAsCodeDir)
	}

	versionInfo, err := kibanaClient.Version()
	if err != nil {
		return fmt.Errorf("getting Kibana version information: %w", err)
	}
	if err := checkDashboardsAsCodeKibanaVersion(versionInfo); err != nil {
		return err
	}

	manifest, err := packages.ReadPackageManifestFromPackageRoot(sourcePackageRoot)
	if err != nil {
		return fmt.Errorf("reading package manifest failed (path: %s): %w", sourcePackageRoot, err)
	}

	for _, file := range files {
		if err := compileDashboardAsCodeFile(ctx, kibanaClient, manifest.Name, sourcePackageRoot, file); err != nil {
			return fmt.Errorf("compiling dashboards-as-code file %s: %w", file, err)
		}
	}
	return nil
}

func checkDashboardsAsCodeKibanaVersion(info kibana.VersionInfo) error {
	v, err := semver.NewVersion(info.Number)
	if err != nil {
		return fmt.Errorf("cannot parse Kibana version %s: %w", info.Number, err)
	}
	if v.LessThan(minDashboardsAsCodeKibanaVersion) {
		return fmt.Errorf("dashboards-as-code requires Kibana %s or later (got %s); the import API at POST /api/dashboards is not available in this version",
			minDashboardsAsCodeKibanaVersion, info.Number)
	}
	return nil
}

func compileDashboardAsCodeFile(ctx context.Context, kibanaClient *kibana.Client, packageName, sourcePackageRoot, file string) error {
	logger.Debugf("Compiling dashboards-as-code file: %s", file)

	body, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("reading dashboards-as-code source failed: %w", err)
	}

	// Use the source filename (without extension) as the saved-object id so the
	// compiled output is deterministic. standardizeObjectID will then prefix it
	// with the package name during export.
	id := strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))
	id, err = kibanaClient.ImportDashboardAsCode(ctx, id, body)
	if err != nil {
		return fmt.Errorf("importing dashboards-as-code failed: %w", err)
	}

	// Best-effort cleanup of the imported dashboard, regardless of how the
	// rest of this function completes. Use a fresh context so cleanup runs
	// even if ctx has been cancelled by the time we reach this point.
	defer func() {
		if cleanupErr := kibanaClient.DeleteDashboard(context.Background(), id); cleanupErr != nil {
			if errors.Is(cleanupErr, context.Canceled) {
				return
			}
			logger.Debugf("Failed to delete imported dashboard %s during cleanup: %v", id, cleanupErr)
		}
	}()

	objects, err := kibanaClient.Export(ctx, []string{id})
	if err != nil {
		return fmt.Errorf("exporting dashboard %s failed: %w", id, err)
	}

	if err := export.TransformAndWriteDashboards(sourcePackageRoot, packageName, objects); err != nil {
		return fmt.Errorf("writing exported dashboard %s failed: %w", id, err)
	}
	return nil
}
