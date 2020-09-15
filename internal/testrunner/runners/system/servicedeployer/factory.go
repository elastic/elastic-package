// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package servicedeployer

import (
	"errors"
	"os"
	"path/filepath"
)

var (
	// ErrNotFound is returned when the appropriate service runner for a package
	// cannot be found.
	ErrNotFound = errors.New("unable to find service runner")
)

// Factory chooses the appropriate service runner for the given package, depending
// on service configuration files defined in the package.
func Factory(packageRootPath string) (ServiceDeployer, error) {
	packageDevPath := filepath.Join(packageRootPath, "_dev")

	// Is the service defined using a docker compose configuration file?
	dockerComposeYMLPath := filepath.Join(packageDevPath, "deploy", "docker-compose.yml")
	if _, err := os.Stat(dockerComposeYMLPath); err == nil {
		return NewDockerComposeServiceDeployer(dockerComposeYMLPath)
	}

	return nil, ErrNotFound
}
