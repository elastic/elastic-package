// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package servicedeployer

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/elastic/elastic-package/internal/compose"
	"github.com/elastic/elastic-package/internal/configuration/locations"
	"github.com/elastic/elastic-package/internal/files"
	"github.com/elastic/elastic-package/internal/logger"
)

// TerraformServiceDeployer is responsible for deploying infrastructure described with Terraform definitions.
type TerraformServiceDeployer struct {
	definitionsDir string
}

// NewTerraformServiceDeployer creates an instance of TerraformServiceDeployer.
func NewTerraformServiceDeployer(definitionsDir string) (*TerraformServiceDeployer, error) {
	return &TerraformServiceDeployer{
		definitionsDir: definitionsDir,
	}, nil
}

// SetUp method boots up the Docker Compose with Terraform executor and mounted .tf definitions.
func (tsd TerraformServiceDeployer) SetUp(inCtxt ServiceContext) (DeployedService, error) {
	logger.Debug("setting up service using Terraform deployer")

	ymlPaths, err := tsd.loadComposeDefinitions()
	if err != nil {
		return nil, fmt.Errorf("can't load Docker Compose definitions: %s", err)
	}

	service := dockerComposeDeployedService{
		ymlPaths: ymlPaths,
		project:  "elastic-package-service",
	}
	outCtxt := inCtxt

	p, err := compose.NewProject(service.project, service.ymlPaths...)
	if err != nil {
		return nil, fmt.Errorf("could not create Docker Compose project for service: %s", err)
	}

	// Clean service logs
	err = files.RemoveContent(outCtxt.Logs.Folder.Local)
	if err != nil {
		return nil, fmt.Errorf("removing service logs failed: %s", err)
	}

	// Set custom aliases, which may be used in agent policies.
	serviceComposeConfig, err := p.Config(compose.CommandOptions{})
	if err != nil {
		return nil, fmt.Errorf("could not get Docker Compose configuration for service: %s", err)
	}
	outCtxt.CustomProperties, err = buildTerraformAliases(serviceComposeConfig)
	if err != nil {
		return nil, fmt.Errorf("can't build Terraform aliases: %s", err)
	}

	// Boot up service
	tfEnvironment := tsd.buildTerraformExecutorEnvironment(inCtxt)
	opts := compose.CommandOptions{
		Env:       tfEnvironment,
		ExtraArgs: []string{"--build", "-d"},
	}

	err = p.Up(opts)
	if err != nil {
		return nil, fmt.Errorf("could not boot up service using Docker Compose: %s", err)
	}

	err = p.WaitForHealthy(opts)
	if err != nil {
		processServiceContainerLogs(p, compose.CommandOptions{
			Env: opts.Env,
		}, outCtxt.Name)
		return nil, fmt.Errorf("terraform deployer is unhealthy: %s", err)
	}

	outCtxt.Agent.Host.NamePrefix = "docker-fleet-agent"
	service.ctxt = outCtxt
	return &service, nil
}

func (tsd TerraformServiceDeployer) loadComposeDefinitions() ([]string, error) {
	locationManager, err := locations.NewLocationManager()
	if err != nil {
		return nil, fmt.Errorf("can't locate Docker Compose file for Terraform deployer: %s", err)
	}

	envYmlPath := filepath.Join(tsd.definitionsDir, envYmlFile)
	_, err = os.Stat(envYmlPath)
	if errors.Is(err, os.ErrNotExist) {
		return []string{
			locationManager.TerraformDeployerYml(),
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("stat failed (path: %s): %s", envYmlPath, err)
	}
	return []string{
		locationManager.TerraformDeployerYml(), envYmlPath,
	}, nil
}

var _ ServiceDeployer = new(TerraformServiceDeployer)
