// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package install

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/elastic/elastic-package/internal/configuration/locations"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/profile"
)

const versionFilename = "version"

// EnsureInstalled method installs once static resources for the testing Docker stack.
func EnsureInstalled() error {
	elasticPackagePath, err := locations.NewLocationManager()
	if err != nil {
		return fmt.Errorf("failed locating the configuration directory: %s", err)
	}

	installed, err := checkIfAlreadyInstalled(elasticPackagePath)
	if err != nil {
		return fmt.Errorf("failed to check if there is an elastic-package installation: %s", err)
	}
	if installed {
		return nil
	}

	err = migrateIfNeeded(elasticPackagePath)
	if err != nil {
		return fmt.Errorf("error migrating old install: %s", err)
	}

	// Create the root .elastic-package path
	err = createElasticPackageDirectory(elasticPackagePath)
	if err != nil {
		return fmt.Errorf("creating elastic package directory failed: %s", err)
	}

	// write the root config.yml file
	err = writeConfigFile(elasticPackagePath)
	if err != nil {
		return fmt.Errorf("writing configuration file failed: %s", err)
	}

	// write root version file
	err = writeVersionFile(elasticPackagePath)
	if err != nil {
		return fmt.Errorf("writing version file failed: %s", err)
	}

	err = writeStackResources(elasticPackagePath)
	if err != nil {
		return fmt.Errorf("writing stack resources failed: %s", err)
	}

	err = writeTerraformDeployerResources(elasticPackagePath)
	if err != nil {
		return fmt.Errorf("writing Terraform deployer resources failed: %s", err)
	}

	err = writeDockerCustomAgentResources(elasticPackagePath)
	if err != nil {
		return fmt.Errorf("writing Terraform deployer resources failed: %s", err)
	}

	if err := createServiceLogsDir(elasticPackagePath); err != nil {
		return fmt.Errorf("creating service logs directory failed: %s", err)
	}

	fmt.Fprintln(os.Stderr, "elastic-package has been installed.")
	return nil
}

func checkIfAlreadyInstalled(elasticPackagePath *locations.LocationManager) (bool, error) {
	_, err := os.Stat(elasticPackagePath.StackDir())
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("stat file failed (path: %s): %s", elasticPackagePath, err)
	}
	return checkIfLatestVersionInstalled(elasticPackagePath)
}

// checkIfUnmigrated checks to see if we have a pre-profile config that needs to be migrated
func migrateIfNeeded(elasticPackagePath *locations.LocationManager) error {
	// use the snapshot.yml file as a canary to see if we have a pre-profile install
	_, err := os.Stat(filepath.Join(elasticPackagePath.StackDir(), string(profile.SnapshotFile)))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("stat file failed (path: %s): %s", elasticPackagePath, err)
	}

	profileName := fmt.Sprintf("default_migrated_%d", time.Now().Unix())
	logger.Warnf("Pre-profiles elastic-package detected. Existing config will be migrated to %s", profileName)
	// Depending on how old the install is, not all the files will be available to migrate,
	// So treat any errors from missing files as "soft"
	oldFiles := []string{
		filepath.Join(elasticPackagePath.StackDir(), string(profile.SnapshotFile)),
		filepath.Join(elasticPackagePath.StackDir(), string(profile.PackageRegistryDockerfileFile)),
		filepath.Join(elasticPackagePath.StackDir(), string(profile.KibanaConfigDefaultFile)),
		filepath.Join(elasticPackagePath.StackDir(), string(profile.PackageRegistryConfigFile)),
	}

	opts := profile.Options{
		PackagePath: elasticPackagePath.StackDir(),
		Name:        profileName,
	}
	err = profile.MigrateProfileFiles(opts, oldFiles)
	if err != nil {
		return fmt.Errorf("error migrating profile config: %s", err)
	}

	// delete the old files
	for _, file := range oldFiles {
		err = os.Remove(file)
		if err != nil {
			return fmt.Errorf("error removing config file %s: %s", file, err)
		}
	}
	return nil
}

func createElasticPackageDirectory(elasticPackagePath *locations.LocationManager) error {
	//remove unmanaged subdirectories
	err := os.RemoveAll(elasticPackagePath.TempDir()) // remove in case of potential upgrade
	if err != nil {
		return fmt.Errorf("removing directory failed (path: %s): %s", elasticPackagePath, err)
	}

	err = os.RemoveAll(elasticPackagePath.DeployerDir()) // remove in case of potential upgrade
	if err != nil {
		return fmt.Errorf("removing directory failed (path: %s): %s", elasticPackagePath, err)
	}

	err = os.MkdirAll(elasticPackagePath.RootDir(), 0755)
	if err != nil {
		return fmt.Errorf("creating directory failed (path: %s): %s", elasticPackagePath, err)
	}
	return nil
}

func writeStackResources(elasticPackagePath *locations.LocationManager) error {
	err := os.MkdirAll(elasticPackagePath.PackagesDir(), 0755)
	if err != nil {
		return fmt.Errorf("creating directory failed (path: %s): %s", elasticPackagePath.PackagesDir(), err)
	}

	err = os.MkdirAll(elasticPackagePath.ProfileDir(), 0755)
	if err != nil {
		return fmt.Errorf("creating directory failed (path: %s): %s", elasticPackagePath.PackagesDir(), err)
	}

	kibanaHealthcheckPath := filepath.Join(elasticPackagePath.StackDir(), "healthcheck.sh")
	err = writeStaticResource(err, kibanaHealthcheckPath, kibanaHealthcheckSh)
	if err != nil {
		return fmt.Errorf("copying healthcheck script failed (%s): %s", kibanaHealthcheckPath, err)
	}

	// Install GeoIP database
	ingestGeoIPDir := filepath.Join(elasticPackagePath.StackDir(), "ingest-geoip")
	err = os.MkdirAll(ingestGeoIPDir, 0755)
	if err != nil {
		return fmt.Errorf("creating directory failed (path: %s): %s", ingestGeoIPDir, err)
	}

	geoIpAsnMmdbPath := filepath.Join(ingestGeoIPDir, "GeoLite2-ASN.mmdb")
	err = writeStaticResource(err, geoIpAsnMmdbPath, geoIpAsnMmdb)
	if err != nil {
		return fmt.Errorf("copying GeoIP ASN database failed (%s): %s", geoIpAsnMmdbPath, err)
	}

	geoIpCityMmdbPath := filepath.Join(ingestGeoIPDir, "GeoLite2-City.mmdb")
	err = writeStaticResource(err, geoIpCityMmdbPath, geoIpCityMmdb)
	if err != nil {
		return fmt.Errorf("copying GeoIP city database failed (%s): %s", geoIpCityMmdbPath, err)
	}

	geoIpCountryMmdbPath := filepath.Join(ingestGeoIPDir, "GeoLite2-Country.mmdb")
	err = writeStaticResource(err, geoIpCountryMmdbPath, geoIpCountryMmdb)
	if err != nil {
		return fmt.Errorf("copying GeoIP country database failed (%s): %s", geoIpCountryMmdbPath, err)
	}

	serviceTokensPath := filepath.Join(elasticPackagePath.StackDir(), "service_tokens")
	err = writeStaticResource(err, serviceTokensPath, serviceTokens)
	if err != nil {
		return fmt.Errorf("copying service_tokens failed (%s): %s", serviceTokensPath, err)
	}

	options := profile.Options{
		PackagePath:       elasticPackagePath.ProfileDir(),
		Name:              profile.DefaultProfile,
		OverwriteExisting: false,
	}
	return profile.CreateProfile(options)
}

func writeTerraformDeployerResources(elasticPackagePath *locations.LocationManager) error {
	terraformDeployer := elasticPackagePath.TerraformDeployerDir()
	err := os.MkdirAll(terraformDeployer, 0755)
	if err != nil {
		return fmt.Errorf("creating directory failed (path: %s): %s", terraformDeployer, err)
	}

	err = writeStaticResource(err, elasticPackagePath.TerraformDeployerYml(), terraformDeployerYml)
	err = writeStaticResource(err, filepath.Join(terraformDeployer, "Dockerfile"), terraformDeployerDockerfile)
	err = writeStaticResource(err, filepath.Join(terraformDeployer, "run.sh"), terraformDeployerRun)
	if err != nil {
		return fmt.Errorf("writing static resource failed: %s", err)
	}
	return nil
}

func writeDockerCustomAgentResources(elasticPackagePath *locations.LocationManager) error {
	dir := elasticPackagePath.DockerCustomAgentDeployerDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating directory failed (path: %s): %s", dir, err)
	}
	if err := writeStaticResource(nil, elasticPackagePath.DockerCustomAgentDeployerYml(), dockerCustomAgentBaseYml); err != nil {
		return fmt.Errorf("writing static resource failed: %s", err)
	}
	return nil
}

func writeConfigFile(elasticPackagePath *locations.LocationManager) error {
	var err error
	err = writeStaticResource(err, filepath.Join(elasticPackagePath.RootDir(), applicationConfigurationYmlFile), applicationConfigurationYml)
	if err != nil {
		return fmt.Errorf("writing static resource failed: %s", err)
	}
	return nil
}

func writeStaticResource(err error, path, content string) error {
	if err != nil {
		return err
	}

	err = os.WriteFile(path, []byte(content), 0644)
	if err != nil {
		return fmt.Errorf("writing file failed (path: %s): %s", path, err)
	}
	return nil
}

func createServiceLogsDir(elasticPackagePath *locations.LocationManager) error {
	dirPath := elasticPackagePath.ServiceLogDir()
	err := os.MkdirAll(dirPath, 0755)
	if err != nil {
		return fmt.Errorf("mkdir failed (path: %s): %s", dirPath, err)
	}
	return nil
}
