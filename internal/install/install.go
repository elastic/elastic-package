// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package install

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/configuration/locations"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/profile"
)

const versionFilename = "version"

// EnsureInstalled method installs once static resources for the testing Docker stack.
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
		return nil
	}

	err = migrateIfNeeded(elasticPackagePath)
	if err != nil {
		return errors.Wrap(err, "error migrating old install")
	}

	// Create the root .elastic-package path
	err = createElasticPackageDirectory(elasticPackagePath)
	if err != nil {
		return errors.Wrap(err, "creating elastic package directory failed")
	}

	// write the root config.yml file
	err = writeConfigFile(elasticPackagePath)
	if err != nil {
		return errors.Wrap(err, "writing configuration file failed")
	}

	// write root version file
	err = writeVersionFile(elasticPackagePath)
	if err != nil {
		return errors.Wrap(err, "writing version file failed")
	}

	err = writeStackResources(elasticPackagePath)
	if err != nil {
		return errors.Wrap(err, "writing stack resources failed")
	}

	err = writeTerraformDeployerResources(elasticPackagePath)
	if err != nil {
		return errors.Wrap(err, "writing Terraform deployer resources failed")
	}

	if err := createServiceLogsDir(elasticPackagePath); err != nil {
		return errors.Wrap(err, "creating service logs directory failed")
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
		return false, errors.Wrapf(err, "stat file failed (path: %s)", elasticPackagePath)
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
		return errors.Wrapf(err, "stat file failed (path: %s)", elasticPackagePath)
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
		return errors.Wrap(err, "error migrating profile config")
	}

	// delete the old files
	for _, file := range oldFiles {
		err = os.Remove(file)
		if err != nil {
			return errors.Wrapf(err, "error removing config file %s", file)
		}
	}
	return nil
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

func writeStackResources(elasticPackagePath *locations.LocationManager) error {
	err := os.MkdirAll(elasticPackagePath.PackagesDir(), 0755)
	if err != nil {
		return errors.Wrapf(err, "creating directory failed (path: %s)", elasticPackagePath.PackagesDir())
	}

	err = os.MkdirAll(elasticPackagePath.ProfileDir(), 0755)
	if err != nil {
		return errors.Wrapf(err, "creating directory failed (path: %s)", elasticPackagePath.PackagesDir())
	}

	kibanaHealthcheckPath := filepath.Join(elasticPackagePath.StackDir(), "healthcheck.sh")
	err = writeStaticResource(err, kibanaHealthcheckPath, kibanaHealthcheckSh)
	if err != nil {
		return errors.Wrapf(err, "copying healthcheck script failed (%s)", kibanaHealthcheckPath)
	}

	// Install GeoIP database
	ingestGeoIPDir := filepath.Join(elasticPackagePath.StackDir(), "ingest-geoip")
	err = os.MkdirAll(ingestGeoIPDir, 0755)
	if err != nil {
		return errors.Wrapf(err, "creating directory failed (path: %s)", ingestGeoIPDir)
	}

	geoIpAsnMmdbPath := filepath.Join(ingestGeoIPDir, "GeoLite2-ASN.mmdb")
	err = writeStaticResource(err, geoIpAsnMmdbPath, geoIpAsnMmdb)
	if err != nil {
		return errors.Wrapf(err, "copying GeoIP ASN database failed (%s)", geoIpAsnMmdbPath)
	}

	geoIpCityMmdbPath := filepath.Join(ingestGeoIPDir, "GeoLite2-City.mmdb")
	err = writeStaticResource(err, geoIpCityMmdbPath, geoIpCityMmdb)
	if err != nil {
		return errors.Wrapf(err, "copying GeoIP city database failed (%s)", geoIpCityMmdbPath)
	}

	geoIpCountryMmdbPath := filepath.Join(ingestGeoIPDir, "GeoLite2-Country.mmdb")
	err = writeStaticResource(err, geoIpCountryMmdbPath, geoIpCountryMmdb)
	if err != nil {
		return errors.Wrapf(err, "copying GeoIP country database failed (%s)", geoIpCountryMmdbPath)
	}

	serviceTokensPath := filepath.Join(elasticPackagePath.StackDir(), "service_tokens")
	err = writeStaticResource(err, serviceTokensPath, serviceTokens)
	if err != nil {
		return errors.Wrapf(err, "copying service_tokens failed (%s)", serviceTokensPath)
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
		return errors.Wrapf(err, "creating directory failed (path: %s)", terraformDeployer)
	}

	err = writeStaticResource(err, elasticPackagePath.TerraformDeployerYml(), terraformDeployerYml)
	err = writeStaticResource(err, filepath.Join(terraformDeployer, "Dockerfile"), terraformDeployerDockerfile)
	err = writeStaticResource(err, filepath.Join(terraformDeployer, "run.sh"), terraformDeployerRun)
	if err != nil {
		return errors.Wrap(err, "writing static resource failed")
	}
	return nil
}

func writeConfigFile(elasticPackagePath *locations.LocationManager) error {
	var err error
	err = writeStaticResource(err, filepath.Join(elasticPackagePath.RootDir(), applicationConfigurationYmlFile), applicationConfigurationYml)
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

func createServiceLogsDir(elasticPackagePath *locations.LocationManager) error {
	dirPath := elasticPackagePath.ServiceLogDir()
	err := os.MkdirAll(dirPath, 0755)
	if err != nil {
		return errors.Wrapf(err, "mkdir failed (path: %s)", dirPath)
	}
	return nil
}
