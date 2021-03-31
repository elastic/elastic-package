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
	stackPath string
}

// NewLocationManager returns a new manager to track the Configuration dir
func NewLocationManager() (*LocationManager, error) {
	cfg, err := configurationDir()
	if err != nil {
		return nil, errors.Wrap(err, "error getting config dir")
	}

	return &LocationManager{stackPath: cfg}, nil

}

// RootDir returns the root elastic-package dir
func (loc LocationManager) RootDir() string {
	return loc.stackPath
}

// TempDir returns the temp directory location
func (loc LocationManager) TempDir() string {
	return filepath.Join(loc.stackPath, temporaryDir)
}

// DeployerDir returns the deployer directory location
func (loc LocationManager) DeployerDir() string {
	return filepath.Join(loc.stackPath, deployerDir)
}

// StackDir returns the stack directory location
func (loc LocationManager) StackDir() string {
	return filepath.Join(loc.stackPath, stackDir)
}

// PackagesDir returns the packages directory location
func (loc LocationManager) PackagesDir() string {
	return filepath.Join(loc.stackPath, stackDir, packagesDir)
}

// KubernetesDeployerDir returns the Kubernetes Deployer directory location
func (loc LocationManager) KubernetesDeployerDir() string {
	return filepath.Join(loc.stackPath, kubernetesDeployerDir)
}

// KubernetesDeployerAgentYml returns the Kubernetes Deployer Elastic Agent yml
func (loc LocationManager) KubernetesDeployerAgentYml() string {
	return filepath.Join(loc.stackPath, kubernetesDeployerDir, kubernetesDeployerElasticAgentYmlFile)
}

// TerraformDeployerDir returns the Terraform Directory
func (loc LocationManager) TerraformDeployerDir() string {
	return filepath.Join(loc.stackPath, terraformDeployerDir)
}

// TerraformDeployerYml returns the Terraform deployer yml file
func (loc LocationManager) TerraformDeployerYml() string {
	return filepath.Join(loc.stackPath, terraformDeployerDir, terraformDeployerYmlFile)
}

// ServiceLogDir returns the log directory
func (loc LocationManager) ServiceLogDir() string {
	return filepath.Join(loc.stackPath, serviceLogsDir)
}

// ConfigurationDir returns the configuration directory location
func configurationDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", errors.Wrap(err, "reading home dir failed")
	}
	return filepath.Join(homeDir, elasticPackageDir), nil
}
