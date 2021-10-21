// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cleanup

import (
	"os"
	"path/filepath"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/builder"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/packages"
)

// Build function removes package resources from build/.
func Build() (string, error) {
	logger.Debug("Clean build resources")

	packageRoot, err := packages.MustFindPackageRoot()
	if err != nil {
		return "", errors.Wrap(err, "locating package root failed")
	}

	m, err := packages.ReadPackageManifestFromPackageRoot(packageRoot)
	if err != nil {
		return "", errors.Wrapf(err, "reading package manifest failed (path: %s)", packageRoot)
	}

	buildDir, found, err := builder.FindBuildPackagesDirectory()
	if err != nil {
		return "", errors.Wrap(err, "locating build directory failed")
	}

	if !found {
		logger.Debug("Build directory doesn't exist")
		return "", nil
	}

	destinationDir := filepath.Join(buildDir, m.Name)
	logger.Debugf("Build directory for integration: %s\n", destinationDir)

	_, err = os.Stat(destinationDir)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", errors.Wrapf(err, "stat file failed: %s", destinationDir)
	}
	if errors.Is(err, os.ErrNotExist) {
		logger.Debugf("Package hasn't been built (missing path: %s)", destinationDir)
		return "", nil
	}

	logger.Debugf("Remove directory (path: %s)", destinationDir)
	err = os.RemoveAll(destinationDir)
	if err != nil {
		return "", errors.Wrapf(err, "can't remove directory (path: %s)", destinationDir)
	}

	zippedBuildPackagePath := builder.ZippedBuiltPackagePath(buildDir, *m)
	logger.Debugf("Remove zipped built package (path: %s)", zippedBuildPackagePath)
	err = os.RemoveAll(zippedBuildPackagePath)
	if err != nil {
		return "", errors.Wrapf(err, "can't remove zipped built package (path: %s)", zippedBuildPackagePath)
	}
	return destinationDir, nil
}
