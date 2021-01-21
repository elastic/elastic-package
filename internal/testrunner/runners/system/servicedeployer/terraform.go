// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package servicedeployer

import (
	"bytes"
	"fmt"
	"os/exec"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/compose"
	"github.com/elastic/elastic-package/internal/files"
	"github.com/elastic/elastic-package/internal/install"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/stack"
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
	logger.Debug("setting up service using Terraform service deployer")
	terraformDeployerYml, err := install.ServiceDeployerComposeFile("terraform")
	if err != nil {
		return nil, errors.Wrap(err, "can't locate docker compose file for service deployer")
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
	serviceName := inCtxt.Name
	opts := compose.CommandOptions{
		Env:       tfEnvironment,
		ExtraArgs: []string{"--build", "-d"},
	}
	if err := p.Up(opts); err != nil {
		return nil, errors.Wrap(err, "could not boot up service using docker compose")
	}

	// Build service container name
	serviceContainer := fmt.Sprintf("%s_terraform_1", service.project)
	outCtxt.Hostname = serviceContainer

	// Connect service network with stack network (for the purpose of metrics collection)
	stackNetwork := fmt.Sprintf("%s_default", stack.DockerComposeProjectName)
	logger.Debugf("attaching service container %s to stack network %s", serviceContainer, stackNetwork)
	cmd := exec.Command("docker", "network", "connect", stackNetwork, serviceContainer)
	errOutput := new(bytes.Buffer)
	cmd.Stderr = errOutput
	if err := cmd.Run(); err != nil {
		return nil, errors.Wrapf(err, "could not attach service container to the stack network (stderr=%q)", errOutput.String())
	}

	logger.Debugf("adding service container %s internal ports to context", serviceContainer)
	serviceComposeConfig, err := p.Config(compose.CommandOptions{
		Env: []string{fmt.Sprintf("%s=%s", serviceLogsDirEnv, outCtxt.Logs.Folder.Local)},
	})
	if err != nil {
		return nil, errors.Wrap(err, "could not get Docker Compose configuration for service")
	}

	s := serviceComposeConfig.Services[serviceName]
	outCtxt.Ports = make([]int, len(s.Ports))
	for idx, port := range s.Ports {
		outCtxt.Ports[idx] = port.InternalPort
	}

	// Shortcut to first port for convenience
	if len(outCtxt.Ports) > 0 {
		outCtxt.Port = outCtxt.Ports[0]
	}

	// Set custom aliases, which may be used in agent policies.
	outCtxt.CustomProperties = buildTerraformAliases()

	service.ctxt = outCtxt
	return &service, nil
}

var _ ServiceDeployer = new(TerraformServiceDeployer)
