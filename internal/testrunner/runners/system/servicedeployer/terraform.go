// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package servicedeployer

import (
	_ "embed"
	"os"
	"path/filepath"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/compose"
	"github.com/elastic/elastic-package/internal/configuration/locations"
	"github.com/elastic/elastic-package/internal/files"
	"github.com/elastic/elastic-package/internal/logger"
)

const (
	terraformDeployerDir = "terraform"
	terraformDeployerYml = "terraform-deployer.yml"
	terraformDeployerRun = "run.sh"
)

//go:embed _static/terraform_deployer.yml
var terraformDeployerYmlContent []byte

//go:embed _static/terraform_deployer_run.sh
var terraformDeployerRunContent []byte

// TerraformServiceDeployer is responsible for deploying infrastructure described with Terraform definitions.
type TerraformServiceDeployer struct {
	definitionsDir string
}

// NewTerraformServiceDeployer creates an instance of TerraformServiceDeployer.
func NewTerraformServiceDeployer(definitionsDir string) (*TerraformServiceDeployer, error) {
	logger.Debug("%+v", definitionsDir)
	return &TerraformServiceDeployer{
		definitionsDir: definitionsDir,
	}, nil
}

// SetUp method boots up the Docker Compose with Terraform executor and mounted .tf definitions.
func (tsd TerraformServiceDeployer) SetUp(inCtxt ServiceContext) (DeployedService, error) {
	logger.Debug("setting up service using Terraform deployer")

	configDir, err := tsd.installDockerfile()
	if err != nil {
		return nil, errors.Wrap(err, "can't load Docker Compose definitions")
	}

	ymlPaths := []string{filepath.Join(configDir, terraformDeployerYml)}
	envYmlPath := filepath.Join(tsd.definitionsDir, envYmlFile)
	_, err = os.Stat(envYmlPath)
	if err == nil {
		ymlPaths = append(ymlPaths, envYmlPath)
	}

	service := dockerComposeDeployedService{
		ymlPaths: ymlPaths,
		project:  "elastic-package-service",
	}
	outCtxt := inCtxt

	p, err := compose.NewProject(service.project, service.ymlPaths...)
	if err != nil {
		return nil, errors.Wrap(err, "could not create Docker Compose project for service")
	}

	// Clean service logs
	err = files.RemoveContent(outCtxt.Logs.Folder.Local)
	if err != nil {
		return nil, errors.Wrap(err, "removing service logs failed")
	}

	// Set custom aliases, which may be used in agent policies.
	serviceComposeConfig, err := p.Config(compose.CommandOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "could not get Docker Compose configuration for service")
	}
	outCtxt.CustomProperties, err = buildTerraformAliases(serviceComposeConfig)
	if err != nil {
		return nil, errors.Wrap(err, "can't build Terraform aliases")
	}

	// Boot up service
	tfEnvironment := tsd.buildTerraformExecutorEnvironment(inCtxt)
	opts := compose.CommandOptions{
		Env:       tfEnvironment,
		ExtraArgs: []string{"--build", "-d"},
	}

	err = p.Up(opts)
	if err != nil {
		return nil, errors.Wrap(err, "could not boot up service using Docker Compose")
	}

	err = p.WaitForHealthy(opts)
	if err != nil {
		processServiceContainerLogs(p, compose.CommandOptions{
			Env: opts.Env,
		}, outCtxt.Name)
		return nil, errors.Wrap(err, "Terraform deployer is unhealthy")
	}

	outCtxt.Agent.Host.NamePrefix = "docker-fleet-agent"
	service.ctxt = outCtxt
	return &service, nil
}

func (tsd TerraformServiceDeployer) installDockerfile() (string, error) {
	locationManager, err := locations.NewLocationManager()
	if err != nil {
		return "", errors.Wrap(err, "failed to find the configuration directory")
	}

	tfDir := filepath.Join(locationManager.DeployerDir(), terraformDeployerDir)
	err = os.MkdirAll(tfDir, 0755)
	if err != nil {
		return "", errors.Wrap(err, "failed to create directory for terraform deployer files")
	}

	deployerYmlFile := filepath.Join(tfDir, terraformDeployerYml)
	err = os.WriteFile(deployerYmlFile, terraformDeployerYmlContent, 0644)
	if err != nil {
		return "", errors.Wrap(err, "failed to create terraform deployer yaml")
	}

	deployerRunFile := filepath.Join(tfDir, terraformDeployerRun)
	err = os.WriteFile(deployerRunFile, terraformDeployerRunContent, 0644)
	if err != nil {
		return "", errors.Wrap(err, "failed to create terraform deployer run script")
	}

	return tfDir, nil
}

var _ ServiceDeployer = new(TerraformServiceDeployer)
