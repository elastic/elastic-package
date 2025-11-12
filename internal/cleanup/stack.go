// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cleanup

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/elastic/elastic-package/internal/configuration/locations"
	"github.com/elastic/elastic-package/internal/packages"

	"github.com/elastic/elastic-package/internal/logger"
)

// Stack function removes built package used by the Package Registry image.
func Stack(workDir string) (string, error) {
	logger.Debug("Clean built packages from the development stack")

	packageRoot, err := packages.MustFindPackageRoot(workDir)
	if err != nil {
		return "", fmt.Errorf("locating package root failed: %w", err)
	}

	m, err := packages.ReadPackageManifestFromPackageRoot(packageRoot)
	if err != nil {
		return "", fmt.Errorf("reading package manifest failed (path: %s): %w", packageRoot, err)
	}

	locationManager, err := locations.NewLocationManager()
	if err != nil {
		return "", fmt.Errorf("can't find stack packages dir: %w", err)
	}
	destinationDir := filepath.Join(locationManager.PackagesDir(), m.Name)

	_, err = os.Stat(destinationDir)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("stat file failed: %s: %w", destinationDir, err)
	}
	if errors.Is(err, os.ErrNotExist) {
		logger.Debugf("Stack package is not part of the development stack (missing path: %s)", destinationDir)
		return "", nil
	}

	logger.Debugf("Remove folder (path: %s)", destinationDir)
	err = os.RemoveAll(destinationDir)
	if err != nil {
		return "", fmt.Errorf("can't remove directory (path: %s): %w", destinationDir, err)
	}
	return destinationDir, nil
}
