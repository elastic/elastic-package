// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cleanup

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/elastic/elastic-package/internal/builder"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/packages"
)

// Build function removes package resources from build/.
func Build(workDir string) (string, error) {
	logger.Debug("Clean build resources")

	packageRoot, err := packages.MustFindPackageRoot(workDir)
	if err != nil {
		return "", fmt.Errorf("locating package root failed: %w", err)
	}

	m, err := packages.ReadPackageManifestFromPackageRoot(packageRoot)
	if err != nil {
		return "", fmt.Errorf("reading package manifest failed (path: %s): %w", packageRoot, err)
	}

	buildDir, found, err := builder.FindBuildPackagesDirectory(workDir)
	if err != nil {
		return "", fmt.Errorf("locating build directory failed: %w", err)
	}

	if !found {
		logger.Debug("Build directory doesn't exist")
		return "", nil
	}

	destinationDir := filepath.Join(buildDir, m.Name)
	logger.Debugf("Build directory for integration: %s\n", destinationDir)

	_, err = os.Stat(destinationDir)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("stat file failed: %s: %w", destinationDir, err)
	}
	if errors.Is(err, os.ErrNotExist) {
		logger.Debugf("Package hasn't been built (missing path: %s)", destinationDir)
		return "", nil
	}

	logger.Debugf("Remove directory (path: %s)", destinationDir)
	err = os.RemoveAll(destinationDir)
	if err != nil {
		return "", fmt.Errorf("can't remove directory (path: %s): %w", destinationDir, err)
	}

	zippedBuildPackagePath := builder.ZippedBuiltPackagePath(buildDir, *m)
	logger.Debugf("Remove zipped built package (path: %s)", zippedBuildPackagePath)
	err = os.RemoveAll(zippedBuildPackagePath)
	if err != nil {
		return "", fmt.Errorf("can't remove zipped built package (path: %s): %w", zippedBuildPackagePath, err)
	}
	return destinationDir, nil
}
