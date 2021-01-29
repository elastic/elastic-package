// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package servicedeployer

import (
	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/compose"
	"github.com/elastic/elastic-package/internal/files"
	"github.com/elastic/elastic-package/internal/install"
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
	terraformDeployerYml, err := install.TerraformDeployerComposeFile()
	if err != nil {
		return nil, errors.Wrap(err, "can't locate docker compose file for Terraform deployer")
	}

	service := dockerComposeDeployedService{
		ymlPath: terraformDeployerYml,
		project: "elastic-package-service",
	}
	outCtxt := inCtxt

	p, err := compose.NewProject(service.project, service.ymlPath)
	if err != nil {
		return nil, errors.Wrap(err, "could not create docker compose project for service")
	}

	// Clean service logs
	err = files.RemoveContent(outCtxt.Logs.Folder.Local)
	if err != nil {
		return nil, errors.Wrap(err, "removing service logs failed")
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

	// Set custom aliases, which may be used in agent policies.
	outCtxt.CustomProperties = buildTerraformAliases()

	service.ctxt = outCtxt
	return &service, nil
}

var _ ServiceDeployer = new(TerraformServiceDeployer)
