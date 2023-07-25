// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package servicedeployer

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/elastic/elastic-package/internal/profile"
)

const devDeployDir = "_dev/benchmark/system/deploy"

// FactoryOptions defines options used to create an instance of a service deployer.
type FactoryOptions struct {
	Profile *profile.Profile

	RootPath string
}

// Factory chooses the appropriate service runner for the given data stream, depending
// on service configuration files defined in the package or data stream.
func Factory(options FactoryOptions) (ServiceDeployer, error) {
	devDeployPath, err := FindDevDeployPath(options)
	if err != nil {
		return nil, fmt.Errorf("can't find \"%s\" directory: %w", devDeployDir, err)
	}

	serviceDeployerName, err := findServiceDeployer(devDeployPath)
	if err != nil {
		return nil, fmt.Errorf("can't find any valid service deployer: %w", err)
	}

	serviceDeployerPath := filepath.Join(devDeployPath, serviceDeployerName)

	switch serviceDeployerName {
	case "docker":
		dockerComposeYMLPath := filepath.Join(serviceDeployerPath, "docker-compose.yml")
		if _, err := os.Stat(dockerComposeYMLPath); err == nil {
			return NewDockerComposeServiceDeployer(options.Profile, []string{dockerComposeYMLPath})
		}
	}
	return nil, fmt.Errorf("unsupported service deployer (name: %s)", serviceDeployerName)
}

// FindDevDeployPath function returns a path reference to the "_dev/deploy" directory.
func FindDevDeployPath(options FactoryOptions) (string, error) {
	path := filepath.Join(options.RootPath, devDeployDir)
	if _, err := os.Stat(path); err == nil {
		return path, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("stat failed for path (path: %s): %w", path, err)
	}
	return "", fmt.Errorf("\"%s\" directory doesn't exist", devDeployDir)
}

func findServiceDeployer(devDeployPath string) (string, error) {
	fis, err := os.ReadDir(devDeployPath)
	if err != nil {
		return "", fmt.Errorf("can't read directory (path: %s): %w", devDeployDir, err)
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
