package install

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
)

const (
	elasticPackageDir = ".elastic-package"
	clusterDir        = "cluster"
	packagesDir       = "development"
)

func EnsureInstalled() error {
	elasticPackagePath, err := configurationDir()
	if err != nil {
		return errors.Wrap(err, "failed locating the configuration directory")
	}

	installed, err := checkIfAlreadyInstalled(elasticPackagePath)
	if installed {
		return nil
	}

	err = createElasticPackageDirectory(elasticPackagePath)
	if err != nil {
		return errors.Wrap(err, "creating elastic package directory failed")
	}

	err = writeClusterResources(elasticPackagePath)
	if err != nil {
		return errors.Wrap(err, "writing static resources failed")
	}

	fmt.Println("elastic-package has been installed.")
	return nil
}

func ClusterDir() (string, error) {
	configurationDir, err := configurationDir()
	if err != nil {
		return "", errors.Wrap(err, "locating configuration directory failed")
	}
	return filepath.Join(configurationDir, clusterDir), nil
}

func ClusterPackagesDir() (string, error) {
	clusterDir, err := ClusterDir()
	if err != nil {
		return "", errors.Wrap(err, "locating cluster directory failed")
	}
	return filepath.Join(clusterDir, packagesDir), nil
}

func configurationDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", errors.Wrap(err, "reading home dir failed")
	}
	return filepath.Join(homeDir, elasticPackageDir), nil
}

func checkIfAlreadyInstalled(elasticPackagePath string) (bool, error) {
	_, err := os.Stat(elasticPackagePath)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, errors.Wrapf(err, "stat file failed (path: %s)", elasticPackagePath)
	}
	return true, nil
}

func createElasticPackageDirectory(elasticPackagePath string) error {
	err := os.MkdirAll(elasticPackagePath, 0755)
	if err != nil {
		return errors.Wrapf(err, "creating directory failed (path: %s)", elasticPackagePath)
	}
	return nil
}

func writeClusterResources(elasticPackagePath string) error {
	clusterPath := filepath.Join(elasticPackagePath, clusterDir)
	packagesPath := filepath.Join(clusterPath, packagesDir)
	err := os.MkdirAll(packagesPath, 0755)
	if err != nil {
		return errors.Wrapf(err, "creating directory failed (path: %s)", elasticPackagePath)
	}

	err = writeStaticResource(err, clusterPath, "kibana.config.yml", kibanaConfigYml)
	err = writeStaticResource(err, clusterPath, "snapshot.yml", snapshotYml)
	err = writeStaticResource(err, clusterPath, "package-registry.config.yml", packageRegistryConfigYml)
	err = writeStaticResource(err, clusterPath, "Dockerfile.package-registry", packageRegistryDockerfile)
	if err != nil {
		return errors.Wrap(err, "writing static resource failed")
	}
	return nil
}

func writeStaticResource(err error, elasticPackagePath, filename, content string) error {
	if err != nil {
		return err
	}

	path := filepath.Join(elasticPackagePath, filename)
	err = ioutil.WriteFile(path, []byte(content), 0644)
	if err != nil {
		return errors.Wrapf(err, "writing file failed (path: %s)", path)
	}
	return nil
}
