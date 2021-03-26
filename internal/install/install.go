// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package install

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
)

const (
	elasticPackageDir = ".elastic-package"
	stackDir          = "stack"
	packagesDir       = "development"
	temporaryDir      = "tmp"
	deployerDir       = "deployer"

	kubernetesDeployerElasticAgentYmlFile = "elastic-agent.yml"
	terraformDeployerYmlFile              = "terraform-deployer.yml"
)

var (
	serviceLogsDir        = filepath.Join(temporaryDir, "service_logs")
	kubernetesDeployerDir = filepath.Join(deployerDir, "kubernetes")
	terraformDeployerDir  = filepath.Join(deployerDir, "terraform")
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

	err = writeConfigFile(elasticPackagePath)
	if err != nil {
		return errors.Wrap(err, "writing configuration file failed")
	}

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

// ServiceLogsDir function returns the location of the directory to store service logs on the
// local filesystem, i.e. the same one where elastic-package is installed.
func ServiceLogsDir() (string, error) {
	configurationDir, err := configurationDir()
	if err != nil {
		return "", errors.Wrap(err, "locating configuration directory failed")
	}
	return filepath.Join(configurationDir, serviceLogsDir), nil
}

// TerraformDeployerComposeFile function returns the path to the Terraform service deployer's definitions.
func TerraformDeployerComposeFile() (string, error) {
	configurationDir, err := configurationDir()
	if err != nil {
		return "", errors.Wrap(err, "locating configuration directory failed")
	}
	return filepath.Join(configurationDir, terraformDeployerDir, terraformDeployerYmlFile), nil
}

// KubernetesDeployerElasticAgentFile function returns the path to the Elastic Agent YAML definition for the Kubernetes cluster.
func KubernetesDeployerElasticAgentFile() (string, error) {
	configurationDir, err := configurationDir()
	if err != nil {
		return "", errors.Wrap(err, "locating configuration directory failed")
	}
	return filepath.Join(configurationDir, kubernetesDeployerDir, kubernetesDeployerElasticAgentYmlFile), nil
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
	if errors.Is(err, os.ErrNotExist) {
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
		return errors.Wrapf(err, "creating directory failed (path: %s)", packagesPath)
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

func writeKubernetesDeployerResources(elasticPackagePath string) error {
	kubernetesDeployer := filepath.Join(elasticPackagePath, kubernetesDeployerDir)
	err := os.MkdirAll(kubernetesDeployer, 0755)
	if err != nil {
		return errors.Wrapf(err, "creating directory failed (path: %s)", kubernetesDeployer)
	}

	appConfig, err := Configuration()
	if err != nil {
		return errors.Wrap(err, "can't read application configuration")
	}

	err = writeStaticResource(err, filepath.Join(kubernetesDeployer, kubernetesDeployerElasticAgentYmlFile),
		strings.ReplaceAll(kubernetesDeployerElasticAgentYml, "{{ ELASTIC_AGENT_IMAGE_REF }}",
			appConfig.DefaultStackImageRefs().ElasticAgent))
	if err != nil {
		return errors.Wrap(err, "writing static resource failed")
	}
	return nil
}

func writeTerraformDeployerResources(elasticPackagePath string) error {
	terraformDeployer := filepath.Join(elasticPackagePath, terraformDeployerDir)
	err := os.MkdirAll(terraformDeployer, 0755)
	if err != nil {
		return errors.Wrapf(err, "creating directory failed (path: %s)", terraformDeployer)
	}

	err = writeStaticResource(err, filepath.Join(terraformDeployer, terraformDeployerYmlFile), terraformDeployerYml)
	err = writeStaticResource(err, filepath.Join(terraformDeployer, "Dockerfile"), terraformDeployerDockerfile)
	err = writeStaticResource(err, filepath.Join(terraformDeployer, "run.sh"), terraformDeployerRun)
	if err != nil {
		return errors.Wrap(err, "writing static resource failed")
	}
	return nil
}

func writeConfigFile(elasticPackagePath string) error {
	var err error
	err = writeStaticResource(err, filepath.Join(elasticPackagePath, applicationConfigurationYmlFile), applicationConfigurationYml)
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

func createServiceLogsDir(elasticPackagePath string) error {
	dirPath := filepath.Join(elasticPackagePath, serviceLogsDir)
	err := os.MkdirAll(dirPath, 0755)
	if err != nil {
		return errors.Wrapf(err, "mkdir failed (path: %s)", dirPath)
	}
	return nil
}
