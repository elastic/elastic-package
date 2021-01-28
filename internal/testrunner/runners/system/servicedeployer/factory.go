// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package servicedeployer

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/logger"
)

const devDeployDir = "_dev/deploy"

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
	devDeployPath, err := findDevDeployPath(options)
	if err != nil {
		logger.Errorf("can't find \"%s\" directory", devDeployDir)
		return nil, ErrNotFound
	}

	// Is the service defined using a docker compose configuration file?
	dockerComposeYMLPath := filepath.Join(devDeployPath, "docker", "docker-compose.yml")
	if _, err := os.Stat(dockerComposeYMLPath); err == nil {
		return NewDockerComposeServiceDeployer(dockerComposeYMLPath)
	}

	// Is the service defined using Terraform definition files?
	terraformDirPath := filepath.Join(devDeployPath, "tf")
	if _, err := os.Stat(terraformDirPath); err == nil {
		return NewTerraformServiceDeployer(terraformDirPath)
	}
	return nil, ErrNotFound
}

func findDevDeployPath(options FactoryOptions) (string, error) {
	dataStreamDevDeployPath := filepath.Join(options.DataStreamRootPath, devDeployDir)
	_, err := os.Stat(dataStreamDevDeployPath)
	if err == nil {
		return dataStreamDevDeployPath, nil
	} else if !os.IsNotExist(err) {
		return "", errors.Wrapf(err, "stat failed for data stream (path: %s)", dataStreamDevDeployPath)
	}

	packageDevDeployPath := filepath.Join(options.PackageRootPath, devDeployDir)
	_, err = os.Stat(packageDevDeployPath)
	if err == nil {
		return packageDevDeployPath, nil
	} else if !os.IsNotExist(err) {
		return "", errors.Wrapf(err, "stat failed for package (path: %s)", packageDevDeployPath)
	}
	return "", fmt.Errorf("\"%s\" directory doesn't exist", devDeployDir)
}
