// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package agentdeployer

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
	dockerTestAgentNamePrefix    = "docker-test-agent"
	dockerTestgentDir            = "docker_test_agent"
	dockerTestAgentDockerCompose = "docker-agent-base.yml"
	defaultAgentPolicyName       = "Elastic-Agent (elastic-package)"
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

	PackageName string
	DataStream  string

	RunTearDown  bool
	RunTestsOnly bool
}

var _ AgentDeployer = new(CustomAgentDeployer)

// NewCustomAgentDeployer returns a new instance of a deployedCustomAgent.
func NewCustomAgentDeployer(options CustomAgentDeployerOptions) (*CustomAgentDeployer, error) {
	return &CustomAgentDeployer{
		profile:           options.Profile,
		dockerComposeFile: options.DockerComposeFile,
		stackVersion:      options.StackVersion,
		packageName:       options.PackageName,
		dataStream:        options.DataStream,
		variant:           options.Variant,
		runTearDown:       options.RunTearDown,
		runTestsOnly:      options.RunTestsOnly,
	}, nil
}

// SetUp sets up the service and returns any relevant information.
func (d *CustomAgentDeployer) SetUp(ctx context.Context, agentInfo AgentInfo) (DeployedAgent, error) {
	logger.Debug("setting up service using Docker Compose agent deployer")

	appConfig, err := install.Configuration()
	if err != nil {
		return nil, fmt.Errorf("can't read application configuration: %w", err)
	}

	caCertPath, err := stack.FindCACertificate(d.profile)
	if err != nil {
		return nil, fmt.Errorf("can't locate CA certificate: %w", err)
	}

	// Local Elastic stacks have a default Agent Policy created,
	// but Cloud or Serverless Projects could not have one
	agentPolicyName := defaultAgentPolicyName
	if strings.HasPrefix(d.stackVersion, "7.") {
		// Local Elastic stacks 7.* have an Agent Policy that is set as default
		// No need to set an Agent Policy Name
		agentPolicyName = ""
	}

	env := append(
		appConfig.StackImageRefs(d.stackVersion).AsEnv(),
		fmt.Sprintf("%s=%s", serviceLogsDirEnv, agentInfo.Logs.Folder.Local),
		fmt.Sprintf("%s=%s", localCACertEnv, caCertPath),
		fmt.Sprintf("%s=%s", fleetPolicyEnv, agentPolicyName),
		fmt.Sprintf("%s=%s", agentHostnameEnv, d.agentHostname()),
		fmt.Sprintf("%s=%s", elasticAgentTagsEnv, strings.Join(agentInfo.Tags, ",")),
	)

	configDir, err := d.installDockerfile()
	if err != nil {
		return nil, fmt.Errorf("could not create resources for custom agent: %w", err)
	}

	ymlPaths := []string{
		filepath.Join(configDir, dockerTestAgentDockerCompose),
	}
	if d.dockerComposeFile != "" {
		ymlPaths = []string{
			d.dockerComposeFile,
			filepath.Join(configDir, dockerTestAgentDockerCompose),
		}
	}

	composeProjectName := fmt.Sprintf("elastic-package-agent-%s", d.agentName())

	agent := dockerComposeDeployedAgent{
		ymlPaths: ymlPaths,
		project:  composeProjectName,
		variant: AgentVariant{
			Name: dockerTestAgentNamePrefix,
			Env:  env,
		},
	}

	agentInfo.ConfigDir = configDir

	p, err := compose.NewProject(agent.project, agent.ymlPaths...)
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
		logger.Debug("Skipping removing service logs folder folder %s", agentInfo.Logs.Folder.Local)
	} else {
		err = files.RemoveContent(agentInfo.Logs.Folder.Local)
		if err != nil {
			return nil, fmt.Errorf("removing service logs failed: %w", err)
		}
	}

	// Service name defined in the docker-compose file
	agentInfo.Name = dockerTestAgentNamePrefix
	agentName := agentInfo.Name

	opts := compose.CommandOptions{
		Env:       env,
		ExtraArgs: []string{"--build", "-d"},
	}

	if d.runTestsOnly || d.runTearDown {
		logger.Debug("Skipping bringing up docker-compose project and connect container to network (non setup steps)")
	} else {
		err = p.Up(ctx, opts)
		if err != nil {
			return nil, fmt.Errorf("could not boot up agent using Docker Compose: %w", err)
		}
		// Connect service network with stack network (for the purpose of metrics collection)
		err = docker.ConnectToNetwork(p.ContainerName(agentName), stack.Network(d.profile))
		if err != nil {
			return nil, fmt.Errorf("can't attach service container to the stack network: %w", err)
		}
	}

	// requires to be connected the service to the stack network
	err = p.WaitForHealthy(ctx, opts)
	if err != nil {
		processAgentContainerLogs(ctx, p, compose.CommandOptions{
			Env: opts.Env,
		}, agentName)
		return nil, fmt.Errorf("service is unhealthy: %w", err)
	}

	// Build agent container name
	// For those packages that require to do requests to agent ports in their tests (e.g. ti_anomali),
	// using the ContainerName of the agent (p.ContainerName(agentName)) as in servicedeployer does not work,
	// probably because it is in another compose project in case of ti_anomali?.
	agentInfo.Hostname = d.agentHostname()

	logger.Debugf("adding service container %s internal ports to context", p.ContainerName(agentName))
	serviceComposeConfig, err := p.Config(ctx, compose.CommandOptions{Env: env})
	if err != nil {
		return nil, fmt.Errorf("could not get Docker Compose configuration for service: %w", err)
	}

	s := serviceComposeConfig.Services[agentName]
	agentInfo.Ports = make([]int, len(s.Ports))
	for idx, port := range s.Ports {
		agentInfo.Ports[idx] = port.InternalPort
	}

	// Shortcut to first port for convenience
	if len(agentInfo.Ports) > 0 {
		agentInfo.Port = agentInfo.Ports[0]
	}

	agentInfo.Agent.Host.NamePrefix = agentInfo.Name
	agent.agentInfo = agentInfo
	return &agent, nil
}

func (d *CustomAgentDeployer) agentHostname() string {
	return fmt.Sprintf("%s-%s", dockerTestAgentNamePrefix, d.agentName())
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

// installDockerfile creates the files needed to run the custom elastic agent and returns
// the directory with these files.
func (d *CustomAgentDeployer) installDockerfile() (string, error) {
	customAgentDir := filepath.Join(d.profile.ProfilePath, fmt.Sprintf("agent-%s", d.agentName()))
	err := os.MkdirAll(customAgentDir, 0755)
	if err != nil {
		return "", fmt.Errorf("failed to create directory for custom agent files: %w", err)
	}

	customAgentDockerfile := filepath.Join(customAgentDir, dockerTestAgentDockerCompose)
	err = os.WriteFile(customAgentDockerfile, dockerAgentDockerComposeContent, 0644)
	if err != nil {
		return "", fmt.Errorf("failed to create docker compose file for custom agent: %w", err)
	}

	return customAgentDir, nil
}

func CreateServiceLogsDir(elasticPackagePath *locations.LocationManager, name string) (string, error) {
	dirPath := elasticPackagePath.ServiceLogDirPerAgent(name)
	err := os.MkdirAll(dirPath, 0755)
	if err != nil {
		return "", fmt.Errorf("mkdir failed (path: %s): %w", dirPath, err)
	}
	return dirPath, nil
}
