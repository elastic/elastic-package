// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package servicedeployer

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/elastic/elastic-package/internal/compose"
	"github.com/elastic/elastic-package/internal/configuration/locations"
	"github.com/elastic/elastic-package/internal/docker"
	"github.com/elastic/elastic-package/internal/files"
	"github.com/elastic/elastic-package/internal/install"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/profile"
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
	profile           *profile.Profile
	dockerComposeFile string
	stackVersion      string
	variant           ServiceVariant
	packageName       string
	dataStream        string

	agentRunID string

	runTearDown  bool
	runTestsOnly bool
}

type CustomAgentDeployerOptions struct {
	Profile           *profile.Profile
	DockerComposeFile string
	StackVersion      string
	Variant           ServiceVariant
	PackageName       string
	DataStream        string

	RunTearDown  bool
	RunTestsOnly bool
}

var _ ServiceDeployer = new(CustomAgentDeployer)

// NewCustomAgentDeployer returns a new instance of a deployedCustomAgent.
func NewCustomAgentDeployer(options CustomAgentDeployerOptions) (*CustomAgentDeployer, error) {
	return &CustomAgentDeployer{
		profile:           options.Profile,
		dockerComposeFile: options.DockerComposeFile,
		stackVersion:      options.StackVersion,
		variant:           options.Variant,
		packageName:       options.PackageName,
		dataStream:        options.DataStream,
		runTearDown:       options.RunTearDown,
		runTestsOnly:      options.RunTestsOnly,
	}, nil
}

// SetUp sets up the service and returns any relevant information.
func (d *CustomAgentDeployer) SetUp(ctx context.Context, svcInfo ServiceInfo) (DeployedService, error) {
	logger.Debug("setting up service using Docker Compose service deployer")

	d.agentRunID = svcInfo.Test.RunID

	appConfig, err := install.Configuration()
	if err != nil {
		return nil, fmt.Errorf("can't read application configuration: %w", err)
	}

	caCertPath, err := stack.FindCACertificate(d.profile)
	if err != nil {
		return nil, fmt.Errorf("can't locate CA certificate: %w", err)
	}

	env := append(
		appConfig.StackImageRefs(d.stackVersion).AsEnv(),
		fmt.Sprintf("%s=%s", serviceLogsDirEnv, svcInfo.Logs.Folder.Local),
		fmt.Sprintf("%s=%s", localCACertEnv, caCertPath),
		fmt.Sprintf("%s=%s", agentHostnameEnv, d.agentHostname()),
		fmt.Sprintf("%s=%s", elasticAgentTagsEnv, strings.Join(svcInfo.Tags, ",")),
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

	p, err := compose.NewProject(service.project, service.ymlPaths...)
	if err != nil {
		return nil, fmt.Errorf("could not create Docker Compose project for service: %w", err)
	}

	// Verify the Elastic stack network
	err = stack.EnsureStackNetworkUp(d.profile)
	if err != nil {
		return nil, fmt.Errorf("stack network is not ready: %w", err)
	}

	// Clean service logs
	if d.runTestsOnly {
		// service logs folder must no be deleted to avoid breaking log files written
		// by the service. If this is required, those files should be rotated or truncated
		// so the service can still write to them.
		logger.Debug("Skipping removing service logs folder folder %s", svcInfo.Logs.Folder.Local)
	} else {
		err = files.RemoveContent(svcInfo.Logs.Folder.Local)
		if err != nil {
			return nil, fmt.Errorf("removing service logs failed: %w", err)
		}
	}

	svcInfo.Name = dockerCustomAgentName
	serviceName := svcInfo.Name

	opts := compose.CommandOptions{
		Env:       env,
		ExtraArgs: []string{"--build", "-d"},
	}

	if d.runTestsOnly || d.runTearDown {
		logger.Debug("Skipping bringing up docker-compose project and connect container to network (non setup steps)")
	} else {
		err = p.Up(ctx, opts)
		if err != nil {
			return nil, fmt.Errorf("could not boot up service using Docker Compose: %w", err)
		}
		// Connect service network with stack network (for the purpose of metrics collection)
		err = docker.ConnectToNetwork(p.ContainerName(serviceName), stack.Network(d.profile))
		if err != nil {
			return nil, fmt.Errorf("can't attach service container to the stack network: %w", err)
		}
	}

	// requires to be connected the service to the stack network
	err = p.WaitForHealthy(ctx, opts)
	if err != nil {
		processServiceContainerLogs(ctx, p, compose.CommandOptions{
			Env: opts.Env,
		}, svcInfo.Name)
		return nil, fmt.Errorf("service is unhealthy: %w", err)
	}

	// Build service container name
	// Set the same hostname as in the docker-compose (environment variable)
	//svcInfo.Hostname = p.ContainerName(serviceName)
	svcInfo.Hostname = d.agentHostname()
	svcInfo.AgentHostname = d.agentHostname()

	logger.Debugf("adding service container %s internal ports to context", p.ContainerName(serviceName))
	serviceComposeConfig, err := p.Config(ctx, compose.CommandOptions{Env: env})
	if err != nil {
		return nil, fmt.Errorf("could not get Docker Compose configuration for service: %w", err)
	}

	s := serviceComposeConfig.Services[serviceName]
	svcInfo.Ports = make([]int, len(s.Ports))
	for idx, port := range s.Ports {
		svcInfo.Ports[idx] = port.InternalPort
	}

	// Shortcut to first port for convenience
	if len(svcInfo.Ports) > 0 {
		svcInfo.Port = svcInfo.Ports[0]
	}

	svcInfo.Agent.Host.NamePrefix = svcInfo.Name
	service.svcInfo = svcInfo
	return &service, nil
}

func (d *CustomAgentDeployer) agentName() string {
	name := d.packageName
	if d.variant.Name != "" {
		name = fmt.Sprintf("%s-%s", name, d.variant.Name)
	}
	if d.dataStream != "" && d.dataStream != "." {
		name = fmt.Sprintf("%s-%s", name, d.dataStream)
	}
	return name
}

func (d *CustomAgentDeployer) agentHostname() string {
	return fmt.Sprintf("%s-%s-%s", dockerCustomAgentName, d.agentName(), d.agentRunID)
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
