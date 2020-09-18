// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package servicedeployer

import (
	"fmt"
	"os/exec"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/compose"
	"github.com/elastic/elastic-package/internal/files"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/stack"
)

const serviceLogsAgentDir = "/tmp/service_logs"

// DockerComposeServiceDeployer knows how to deploy a service defined via
// a Docker Compose file.
type DockerComposeServiceDeployer struct {
	ymlPath string
}

type dockerComposeDeployedService struct {
	ctxt ServiceContext

	ymlPath string
	project string
}

// NewDockerComposeServiceDeployer returns a new instance of a DockerComposeServiceDeployer.
func NewDockerComposeServiceDeployer(ymlPath string) (*DockerComposeServiceDeployer, error) {
	return &DockerComposeServiceDeployer{
		ymlPath: ymlPath,
	}, nil
}

// SetUp sets up the service and returns any relevant information.
func (r *DockerComposeServiceDeployer) SetUp(inCtxt ServiceContext) (DeployedService, error) {
	logger.Debug("setting up service using Docker Compose service deployer")
	service := dockerComposeDeployedService{
		ymlPath: r.ymlPath,
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
	outCtxt.Logs.Folder.Agent = serviceLogsAgentDir

	// Boot up service
	opts := compose.CommandOptions{
		Env:       []string{fmt.Sprintf("%s=%s", serviceLogsDirEnv, outCtxt.Logs.Folder.Local)},
		ExtraArgs: []string{"-d"},
	}
	if err := p.Up(opts); err != nil {
		return nil, errors.Wrap(err, "could not boot up service using docker compose")
	}

	// Build service container name
	serviceName := inCtxt.Name
	serviceContainer := fmt.Sprintf("%s_%s_1", service.project, serviceName)
	outCtxt.Hostname = serviceContainer

	// Connect service network with stack network (for the purpose of metrics collection)
	stackNetwork := fmt.Sprintf("%s_default", stack.DockerComposeProjectName)
	logger.Debugf("attaching service container %s to stack network %s", serviceContainer, stackNetwork)
	cmd := exec.Command("docker", "network", "connect", stackNetwork, serviceContainer)
	if err := cmd.Run(); err != nil {
		return nil, errors.Wrap(err, "could not attach service container to the stack network")
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

	service.ctxt = outCtxt
	return &service, nil
}

// TearDown tears down the service.
func (s *dockerComposeDeployedService) TearDown() error {
	logger.Infof("tearing down service using docker compose runner")
	defer func() {
		err := files.RemoveContent(s.ctxt.Logs.Folder.Local)
		if err != nil {
			logger.Errorf("could not remove the service logs (path: %s)", s.ctxt.Logs.Folder.Local)
		}
	}()

	p, err := compose.NewProject(s.project, s.ymlPath)
	if err != nil {
		return errors.Wrap(err, "could not create docker compose project for service")
	}

	if err := p.Down(compose.CommandOptions{
		Env: []string{fmt.Sprintf("%s=%s", serviceLogsDirEnv, s.ctxt.Logs.Folder.Local)},
	}); err != nil {
		return errors.Wrap(err, "could not shut down service using docker compose")
	}
	return nil
}

// Context returns the current context for the service.
func (s *dockerComposeDeployedService) Context() ServiceContext {
	return s.ctxt
}

// SetContext sets the current context for the service.
func (s *dockerComposeDeployedService) SetContext(ctxt ServiceContext) error {
	s.ctxt = ctxt
	return nil
}
