// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package agentdeployer

import (
	_ "embed"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"

	"github.com/elastic/elastic-package/internal/compose"
	"github.com/elastic/elastic-package/internal/docker"
	"github.com/elastic/elastic-package/internal/elasticsearch"
	"github.com/elastic/elastic-package/internal/files"
	"github.com/elastic/elastic-package/internal/install"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/profile"
	"github.com/elastic/elastic-package/internal/stack"
)

const (
	dockerCustomAgentName          = "docker-custom-agent"
	dockerCustomAgentDir           = "docker_custom_agent"
	dockerCustomAgentDockerCompose = "docker-agent-base.yml"
	defaultAgentPolicyName         = "Elastic-Agent (elastic-package)"
)

//go:embed _static/docker-agent-base.yml
var dockerAgentDockerComposeContent []byte

// CustomAgentDeployer knows how to deploy a custom elastic-agent defined via
// a Docker Compose file.
type CustomAgentDeployer struct {
	profile           *profile.Profile
	dockerComposeFile string
	stackVersion      string

	variant AgentVariant

	API *elasticsearch.API

	packageName string
	dataStream  string

	runTearDown  bool
	runTestsOnly bool
}

type CustomAgentDeployerOptions struct {
	Profile           *profile.Profile
	DockerComposeFile string
	StackVersion      string
	Variant           AgentVariant

	API *elasticsearch.API

	PackageName string
	DataStream  string

	RunTearDown  bool
	RunTestsOnly bool
}

// NewCustomAgentDeployer returns a new instance of a deployedCustomAgent.
func NewCustomAgentDeployer(options CustomAgentDeployerOptions) (*CustomAgentDeployer, error) {
	return &CustomAgentDeployer{
		profile:           options.Profile,
		dockerComposeFile: options.DockerComposeFile,
		stackVersion:      options.StackVersion,
		packageName:       options.PackageName,
		dataStream:        options.DataStream,
		API:               options.API,
		variant:           options.Variant,
		runTearDown:       options.RunTearDown,
		runTestsOnly:      options.RunTestsOnly,
	}, nil
}

func readCACertBase64(profile *profile.Profile) (string, error) {
	caCertPath, err := stack.FindCACertificate(profile)
	if err != nil {
		return "", fmt.Errorf("can't locate CA certificate: %w", err)
	}

	d, err := os.ReadFile(caCertPath)
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(d), nil
}

// SetUp sets up the service and returns any relevant information.
func (d *CustomAgentDeployer) SetUp(inCtxt AgentInfo) (DeployedAgent, error) {
	logger.Debug("setting up service using Docker Compose agent deployer")

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
		fmt.Sprintf("%s=%s", serviceLogsDirEnv, inCtxt.Logs.Folder.Local),
		fmt.Sprintf("%s=%s", localCACertEnv, caCertPath),
		fmt.Sprintf("%s=%s", fleetPolicyEnv, defaultAgentPolicyName),
	)

	configDir, err := d.installDockerfile()
	if err != nil {
		return nil, fmt.Errorf("could not create resources for custom agent: %w", err)
	}

	ymlPaths := []string{
		filepath.Join(configDir, dockerCustomAgentDockerCompose),
	}
	if d.dockerComposeFile != "" {
		ymlPaths = []string{
			d.dockerComposeFile,
			filepath.Join(configDir, dockerCustomAgentDockerCompose),
		}
	}

	composeProjectName := fmt.Sprintf("elastic-package-agent-%s", d.agentName())

	service := dockerComposeDeployedAgent{
		ymlPaths: ymlPaths,
		project:  composeProjectName,
		variant: AgentVariant{
			Name: dockerCustomAgentName,
			Env:  env,
		},
	}

	outCtxt := inCtxt
	outCtxt.ConfigDir = configDir

	p, err := compose.NewProject(service.project, service.ymlPaths...)
	if err != nil {
		return nil, fmt.Errorf("could not create Docker Compose project for agent: %w", err)
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
		logger.Debug("Skipping removing service logs folder folder %s", outCtxt.Logs.Folder.Local)
	} else {
		err = files.RemoveContent(outCtxt.Logs.Folder.Local)
		if err != nil {
			return nil, fmt.Errorf("removing service logs failed: %w", err)
		}
	}

	// Service name defined in the docker-compose file
	inCtxt.Name = dockerCustomAgentName
	serviceName := inCtxt.Name

	opts := compose.CommandOptions{
		Env:       env,
		ExtraArgs: []string{"--build", "-d"},
	}

	if d.runTestsOnly || d.runTearDown {
		logger.Debug("Skipping bringing up docker-compose project and connect container to network (non setup steps)")
	} else {
		err = p.Up(opts)
		if err != nil {
			return nil, fmt.Errorf("could not boot up agent using Docker Compose: %w", err)
		}
		// Connect service network with stack network (for the purpose of metrics collection)
		err = docker.ConnectToNetwork(p.ContainerName(serviceName), stack.Network(d.profile))
		if err != nil {
			return nil, fmt.Errorf("can't attach service container to the stack network: %w", err)
		}
	}

	// requires to be connected the service to the stack network
	err = p.WaitForHealthy(opts)
	if err != nil {
		processAgentContainerLogs(p, compose.CommandOptions{
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
	service.agentInfo = outCtxt
	return &service, nil
}

func (d *CustomAgentDeployer) agentName() string {
	if d.variant.Name != "" {
		return fmt.Sprintf("%s-%s-%s", d.packageName, d.variant.Name, d.dataStream)
	}
	return fmt.Sprintf("%s-%s", d.packageName, d.dataStream)
}

// installDockerfile creates the files needed to run the custom elastic agent and returns
// the directory with these files.
func (d *CustomAgentDeployer) installDockerfile() (string, error) {
	customAgentDir := filepath.Join(d.profile.ProfilePath, fmt.Sprintf("agent-%s", d.agentName()))
	err := os.MkdirAll(customAgentDir, 0o755)
	if err != nil {
		return "", fmt.Errorf("failed to create directory for custom agent files: %w", err)
	}

	customAgentDockerfile := filepath.Join(customAgentDir, dockerCustomAgentDockerCompose)
	err = os.WriteFile(customAgentDockerfile, dockerAgentDockerComposeContent, 0o644)
	if err != nil {
		return "", fmt.Errorf("failed to create docker compose file for custom agent: %w", err)
	}

	return customAgentDir, nil
}
