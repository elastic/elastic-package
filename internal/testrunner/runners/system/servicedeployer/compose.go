// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package servicedeployer

import (
	"fmt"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/compose"
	"github.com/elastic/elastic-package/internal/docker"
	"github.com/elastic/elastic-package/internal/files"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/stack"
)

// DockerComposeServiceDeployer knows how to deploy a service defined via
// a Docker Compose file.
type DockerComposeServiceDeployer struct {
	ymlPaths []string
	sv       ServiceVariant
}

type dockerComposeDeployedService struct {
	ctxt ServiceContext

	ymlPaths []string
	project  string
	sv       ServiceVariant
}

// NewDockerComposeServiceDeployer returns a new instance of a DockerComposeServiceDeployer.
func NewDockerComposeServiceDeployer(ymlPaths []string, sv ServiceVariant) (*DockerComposeServiceDeployer, error) {
	return &DockerComposeServiceDeployer{
		ymlPaths: ymlPaths,
		sv:       sv,
	}, nil
}

// SetUp sets up the service and returns any relevant information.
func (d *DockerComposeServiceDeployer) SetUp(inCtxt ServiceContext) (DeployedService, error) {
	logger.Debug("setting up service using Docker Compose service deployer")
	service := dockerComposeDeployedService{
		ymlPaths: d.ymlPaths,
		project:  "elastic-package-service",
		sv:       d.sv,
	}
	outCtxt := inCtxt

	p, err := compose.NewProject(service.project, service.ymlPaths...)
	if err != nil {
		return nil, errors.Wrap(err, "could not create Docker Compose project for service")
	}

	// Verify the Elastic stack network
	err = stack.EnsureStackNetworkUp()
	if err != nil {
		return nil, errors.Wrap(err, "Elastic stack network is not ready")
	}

	// Clean service logs
	err = files.RemoveContent(outCtxt.Logs.Folder.Local)
	if err != nil {
		return nil, errors.Wrap(err, "removing service logs failed")
	}

	// Boot up service
	if d.sv.active() {
		logger.Infof("Using service variant: %s", d.sv.String())
	}

	serviceName := inCtxt.Name
	opts := compose.CommandOptions{
		Env: append(
			[]string{fmt.Sprintf("%s=%s", serviceLogsDirEnv, outCtxt.Logs.Folder.Local)},
			d.sv.Env...),
		ExtraArgs: []string{"--build", "-d"},
	}
	err = p.Up(opts)
	if err != nil {
		return nil, errors.Wrap(err, "could not boot up service using Docker Compose")
	}

	err = p.WaitForHealthy(opts)
	if err != nil {
		return nil, errors.Wrap(err, "service is unhealthy")
	}

	// Build service container name
	outCtxt.Hostname = p.ContainerName(serviceName)

	// Connect service network with stack network (for the purpose of metrics collection)
	err = docker.ConnectToNetwork(p.ContainerName(serviceName), stack.Network())
	if err != nil {
		return nil, errors.Wrapf(err, "can't attach service container to the stack network")
	}

	logger.Debugf("adding service container %s internal ports to context", p.ContainerName(serviceName))
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

	outCtxt.Agent.Host.NamePrefix = "docker-fleet-agent"
	service.ctxt = outCtxt
	return &service, nil
}

// Signal sends a signal to the service.
func (s *dockerComposeDeployedService) Signal(signal string) error {
	p, err := compose.NewProject(s.project, s.ymlPaths...)
	if err != nil {
		return errors.Wrap(err, "could not create Docker Compose project for service")
	}

	opts := compose.CommandOptions{ExtraArgs: []string{"-s", signal}}
	if s.ctxt.Name != "" {
		opts.Services = append(opts.Services, s.ctxt.Name)
	}

	return errors.Wrapf(p.Kill(opts), "could not send %q signal", signal)
}

// TearDown tears down the service.
func (s *dockerComposeDeployedService) TearDown() error {
	logger.Debugf("tearing down service using Docker Compose runner")
	defer func() {
		err := files.RemoveContent(s.ctxt.Logs.Folder.Local)
		if err != nil {
			logger.Errorf("could not remove the service logs (path: %s)", s.ctxt.Logs.Folder.Local)
		}
	}()

	p, err := compose.NewProject(s.project, s.ymlPaths...)
	if err != nil {
		return errors.Wrap(err, "could not create Docker Compose project for service")
	}

	downOptions := compose.CommandOptions{
		Env: append(
			[]string{fmt.Sprintf("%s=%s", serviceLogsDirEnv, s.ctxt.Logs.Folder.Local)},
			s.sv.Env...),

		// Remove associated volumes.
		ExtraArgs: []string{"--volumes"},
	}
	if err := p.Down(downOptions); err != nil {
		return errors.Wrap(err, "could not shut down service using Docker Compose")
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
