// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package servicedeployer

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/common"
	"github.com/elastic/elastic-package/internal/compose"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/stack"
)

const (
	stdoutFileName = "stdout"
	stderrFileName = "stderr"
)

// DockerComposeServiceDeployer knows how to deploy a service defined via
// a Docker Compose file.
type DockerComposeServiceDeployer struct {
	ymlPath string
}

type dockerComposeDeployedService struct {
	ctxt common.ServiceContext

	ymlPath string
	project string

	stdout io.WriteCloser
	stderr io.WriteCloser

	stdoutFilePath string
	stderrFilePath string
}

// NewDockerComposeServiceDeployer returns a new instance of a DockerComposeServiceDeployer.
func NewDockerComposeServiceDeployer(ymlPath string) (*DockerComposeServiceDeployer, error) {
	return &DockerComposeServiceDeployer{
		ymlPath: ymlPath,
	}, nil
}

// SetUp sets up the service and returns any relevant information.
func (r *DockerComposeServiceDeployer) SetUp(ctxt common.ServiceContext) (DeployedService, error) {
	logger.Debug("setting up service using Docker Compose service deployer")
	service := dockerComposeDeployedService{
		ymlPath: r.ymlPath,
		ctxt:    ctxt,
		project: "elastic-package-service",
	}

	p, err := compose.NewProject(service.project, service.ymlPath)
	if err != nil {
		return nil, errors.Wrap(err, "could not create docker compose project for service")
	}

	// Boot up service
	opts := compose.CommandOptions{
		ExtraArgs: []string{"-d"},
	}
	if err := p.Up(opts); err != nil {
		return nil, errors.Wrap(err, "could not boot up service using docker compose")
	}

	// Build service container name
	serviceName := ctxt.Name
	serviceContainer := fmt.Sprintf("%s_%s_1", service.project, serviceName)
	service.ctxt.Hostname = serviceContainer

	// Redirect service container's STDOUT and STDERR streams to files in local logs folder
	localLogsFolder := ctxt.Logs.Folder.Local
	agentLogsFolder := ctxt.Logs.Folder.Agent

	service.stdoutFilePath = filepath.Join(localLogsFolder, stdoutFileName)
	logger.Debugf("creating temp file %s to hold service container %s STDOUT", service.stdoutFilePath, serviceContainer)
	outFile, err := os.Create(service.stdoutFilePath)
	if err != nil {
		return nil, errors.Wrap(err, "could not create STDOUT file")
	}
	service.stdout = outFile
	ctxt.STDOUT = agentLogsFolder + stdoutFileName

	service.stderrFilePath = filepath.Join(localLogsFolder, stderrFileName)
	logger.Debugf("creating temp file %s to hold service container %s STDERR", service.stderrFilePath, serviceContainer)
	errFile, err := os.Create(service.stderrFilePath)
	if err != nil {
		return nil, errors.Wrap(err, "could not create STDERR file")
	}
	service.stderr = errFile
	ctxt.STDERR = agentLogsFolder + stderrFileName

	logger.Debugf("redirecting service container %s STDOUT and STDERR to temp files", serviceContainer)
	cmd := exec.Command("docker", "attach", "--no-stdin", serviceContainer)
	cmd.Stdout = service.stdout
	cmd.Stderr = service.stderr

	if err := cmd.Start(); err != nil {
		return nil, errors.Wrap(err, "could not redirect service container STDOUT and STDERR streams")
	}

	stackNetwork := fmt.Sprintf("%s_default", stack.DockerComposeProjectName)
	logger.Debugf("attaching service container %s to stack network %s", serviceContainer, stackNetwork)

	cmd = exec.Command("docker", "network", "connect", stackNetwork, serviceContainer)
	if err := cmd.Run(); err != nil {
		return nil, errors.Wrap(err, "could not attach service container to stack network")
	}

	logger.Debugf("adding service container %s internal ports to context", serviceContainer)
	serviceComposeConfig, err := p.Config(compose.CommandOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "could not get Docker Compose configuration for service")
	}

	s := serviceComposeConfig.Services[serviceName]
	service.ctxt.Ports = make([]int, len(s.Ports))
	for idx, port := range s.Ports {
		service.ctxt.Ports[idx] = port.InternalPort
	}

	return &service, nil
}

// TearDown tears down the service.
func (s *dockerComposeDeployedService) TearDown() error {
	logger.Infof("tearing down service using docker compose runner")
	defer func() {
		if err := s.stderr.Close(); err != nil {
			logger.Errorf("could not close STDERR file: %s: %s", s.stderrFilePath, err)
		} else if err := os.Remove(s.stderrFilePath); err != nil {
			logger.Errorf("could not delete STDERR file: %s: %s", s.stderrFilePath, err)
		}
	}()

	defer func() {
		if err := s.stdout.Close(); err != nil {
			logger.Errorf("could not close STDOUT file: %s: %s", s.stdoutFilePath, err)
		} else if err := os.Remove(s.stdoutFilePath); err != nil {
			logger.Errorf("could not delete STDOUT file: %s: %s", s.stdoutFilePath, err)
		}
	}()

	p, err := compose.NewProject(s.project, s.ymlPath)
	if err != nil {
		return errors.Wrap(err, "could not create docker compose project for service")
	}

	opts := compose.CommandOptions{}
	if err := p.Down(opts); err != nil {
		return errors.Wrap(err, "could not shut down service using docker compose")
	}

	return nil
}

// GetContext returns the current context for the service.
func (s *dockerComposeDeployedService) GetContext() common.ServiceContext {
	return s.ctxt
}

// SetContext sets the current context for the service.
func (s *dockerComposeDeployedService) SetContext(ctxt common.ServiceContext) error {
	s.ctxt = ctxt
	return nil
}
