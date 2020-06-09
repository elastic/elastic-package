package cluster

import (
	"fmt"
	"io/ioutil"
	"path/filepath"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/install"
)

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

	err = runDockerCompose(found)
	if err != nil {
		return errors.Wrap(err, "running docker-compose failed")
	}
	return nil // TODO
}

func findBuildPackagesDirectory() (string, bool, error) {
	panic("TODO")
}

func writeEnvFile(buildPackagesPath string) error {
	envFile := fmt.Sprintf("PACKAGES_PATH=%s\n", buildPackagesPath)

	clusterDir, err := install.ClusterDir()
	if err != nil {
		return errors.Wrap(err, "locating cluster directory failed")
	}

	envFilePath := filepath.Join(clusterDir, ".env")
	err = ioutil.WriteFile(envFilePath, []byte(envFile), 0644)
	if err != nil {
		return errors.Wrapf(err, "writing .env file failed (path: %s)", envFilePath)
	}
	return nil
}

func runDockerCompose(useCustomPackagesPath bool) error {
	panic("TODO")
}
