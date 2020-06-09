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
)

func EnsureInstalled() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return errors.Wrap(err, "reading home dir failed")
	}
	elasticPackagePath := filepath.Join(homeDir, elasticPackageDir)

	installed, err := checkIfAlreadyInstalled(elasticPackagePath)
	if err != nil {
		return errors.Wrap(err, "can't check if the tool is installed")
	}

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
	err := os.MkdirAll(clusterPath, 0755)
	if err != nil {
		return errors.Wrapf(err, "creating directory failed (path: %s)", elasticPackagePath)
	}

	err = writeStaticResource(err, clusterPath, "kibana.config.yml", kibanaConfigYml)
	err = writeStaticResource(err, clusterPath, "local.yml", localYml)
	err = writeStaticResource(err, clusterPath, "snapshot.yml", snapshotYml)
	err = writeStaticResource(err, clusterPath, "package-registry-volume.yml", packageRegistryVolumeYml)

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
