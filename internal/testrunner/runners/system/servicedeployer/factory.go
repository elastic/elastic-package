package servicedeployer

import (
	"errors"
	"os"
	"path"
)

var (
	// ErrNotFound is returned when the appropriate service runner for a package
	// cannot be found.
	ErrNotFound = errors.New("unable to find service runner")
)

// Factory chooses the appropriate service runner for the given package, depending
// on service configuration files defined in the package.
func Factory(packageRootPath string) (ServiceDeployer, error) {
	packageDevPath := path.Join(packageRootPath, "_dev")

	// Is the service defined using a docker compose configuration file?
	dockerComposeYMLPath := path.Join(packageDevPath, "docker-compose.yml")
	if _, err := os.Stat(dockerComposeYMLPath); err == nil {
		return NewDockerComposeServiceDeployer(dockerComposeYMLPath)
	}

	return nil, ErrNotFound
}
