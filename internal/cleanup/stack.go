// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cleanup

import (
	"os"
	"path/filepath"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/configuration/locations"
	"github.com/elastic/elastic-package/internal/packages"

	"github.com/elastic/elastic-package/internal/logger"
)

// Stack function removes built package used by the Package Registry image.
func Stack() (string, error) {
	logger.Debug("Clean built packages from the development stack")

	packageRoot, err := packages.MustFindPackageRoot()
	if err != nil {
		return "", errors.Wrap(err, "locating package root failed")
	}

	m, err := packages.ReadPackageManifestFromPackageRoot(packageRoot)
	if err != nil {
		return "", errors.Wrapf(err, "reading package manifest failed (path: %s)", packageRoot)
	}

	locationManager, err := locations.NewLocationManager()
	if err != nil {
		return "", errors.Wrap(err, "can't find stack packages dir")
	}
	destinationDir := filepath.Join(locationManager.PackagesDir(), m.Name)

	_, err = os.Stat(destinationDir)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", errors.Wrapf(err, "stat file failed: %s", destinationDir)
	}
	if errors.Is(err, os.ErrNotExist) {
		logger.Debugf("Stack package is not part of the development stack (missing path: %s)", destinationDir)
		return "", nil
	}

	logger.Debugf("Remove folder (path: %s)", destinationDir)
	err = os.RemoveAll(destinationDir)
	if err != nil {
		return "", errors.Wrapf(err, "can't remove directory (path: %s)", destinationDir)
	}
	return destinationDir, nil
}
