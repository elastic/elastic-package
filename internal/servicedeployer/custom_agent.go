// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package servicedeployer

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/elastic/elastic-package/internal/compose"
	"github.com/elastic/elastic-package/internal/configuration/locations"
	"github.com/elastic/elastic-package/internal/docker"
	"github.com/elastic/elastic-package/internal/files"
	"github.com/elastic/elastic-package/internal/install"
	"github.com/elastic/elastic-package/internal/kibana"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/stack"
)

const (
	dockerCustomAgentName       = "docker-custom-agent"
	dockerCustomAgentDir        = "docker_custom_agent"
	dockerCustomAgentDockerfile = "docker-custom-agent-base.yml"
)

//go:embed _static/docker-custom-agent-base.yml
var dockerCustomAgentDockerfileContent []byte

// CustomAgentDeployer knows how to deploy a custom elastic-agent defined via
// a Docker Compose file.
type CustomAgentDeployer struct {
	dockerComposeFile string
}

// NewCustomAgentDeployer returns a new instance of a deployedCustomAgent.
func NewCustomAgentDeployer(dockerComposeFile string) (*CustomAgentDeployer, error) {
	return &CustomAgentDeployer{
		dockerComposeFile: dockerComposeFile,
	}, nil
}

// SetUp sets up the service and returns any relevant information.
func (d *CustomAgentDeployer) SetUp(inCtxt ServiceContext) (DeployedService, error) {
	logger.Debug("setting up service using Docker Compose service deployer")

	appConfig, err := install.Configuration()
	if err != nil {
		return nil, fmt.Errorf("can't read application configuration: %w", err)
	}

	kibanaClient, err := kibana.NewClient()
	if err != nil {
		return nil, fmt.Errorf("can't create Kibana client: %w", err)
	}

	stackVersion, err := kibanaClient.Version()
	if err != nil {
		return nil, fmt.Errorf("can't read Kibana injected metadata: %w", err)
	}

	caCertPath, ok := os.LookupEnv(stack.CACertificateEnv)
	if !ok {
		return nil, fmt.Errorf("can't locate CA certificate: %s environment variable not set", stack.CACertificateEnv)
	}

	env := append(
		appConfig.StackImageRefs(stackVersion.Version()).AsEnv(),
		fmt.Sprintf("%s=%s", serviceLogsDirEnv, inCtxt.Logs.Folder.Local),
		fmt.Sprintf("%s=%s", localCACertEnv, caCertPath),
	)

	configDir, err := d.installDockerfile()
	if err != nil {
		return nil, fmt.Errorf("could not create resources for custom agent: %w", err)
	}

	ymlPaths := []string{
		d.dockerComposeFile,
		filepath.Join(configDir, dockerCustomAgentDockerfile),
	}

	service := dockerComposeDeployedService{
		ymlPaths: ymlPaths,
		project:  "elastic-package-service",
		variant: ServiceVariant{
			Name: dockerCustomAgentName,
			Env:  env,
		},
	}

	outCtxt := inCtxt

	p, err := compose.NewProject(service.project, service.ymlPaths...)
	if err != nil {
		return nil, fmt.Errorf("could not create Docker Compose project for service: %w", err)
	}

	// Verify the Elastic stack network
	err = stack.EnsureStackNetworkUp()
	if err != nil {
		return nil, fmt.Errorf("stack network is not ready: %w", err)
	}

	// Clean service logs
	err = files.RemoveContent(outCtxt.Logs.Folder.Local)
	if err != nil {
		return nil, fmt.Errorf("removing service logs failed: %w", err)
	}

	inCtxt.Name = dockerCustomAgentName
	serviceName := inCtxt.Name
	opts := compose.CommandOptions{
		Env:       env,
		ExtraArgs: []string{"--build", "-d"},
	}
	err = p.Up(opts)
	if err != nil {
		return nil, fmt.Errorf("could not boot up service using Docker Compose: %w", err)
	}

	// Connect service network with stack network (for the purpose of metrics collection)
	err = docker.ConnectToNetwork(p.ContainerName(serviceName), stack.Network())
	if err != nil {
		return nil, fmt.Errorf("can't attach service container to the stack network: %w", err)
	}

	err = p.WaitForHealthy(opts)
	if err != nil {
		processServiceContainerLogs(p, compose.CommandOptions{
			Env: opts.Env,
		}, outCtxt.Name)
		return nil, fmt.Errorf("service is unhealthy: %w", err)
	}

	// Build service container name
	outCtxt.Hostname = p.ContainerName(serviceName)

	logger.Debugf("adding service container %s internal ports to context", p.ContainerName(serviceName))
	serviceComposeConfig, err := p.Config(compose.CommandOptions{Env: env})
	if err != nil {
		return nil, fmt.Errorf("could not get Docker Compose configuration for service: %w", err)
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

	outCtxt.Agent.Host.NamePrefix = inCtxt.Name
	service.ctxt = outCtxt
	return &service, nil
}

// installDockerfile creates the files needed to run the custom elastic agent and returns
// the directory with these files.
func (d *CustomAgentDeployer) installDockerfile() (string, error) {
	locationManager, err := locations.NewLocationManager()
	if err != nil {
		return "", fmt.Errorf("failed to find the configuration directory: %w", err)
	}

	customAgentDir := filepath.Join(locationManager.DeployerDir(), dockerCustomAgentDir)
	err = os.MkdirAll(customAgentDir, 0755)
	if err != nil {
		return "", fmt.Errorf("failed to create directory for custom agent files: %w", err)
	}

	customAgentDockerfile := filepath.Join(customAgentDir, dockerCustomAgentDockerfile)
	err = os.WriteFile(customAgentDockerfile, dockerCustomAgentDockerfileContent, 0644)
	if err != nil {
		return "", fmt.Errorf("failed to create docker compose file for custom agent: %w", err)
	}

	return customAgentDir, nil
}
