// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

// Package locations manages base file and directory locations from within the elastic-package config
package locations

import (
	"os"
	"path/filepath"

	"github.com/pkg/errors"
)

const (
	elasticPackageDir = ".elastic-package"
	stackDir          = "stack"
	packagesDir       = "development"

	temporaryDir = "tmp"
	deployerDir  = "deployer"

	kubernetesDeployerElasticAgentYmlFile = "elastic-agent.yml"
	terraformDeployerYmlFile              = "terraform-deployer.yml"
)

var (
	serviceLogsDir        = filepath.Join(temporaryDir, "service_logs")
	kubernetesDeployerDir = filepath.Join(deployerDir, "kubernetes")
	terraformDeployerDir  = filepath.Join(deployerDir, "terraform")
)

//LocationManager maintains an instance of a config path location
type LocationManager struct {
	StackPath string
}

// NewLocationManager returns a new manager to track the Configuration dir
func NewLocationManager() (LocationManager, error) {

	cfg, err := ConfigurationDir()
	if err != nil {
		return LocationManager{}, errors.Wrap(err, "error getting config dir")
	}

	return LocationManager{cfg}, nil

}

// TempDir returns the temp directory location
func (loc LocationManager) TempDir() string {
	return filepath.Join(loc.StackPath, temporaryDir)
}

// DeployerDir returns the deployer directory location
func (loc LocationManager) DeployerDir() string {
	return filepath.Join(loc.StackPath, deployerDir)
}

// StackDir returns the stack directory location
func (loc LocationManager) StackDir() string {
	return filepath.Join(loc.StackPath, stackDir)
}

// PackagesDir returns the packages directory location
func (loc LocationManager) PackagesDir() string {
	return filepath.Join(loc.StackPath, stackDir, packagesDir)
}

// KubernetesDeployerDir returns the Kubernetes Deployer directory location
func (loc LocationManager) KubernetesDeployerDir() string {
	return filepath.Join(loc.StackPath, kubernetesDeployerDir)
}

// KubernetesDeployerAgentYml returns the Kubernetes Deployer Elastic Agent yml
func (loc LocationManager) KubernetesDeployerAgentYml() string {
	return filepath.Join(loc.StackPath, kubernetesDeployerDir, kubernetesDeployerElasticAgentYmlFile)
}

// TerraformDeployerDir returns the Terraform Directory
func (loc LocationManager) TerraformDeployerDir() string {
	return filepath.Join(loc.StackPath, terraformDeployerDir)
}

// TerraformDeployerYml returns the Terraform deployer yml file
func (loc LocationManager) TerraformDeployerYml() string {
	return filepath.Join(loc.StackPath, terraformDeployerDir, terraformDeployerYmlFile)
}

// ServiceLogDir returns the log directory
func (loc LocationManager) ServiceLogDir() string {
	return filepath.Join(loc.StackPath, serviceLogsDir)
}

// ConfigurationDir returns the configuration directory location
func ConfigurationDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", errors.Wrap(err, "reading home dir failed")
	}
	return filepath.Join(homeDir, elasticPackageDir), nil
}

// StackDir method returns the stack directory (see: stackDir).
func StackDir() (string, error) {
	configurationDir, err := ConfigurationDir()
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
	configurationDir, err := ConfigurationDir()
	if err != nil {
		return "", errors.Wrap(err, "locating configuration directory failed")
	}
	return filepath.Join(configurationDir, serviceLogsDir), nil
}

// TerraformDeployerComposeFile function returns the path to the Terraform service deployer's definitions.
func TerraformDeployerComposeFile() (string, error) {
	configurationDir, err := ConfigurationDir()
	if err != nil {
		return "", errors.Wrap(err, "locating configuration directory failed")
	}
	return filepath.Join(configurationDir, terraformDeployerDir, terraformDeployerYmlFile), nil
}

// KubernetesDeployerElasticAgentFile function returns the path to the Elastic Agent YAML definition for the Kubernetes cluster.
func KubernetesDeployerElasticAgentFile() (string, error) {
	configurationDir, err := ConfigurationDir()
	if err != nil {
		return "", errors.Wrap(err, "locating configuration directory failed")
	}
	return filepath.Join(configurationDir, kubernetesDeployerDir, kubernetesDeployerElasticAgentYmlFile), nil
}
