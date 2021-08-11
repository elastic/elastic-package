// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package servicedeployer

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
)

const devDeployDir = "_dev/deploy"

// FactoryOptions defines options used to create an instance of a service deployer.
type FactoryOptions struct {
	PackageRootPath    string
	DataStreamRootPath string

	Variant string
}

// Factory chooses the appropriate service runner for the given data stream, depending
// on service configuration files defined in the package or data stream.
func Factory(options FactoryOptions) (ServiceDeployer, error) {
	devDeployPath, err := FindDevDeployPath(options)
	if err != nil {
		return nil, errors.Wrapf(err, "can't find \"%s\" directory", devDeployDir)
	}

	serviceDeployerName, err := findServiceDeployer(devDeployPath)
	if err != nil {
		return nil, errors.Wrap(err, "can't find any valid service deployer")
	}

	serviceDeployerPath := filepath.Join(devDeployPath, serviceDeployerName)

	switch serviceDeployerName {
	case "k8s":
		if _, err := os.Stat(serviceDeployerPath); err == nil {
			return NewKubernetesServiceDeployer(serviceDeployerPath)
		}
	case "docker":
		dockerComposeYMLPath := filepath.Join(serviceDeployerPath, "docker-compose.yml")
		if _, err := os.Stat(dockerComposeYMLPath); err == nil {
			sv, err := useServiceVariant(devDeployPath, options.Variant)
			if err != nil {
				return nil, errors.Wrap(err, "can't use service variant")
			}
			return NewDockerComposeServiceDeployer([]string{dockerComposeYMLPath}, sv)
		}
	case "tf":
		if _, err := os.Stat(serviceDeployerPath); err == nil {
			return NewTerraformServiceDeployer(serviceDeployerPath)
		}
	}
	return nil, fmt.Errorf("unsupported service deployer (name: %s)", serviceDeployerName)
}

// FindDevDeployPath function returns a path reference to the "_dev/deploy" directory.
func FindDevDeployPath(options FactoryOptions) (string, error) {
	dataStreamDevDeployPath := filepath.Join(options.DataStreamRootPath, devDeployDir)
	_, err := os.Stat(dataStreamDevDeployPath)
	if err == nil {
		return dataStreamDevDeployPath, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", errors.Wrapf(err, "stat failed for data stream (path: %s)", dataStreamDevDeployPath)
	}

	packageDevDeployPath := filepath.Join(options.PackageRootPath, devDeployDir)
	_, err = os.Stat(packageDevDeployPath)
	if err == nil {
		return packageDevDeployPath, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", errors.Wrapf(err, "stat failed for package (path: %s)", packageDevDeployPath)
	}
	return "", fmt.Errorf("\"%s\" directory doesn't exist", devDeployDir)
}

func findServiceDeployer(devDeployPath string) (string, error) {
	fis, err := os.ReadDir(devDeployPath)
	if err != nil {
		return "", errors.Wrapf(err, "can't read directory (path: %s)", devDeployDir)
	}

	var folders []os.DirEntry
	for _, fi := range fis {
		if fi.IsDir() {
			folders = append(folders, fi)
		}
	}

	if len(folders) != 1 {
		return "", fmt.Errorf("expected to find only one service deployer in \"%s\"", devDeployPath)
	}
	return folders[0].Name(), nil
}
