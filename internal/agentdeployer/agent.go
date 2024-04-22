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
	"text/template"
	"time"

	"github.com/elastic/elastic-package/internal/compose"
	"github.com/elastic/elastic-package/internal/docker"
	"github.com/elastic/elastic-package/internal/files"
	"github.com/elastic/elastic-package/internal/install"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/profile"
	"github.com/elastic/elastic-package/internal/stack"
)

const (
	dockerTestAgentNamePrefix    = "elastic-agent"
	dockerTestAgentDockerCompose = "docker-agent-base.yml"
	defaultAgentPolicyName       = "Elastic-Agent (elastic-package)"
)

//go:embed _static/docker-agent-base.yml.tmpl
var dockerTestAgentDockerComposeTemplate string

// CustomAgentDeployer knows how to deploy a custom elastic-agent defined via
// a Docker Compose file.
type DockerComposeAgentDeployer struct {
	profile           *profile.Profile
	dockerComposeFile string
	stackVersion      string

	policyName string

	agentRunID string

	packageName string
	dataStream  string

	runTearDown  bool
	runTestsOnly bool
}

type DockerComposeAgentDeployerOptions struct {
	Profile           *profile.Profile
	DockerComposeFile string
	StackVersion      string
	PolicyName        string

	PackageName string
	DataStream  string

	RunTearDown  bool
	RunTestsOnly bool
}

var _ AgentDeployer = new(DockerComposeAgentDeployer)

type dockerComposeDeployedAgent struct {
	agentInfo AgentInfo

	ymlPaths []string
	project  string
	env      []string
}

var _ DeployedAgent = new(dockerComposeDeployedAgent)

// NewCustomAgentDeployer returns a new instance of a deployedCustomAgent.
func NewCustomAgentDeployer(options DockerComposeAgentDeployerOptions) (*DockerComposeAgentDeployer, error) {
	return &DockerComposeAgentDeployer{
		profile:           options.Profile,
		dockerComposeFile: options.DockerComposeFile,
		stackVersion:      options.StackVersion,
		packageName:       options.PackageName,
		dataStream:        options.DataStream,
		policyName:        options.PolicyName,
		runTearDown:       options.RunTearDown,
		runTestsOnly:      options.RunTestsOnly,
	}, nil
}

// SetUp sets up the service and returns any relevant information.
func (d *DockerComposeAgentDeployer) SetUp(ctx context.Context, agentInfo AgentInfo) (DeployedAgent, error) {
	logger.Debug("setting up agent using Docker Compose agent deployer")
	d.agentRunID = agentInfo.Test.RunID

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
		fmt.Sprintf("%s=%s", serviceLogsDirEnv, agentInfo.Logs.Folder.Local),
		fmt.Sprintf("%s=%s", localCACertEnv, caCertPath),
		fmt.Sprintf("%s=%s", fleetPolicyEnv, d.policyName),
		fmt.Sprintf("%s=%s", agentHostnameEnv, d.agentHostname()),
	)

	configDir, err := d.installDockerfile(agentInfo)
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
		env:      env,
	}

	agentInfo.ConfigDir = configDir
	agentInfo.NetworkName = fmt.Sprintf("%s_default", composeProjectName)

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
		logger.Debugf("Skipping removing service logs folder %s", agentInfo.Logs.Folder.Local)
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
			return nil, fmt.Errorf("can't attach agent container to the stack network: %w", err)
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

func (d *DockerComposeAgentDeployer) agentHostname() string {
	return fmt.Sprintf("%s-%s-%s", dockerTestAgentNamePrefix, d.agentName(), d.agentRunID)
}

func (d *DockerComposeAgentDeployer) agentName() string {
	name := d.packageName
	if d.dataStream != "" && d.dataStream != "." {
		name = fmt.Sprintf("%s-%s", name, d.dataStream)
	}
	return name
}

// installDockerfile creates the files needed to run the custom elastic agent and returns
// the directory with these files.
func (d *DockerComposeAgentDeployer) installDockerfile(agentInfo AgentInfo) (string, error) {
	customAgentDir, err := CreateDeployerDir(d.profile, fmt.Sprintf("docker-agent-%s-%s", d.agentName(), d.agentRunID))
	if err != nil {
		return "", fmt.Errorf("failed to create directory for custom agent files: %w", err)
	}

	customAgentDockerfile := filepath.Join(customAgentDir, dockerTestAgentDockerCompose)
	file, err := os.Create(customAgentDockerfile)
	if err != nil {
		return "", fmt.Errorf("failed to create file (name %s): %w", customAgentDockerfile, err)
	}
	defer file.Close()

	tmpl := template.Must(template.New(dockerTestAgentDockerCompose).Parse(dockerTestAgentDockerComposeTemplate))
	err = tmpl.Execute(file, map[string]any{
		"user":         agentInfo.Agent.User,
		"capabilities": agentInfo.Agent.LinuxCapabilities,
		"runtime":      agentInfo.Agent.Runtime,
		"pidMode":      agentInfo.Agent.PidMode,
	})
	if err != nil {
		return "", fmt.Errorf("failed to create contents of the docker-compose file %q: %w", customAgentDockerfile, err)
	}

	return customAgentDir, nil
}

// ExitCode returns true if the agent is exited and its exit code.
func (s *dockerComposeDeployedAgent) ExitCode(ctx context.Context) (bool, int, error) {
	p, err := compose.NewProject(s.project, s.ymlPaths...)
	if err != nil {
		return false, -1, fmt.Errorf("could not create Docker Compose project for agent: %w", err)
	}

	opts := compose.CommandOptions{Env: s.env}

	return p.ServiceExitCode(ctx, s.agentInfo.Name, opts)
}

// Logs returns the logs from the agent starting at the given time
func (s *dockerComposeDeployedAgent) Logs(ctx context.Context, t time.Time) ([]byte, error) {
	p, err := compose.NewProject(s.project, s.ymlPaths...)
	if err != nil {
		return nil, fmt.Errorf("could not create Docker Compose project for agent: %w", err)
	}

	opts := compose.CommandOptions{Env: s.env}

	return p.Logs(ctx, opts)
}

// TearDown tears down the agent.
func (s *dockerComposeDeployedAgent) TearDown(ctx context.Context) error {
	logger.Debugf("tearing down agent using Docker Compose runner")
	defer func() {
		// Remove the service logs dir for this agent
		if err := os.RemoveAll(s.agentInfo.Logs.Folder.Local); err != nil {
			logger.Errorf("could not remove the agent logs (path: %s): %w", s.agentInfo.Logs.Folder.Local, err)
		}

		// Remove the configuration dir for this agent (e.g. compose scenario files)
		if err := os.RemoveAll(s.agentInfo.ConfigDir); err != nil {
			logger.Errorf("could not remove the agent configuration directory (path: %s) %w", s.agentInfo.ConfigDir, err)
		}
	}()

	p, err := compose.NewProject(s.project, s.ymlPaths...)
	if err != nil {
		return fmt.Errorf("could not create Docker Compose project for service: %w", err)
	}

	opts := compose.CommandOptions{Env: s.env}
	processAgentContainerLogs(ctx, p, opts, s.agentInfo.Name)

	if err := p.Down(ctx, compose.CommandOptions{
		Env:       opts.Env,
		ExtraArgs: []string{"--volumes"}, // Remove associated volumes.
	}); err != nil {
		return fmt.Errorf("could not shut down agent using Docker Compose: %w", err)
	}
	return nil
}

// Info returns the current context for the agent.
func (s *dockerComposeDeployedAgent) Info() AgentInfo {
	return s.agentInfo
}

// SetInfo sets the current context for the agent.
func (s *dockerComposeDeployedAgent) SetInfo(info AgentInfo) {
	s.agentInfo = info
}
