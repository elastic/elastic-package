// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cleanup

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/elastic/elastic-package/internal/builder"
	"github.com/elastic/elastic-package/internal/packages"
)

// Build function removes package resources from build/.
func Build(logger *slog.Logger) (string, error) {
	logger.Debug("Clean build resources")

	packageRoot, err := packages.MustFindPackageRoot()
	if err != nil {
		return "", fmt.Errorf("locating package root failed: %w", err)
	}

	m, err := packages.ReadPackageManifestFromPackageRoot(packageRoot)
	if err != nil {
		return "", fmt.Errorf("reading package manifest failed (path: %s): %w", packageRoot, err)
	}

	buildDir, found, err := builder.FindBuildPackagesDirectory()
	if err != nil {
		return "", fmt.Errorf("locating build directory failed: %w", err)
	}

	if !found {
		logger.Debug("Build directory doesn't exist")
		return "", nil
	}

	destinationDir := filepath.Join(buildDir, m.Name)
	logger.Debug("Build directory for integration", slog.String("path", destinationDir))

	_, err = os.Stat(destinationDir)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("stat file failed: %s: %w", destinationDir, err)
	}
	if errors.Is(err, os.ErrNotExist) {
		logger.Debug("Package hasn't been built (missing path)", slog.String("path", destinationDir))
		return "", nil
	}

	logger.Debug("Remove directory", slog.String("path", destinationDir))
	err = os.RemoveAll(destinationDir)
	if err != nil {
		return "", fmt.Errorf("can't remove directory (path: %s): %w", destinationDir, err)
	}

	zippedBuildPackagePath := builder.ZippedBuiltPackagePath(buildDir, *m)
	logger.Debug("Remove zipped built package", slog.String("path", zippedBuildPackagePath))
	err = os.RemoveAll(zippedBuildPackagePath)
	if err != nil {
		return "", fmt.Errorf("can't remove zipped built package (path: %s): %w", zippedBuildPackagePath, err)
	}
	return destinationDir, nil
}
