// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package servicedeployer

import (
	"os"
	"path/filepath"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/logger"
)

const devDir = "_dev"

var (
	// ErrNotFound is returned when the appropriate service runner for a package
	// cannot be found.
	ErrNotFound = errors.New("unable to find service runner")
)

// FactoryOptions defines options used to create an instance of a service deployer.
type FactoryOptions struct {
	PackageRootPath    string
	DataStreamRootPath string
}

// Factory chooses the appropriate service runner for the given data stream, depending
// on service configuration files defined in the package or data stream.
func Factory(options FactoryOptions) (ServiceDeployer, error) {
	devPath, err := findDevPath(options)
	if err != nil {
		logger.Errorf("can't find _dev directory")
		return nil, ErrNotFound
	}

	// Is the service defined using a docker compose configuration file?
	dockerComposeYMLPath := filepath.Join(devPath, "deploy", "docker", "docker-compose.yml")
	if _, err := os.Stat(dockerComposeYMLPath); err == nil {
		return NewDockerComposeServiceDeployer(dockerComposeYMLPath)
	}

	return nil, ErrNotFound
}

func findDevPath(options FactoryOptions) (string, error) {
	dataStreamDevPath := filepath.Join(options.DataStreamRootPath, devDir)
	_, err := os.Stat(dataStreamDevPath)
	if err == nil {
		return dataStreamDevPath, nil
	} else if !os.IsNotExist(err) {
		return "", errors.Wrapf(err, "stat failed (path: %s)", dataStreamDevPath)
	}

	packageDevPath := filepath.Join(options.PackageRootPath, devDir)
	_, err = os.Stat(packageDevPath)
	if err == nil {
		return packageDevPath, nil
	} else if !os.IsNotExist(err) {
		return "", errors.Wrapf(err, "stat failed (path: %s)", packageDevPath)
	}
	return "", errors.New("_dev directory doesn't exist")
}
