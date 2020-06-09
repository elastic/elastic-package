package cluster

import "github.com/pkg/errors"

func BootUp() error {
	buildPackagesPath, found, err := findBuildPackagesDirectory()
	if err != nil {
		return errors.Wrap(err, "finding build packages directory failed")
	}

	if found {
		err := writeEnvFile(buildPackagesPath)
		if err != nil {
			return errors.Wrapf(err, "writing .env file failed (packagesPath: %s)", buildPackagesPath)
		}
	}

	err = runDockerCompose()
	if err != nil {
		return errors.Wrap(err, "running docker-compose failed")
	}
	return nil // TODO
}

func findBuildPackagesDirectory() (string, bool, error) {
	panic("TODO")
}

func writeEnvFile(buildPackagesPath string) error {
	panic("TODO")
}

func runDockerCompose() error {
	panic("TODO")
}
