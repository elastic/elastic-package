// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package install

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"

	"github.com/elastic/elastic-package/internal/configuration/locations"
	"github.com/elastic/elastic-package/internal/profile"
)

const versionFilename = "version"

// EnsureInstalled method installs once the required configuration files.
func EnsureInstalled() error {
	elasticPackagePath, err := locations.NewLocationManager()
	if err != nil {
		return errors.Wrap(err, "failed locating the configuration directory")
	}

	installed, err := checkIfAlreadyInstalled(elasticPackagePath)
	if err != nil {
		return errors.Wrap(err, "failed to check if there is an elastic-package installation")
	}
	if installed {
		latestInstalled, err := checkIfLatestVersionInstalled(elasticPackagePath)
		if err != nil {
			return errors.Wrap(err, "failed to check if latest version is installed")
		}
		if latestInstalled {
			return nil
		}
		return migrateConfigDirectory(elasticPackagePath)
	}

	// Create the root .elastic-package path.
	err = createElasticPackageDirectory(elasticPackagePath)
	if err != nil {
		return errors.Wrap(err, "creating elastic package directory failed")
	}

	// Write the root config.yml file.
	err = WriteConfigFile(elasticPackagePath, DefaultConfiguration())
	if err != nil {
		return errors.Wrap(err, "writing configuration file failed")
	}

	// Write root version file.
	err = writeVersionFile(elasticPackagePath)
	if err != nil {
		return errors.Wrap(err, "writing version file failed")
	}

	// Create initial profile:
	options := profile.Options{
		PackagePath:       elasticPackagePath.ProfileDir(),
		Name:              profile.DefaultProfile,
		OverwriteExisting: false,
	}
	err = profile.CreateProfile(options)
	if err != nil {
		return errors.Wrap(err, "creation of initial profile failed")
	}

	if err := createServiceLogsDir(elasticPackagePath); err != nil {
		return errors.Wrap(err, "creating service logs directory failed")
	}

	fmt.Fprintln(os.Stderr, "elastic-package has been installed.")
	return nil
}

func checkIfAlreadyInstalled(elasticPackagePath *locations.LocationManager) (bool, error) {
	_, err := os.Stat(elasticPackagePath.RootDir())
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, errors.Wrapf(err, "stat file failed (path: %s)", elasticPackagePath)
	}
	return true, nil
}

func createElasticPackageDirectory(elasticPackagePath *locations.LocationManager) error {
	//remove unmanaged subdirectories
	err := os.RemoveAll(elasticPackagePath.TempDir()) // remove in case of potential upgrade
	if err != nil {
		return errors.Wrapf(err, "removing directory failed (path: %s)", elasticPackagePath)
	}

	err = os.RemoveAll(elasticPackagePath.DeployerDir()) // remove in case of potential upgrade
	if err != nil {
		return errors.Wrapf(err, "removing directory failed (path: %s)", elasticPackagePath)
	}

	err = os.MkdirAll(elasticPackagePath.RootDir(), 0755)
	if err != nil {
		return errors.Wrapf(err, "creating directory failed (path: %s)", elasticPackagePath)
	}
	return nil
}

func WriteConfigFile(elasticPackagePath *locations.LocationManager, configuration *ApplicationConfiguration) error {
	d, err := yaml.Marshal(configuration.c)
	if err != nil {
		return errors.Wrap(err, "failed to encode configuration")
	}

	err = writeStaticResource(err, filepath.Join(elasticPackagePath.RootDir(), applicationConfigurationYmlFile), string(d))
	if err != nil {
		return errors.Wrap(err, "writing static resource failed")
	}
	return nil
}

func writeStaticResource(err error, path, content string) error {
	if err != nil {
		return err
	}

	err = os.WriteFile(path, []byte(content), 0644)
	if err != nil {
		return errors.Wrapf(err, "writing file failed (path: %s)", path)
	}
	return nil
}

func migrateConfigDirectory(elasticPackagePath *locations.LocationManager) error {
	err := writeVersionFile(elasticPackagePath)
	if err != nil {
		return errors.Wrap(err, "writing version file failed")
	}

	return nil
}

func createServiceLogsDir(elasticPackagePath *locations.LocationManager) error {
	dirPath := elasticPackagePath.ServiceLogDir()
	err := os.MkdirAll(dirPath, 0755)
	if err != nil {
		return errors.Wrapf(err, "mkdir failed (path: %s)", dirPath)
	}
	return nil
}
