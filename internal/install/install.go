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
	stackDir          = "stack"
	packagesDir       = "development"
)

const versionFilename = "version"

// EnsureInstalled method installs once static resources for the testing Docker stack.
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

	err = writeVersionFile(elasticPackagePath)
	if err != nil {
		return errors.Wrap(err, "writing version file failed")
	}

	err = writeStackResources(elasticPackagePath)
	if err != nil {
		return errors.Wrap(err, "writing static resources failed")
	}

	fmt.Println("elastic-package has been installed.")
	return nil
}

// StackDir method returns the stack directory (see: stackDir).
func StackDir() (string, error) {
	configurationDir, err := configurationDir()
	if err != nil {
		return "", errors.Wrap(err, "locating configuration directory failed")
	}
	return filepath.Join(configurationDir, stackDir), nil
}

// StackPackagesDir method returns the stack packages directory used for package development.
func StackPackagesDir() (string, error) {
	stackDir, err := StackDir()
	if err != nil {
		return "", errors.Wrap(err, "locating stack directory failed")
	}
	return filepath.Join(stackDir, packagesDir), nil
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
	return checkIfLatestVersionInstalled(elasticPackagePath)
}

func createElasticPackageDirectory(elasticPackagePath string) error {
	err := os.RemoveAll(elasticPackagePath) // remove in case of potential upgrade
	if err != nil {
		return errors.Wrapf(err, "removing directory failed (path: %s)", elasticPackagePath)
	}

	err = os.MkdirAll(elasticPackagePath, 0755)
	if err != nil {
		return errors.Wrapf(err, "creating directory failed (path: %s)", elasticPackagePath)
	}
	return nil
}

func writeStackResources(elasticPackagePath string) error {
	stackPath := filepath.Join(elasticPackagePath, stackDir)
	packagesPath := filepath.Join(stackPath, packagesDir)
	err := os.MkdirAll(packagesPath, 0755)
	if err != nil {
		return errors.Wrapf(err, "creating directory failed (path: %s)", elasticPackagePath)
	}

	err = writeStaticResource(err, filepath.Join(stackPath, "kibana.config.yml"), kibanaConfigYml)
	err = writeStaticResource(err, filepath.Join(stackPath, "snapshot.yml"), snapshotYml)
	err = writeStaticResource(err, filepath.Join(stackPath, "package-registry.config.yml"), packageRegistryConfigYml)
	err = writeStaticResource(err, filepath.Join(stackPath, "Dockerfile.package-registry"), packageRegistryDockerfile)
	if err != nil {
		return errors.Wrap(err, "writing static resource failed")
	}
	return nil
}

func writeStaticResource(err error, path, content string) error {
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(path, []byte(content), 0644)
	if err != nil {
		return errors.Wrapf(err, "writing file failed (path: %s)", path)
	}
	return nil
}
