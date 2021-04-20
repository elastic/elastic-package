// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package install

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/configuration/locations"
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
	if installed {
		return nil
	}

	// Create the root .elastic-package path
	err = createElasticPackageDirectory(elasticPackagePath)
	if err != nil {
		return errors.Wrap(err, "creating elastic package directory failed")
	}

	//write the root config.yml file
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

	err = writeKubernetesDeployerResources(elasticPackagePath)
	if err != nil {
		return errors.Wrap(err, "writing Kubernetes deployer resources failed")
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
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, errors.Wrapf(err, "stat file failed (path: %s)", elasticPackagePath)
	}
	return checkIfLatestVersionInstalled(elasticPackagePath.RootDir())
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

	err = os.MkdirAll(elasticPackagePath.StackPath, 0755)
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

	return profile.CreateProfile(elasticPackagePath.StackDir(), profile.DefaultProfile, false)

}

func writeKubernetesDeployerResources(elasticPackagePath locations.LocationManager) error {
	err := os.MkdirAll(elasticPackagePath.KubernetesDeployerDir(), 0755)
	if err != nil {
		return errors.Wrapf(err, "creating directory failed (path: %s)", elasticPackagePath.KubernetesDeployerDir())
	}

	appConfig, err := Configuration()
	if err != nil {
		return errors.Wrap(err, "can't read application configuration")
	}

	err = writeStaticResource(err, elasticPackagePath.KubernetesDeployerAgentYml(),
		strings.ReplaceAll(kubernetesDeployerElasticAgentYml, "{{ ELASTIC_AGENT_IMAGE_REF }}",
			appConfig.DefaultStackImageRefs().ElasticAgent))
	if err != nil {
		return errors.Wrap(err, "writing static resource failed")
	}
	return nil
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

	err = ioutil.WriteFile(path, []byte(content), 0644)
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
