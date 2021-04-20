// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package servicedeployer

import (
	"os"
	"path/filepath"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/compose"
	"github.com/elastic/elastic-package/internal/configuration/locations"
	"github.com/elastic/elastic-package/internal/files"
	"github.com/elastic/elastic-package/internal/locations"
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
		return nil, errors.Wrap(err, "can't load docker compose definitions")
	}

	service := dockerComposeDeployedService{
		ymlPaths: ymlPaths,
		project:  "elastic-package-service",
	}
	outCtxt := inCtxt

	p, err := compose.NewProject(service.project, service.ymlPaths...)
	if err != nil {
		return nil, errors.Wrap(err, "could not create docker compose project for service")
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
		return nil, errors.Wrap(err, "can't build terraform aliases")
	}

	// Boot up service
	tfEnvironment := tsd.buildTerraformExecutorEnvironment(inCtxt)
	opts := compose.CommandOptions{
		Env:       tfEnvironment,
		ExtraArgs: []string{"--build", "-d"},
	}
	if err := p.Up(opts); err != nil {
		return nil, errors.Wrap(err, "could not boot up service using docker compose")
	}

	outCtxt.Agent.Host.NamePrefix = "docker-fleet-agent"
	service.ctxt = outCtxt
	return &service, nil
}

func (tsd TerraformServiceDeployer) loadComposeDefinitions() ([]string, error) {
	locationManager, err := locations.NewLocationManager()
	if err != nil {
		return nil, errors.Wrap(err, "can't locate docker compose file for Terraform deployer")
	}

	envYmlPath := filepath.Join(tsd.definitionsDir, envYmlFile)
	_, err = os.Stat(envYmlPath)
	if os.IsNotExist(err) {
		return []string{
			locationManager.TerraformDeployerYml(),
		}, nil
	}
	if err != nil {
		return nil, errors.Wrapf(err, "stat failed (path: %s)", envYmlPath)
	}
	return []string{
		locationManager.TerraformDeployerYml(), envYmlPath,
	}, nil
}

var _ ServiceDeployer = new(TerraformServiceDeployer)
