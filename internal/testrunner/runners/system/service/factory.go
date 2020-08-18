package service

import (
	"errors"
	"os"
	"path"
)

var (
	ErrNotFound = errors.New("unable to find service runner")
)

// Factory chooses the appropriate service runner for the given package, depending
// on service configuration files defined in the package.
func Factory(packageRootPath string) (Runner, error) {
	packageDevPath := path.Join(packageRootPath, "_dev")

	// Is the service defined using a docker compose configuration file?
	dockerComposeYMLPath := path.Join(packageDevPath, "docker-compose.yml")
	if _, err := os.Stat(dockerComposeYMLPath); err == nil {
		return NewDockerComposeRunner(dockerComposeYMLPath)
	}

	return nil, ErrNotFound
}
