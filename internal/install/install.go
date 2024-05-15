// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package install

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/elastic/elastic-package/internal/configuration/locations"
	"github.com/elastic/elastic-package/internal/profile"
)

const versionFilename = "version"

// EnsureInstalled method installs once the required configuration files.
func EnsureInstalled() error {
	elasticPackagePath, err := locations.NewLocationManager()
	if err != nil {
		return fmt.Errorf("failed locating the configuration directory: %w", err)
	}

	installed, err := checkIfAlreadyInstalled(elasticPackagePath)
	if err != nil {
		return fmt.Errorf("failed to check if there is an elastic-package installation: %w", err)
	}
	if installed {
		latestInstalled, err := checkIfLatestVersionInstalled(elasticPackagePath)
		if err != nil {
			return fmt.Errorf("failed to check if latest version is installed: %w", err)
		}
		if latestInstalled {
			return nil
		}
		return migrateConfigDirectory(elasticPackagePath)
	}

	// Create the root .elastic-package path.
	err = createElasticPackageDirectory(elasticPackagePath)
	if err != nil {
		return fmt.Errorf("creating elastic package directory failed: %w", err)
	}

	// Write the root config.yml file.
	err = WriteConfigFile(elasticPackagePath, DefaultConfiguration())
	if err != nil {
		return fmt.Errorf("writing configuration file failed: %w", err)
	}

	// Write root version file.
	err = writeVersionFile(elasticPackagePath)
	if err != nil {
		return fmt.Errorf("writing version file failed: %w", err)
	}

	// Create initial profile:
	options := profile.Options{
		ProfilesDirPath:   elasticPackagePath.ProfileDir(),
		Name:              profile.DefaultProfile,
		OverwriteExisting: false,
	}
	err = profile.CreateProfile(options)
	if err != nil {
		return fmt.Errorf("creation of initial profile failed: %w", err)
	}

	if err := createServiceLogsDir(elasticPackagePath); err != nil {
		return fmt.Errorf("creating service logs directory failed: %w", err)
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
		return false, fmt.Errorf("stat file failed (path: %s): %w", elasticPackagePath, err)
	}
	return true, nil
}

func createElasticPackageDirectory(elasticPackagePath *locations.LocationManager) error {
	//remove unmanaged subdirectories
	err := os.RemoveAll(elasticPackagePath.TempDir()) // remove in case of potential upgrade
	if err != nil {
		return fmt.Errorf("removing directory failed (path: %s): %w", elasticPackagePath, err)
	}

	err = os.RemoveAll(elasticPackagePath.DeployerDir()) // remove in case of potential upgrade
	if err != nil {
		return fmt.Errorf("removing directory failed (path: %s): %w", elasticPackagePath, err)
	}

	err = os.MkdirAll(elasticPackagePath.RootDir(), 0755)
	if err != nil {
		return fmt.Errorf("creating directory failed (path: %s): %w", elasticPackagePath, err)
	}
	return nil
}

func WriteConfigFile(elasticPackagePath *locations.LocationManager, configuration *ApplicationConfiguration) error {
	d, err := yaml.Marshal(configuration.c)
	if err != nil {
		return fmt.Errorf("failed to encode configuration: %w", err)
	}

	err = writeStaticResource(err, filepath.Join(elasticPackagePath.RootDir(), applicationConfigurationYmlFile), string(d))
	if err != nil {
		return fmt.Errorf("writing static resource failed: %w", err)
	}
	return nil
}

func writeStaticResource(err error, path, content string) error {
	if err != nil {
		return err
	}

	err = os.WriteFile(path, []byte(content), 0644)
	if err != nil {
		return fmt.Errorf("writing file failed (path: %s): %w", path, err)
	}
	return nil
}

func migrateConfigDirectory(elasticPackagePath *locations.LocationManager) error {
	err := writeVersionFile(elasticPackagePath)
	if err != nil {
		return fmt.Errorf("writing version file failed: %w", err)
	}

	return nil
}

func createServiceLogsDir(elasticPackagePath *locations.LocationManager) error {
	dirPath := elasticPackagePath.ServiceLogDir()
	err := os.MkdirAll(dirPath, 0755)
	if err != nil {
		return fmt.Errorf("mkdir failed (path: %s): %w", dirPath, err)
	}
	return nil
}
