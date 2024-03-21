// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.
package agentdeployer

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/net/context"

	"github.com/elastic/elastic-package/internal/builder"
	"github.com/elastic/elastic-package/internal/compose"
	"github.com/elastic/elastic-package/internal/docker"
	"github.com/elastic/elastic-package/internal/files"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/profile"
	"github.com/elastic/elastic-package/internal/stack"
)

// DockerComposeServiceDeployer knows how to deploy a service defined via
// a Docker Compose file.
type DockerComposeAgentDeployer struct {
	profile  *profile.Profile
	ymlPaths []string
	variant  AgentVariant
}

type dockerComposeDeployedAgent struct {
	agentInfo AgentInfo

	ymlPaths []string
	project  string
	variant  AgentVariant
	env      []string
}

// NewDockerComposeAgentDeployer returns a new instance of a DockerComposeAgentDeployer.
func NewDockerComposeAgentDeployer(profile *profile.Profile, ymlPaths []string) (*DockerComposeAgentDeployer, error) {
	return &DockerComposeAgentDeployer{
		profile:  profile,
		ymlPaths: ymlPaths,
	}, nil
}

var _ AgentDeployer = new(DockerComposeAgentDeployer)

// SetUp sets up the service and returns any relevant information.
func (d *DockerComposeAgentDeployer) SetUp(ctx context.Context, agentInfo AgentInfo) (DeployedAgent, error) {
	logger.Debug("setting up agent using Docker Compose agent deployer")
	agent := dockerComposeDeployedAgent{
		ymlPaths: d.ymlPaths,
		project:  "elastic-package-agent",
		variant:  d.variant,
		env:      []string{fmt.Sprintf("%s=%s", serviceLogsDirEnv, agentInfo.Logs.Folder.Local)},
	}

	p, err := compose.NewProject(agent.project, agent.ymlPaths...)
	if err != nil {
		return nil, fmt.Errorf("could not create Docker Compose project for service: %w", err)
	}

	// Verify the Elastic stack network
	err = stack.EnsureStackNetworkUp(d.profile)
	if err != nil {
		return nil, fmt.Errorf("stack network is not ready: %w", err)
	}

	// Clean service logs
	err = files.RemoveContent(agentInfo.Logs.Folder.Local)
	if err != nil {
		return nil, fmt.Errorf("removing service logs failed: %w", err)
	}

	// Boot up service
	if d.variant.active() {
		logger.Infof("Using service variant: %s", d.variant.String())
	}

	agentName := agentInfo.Name
	opts := compose.CommandOptions{
		Env: append(
			agent.env,
			d.variant.Env...,
		),
		ExtraArgs: []string{"--build", "-d"},
	}
	err = p.Up(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("could not boot up service using Docker Compose: %w", err)
	}

	err = p.WaitForHealthy(ctx, opts)
	if err != nil {
		processAgentContainerLogs(ctx, p, compose.CommandOptions{
			Env: opts.Env,
		}, agentName)
		return nil, fmt.Errorf("service is unhealthy: %w", err)
	}

	// Build agent container name
	agentInfo.Hostname = p.ContainerName(agentName)

	// Connect service network with stack network (for the purpose of metrics collection)
	err = docker.ConnectToNetwork(p.ContainerName(agentName), stack.Network(d.profile))
	if err != nil {
		return nil, fmt.Errorf("can't attach service container to the stack network: %w", err)
	}

	logger.Debugf("adding agent container %s internal ports to context", p.ContainerName(agentName))
	agentComposeConfig, err := p.Config(ctx, compose.CommandOptions{
		Env: []string{fmt.Sprintf("%s=%s", serviceLogsDirEnv, agentInfo.Logs.Folder.Local)},
	})
	if err != nil {
		return nil, fmt.Errorf("could not get Docker Compose configuration for agent: %w", err)
	}

	s := agentComposeConfig.Services[agentName]
	agentInfo.Ports = make([]int, len(s.Ports))
	for idx, port := range s.Ports {
		agentInfo.Ports[idx] = port.InternalPort
	}

	// Shortcut to first port for convenience
	if len(agentInfo.Ports) > 0 {
		agentInfo.Port = agentInfo.Ports[0]
	}

	agentInfo.Agent.Host.NamePrefix = "docker-custom-agent"
	agent.agentInfo = agentInfo
	return &agent, nil
}

// Signal sends a signal to the agent.
func (s *dockerComposeDeployedAgent) Signal(ctx context.Context, signal string) error {
	p, err := compose.NewProject(s.project, s.ymlPaths...)
	if err != nil {
		return fmt.Errorf("could not create Docker Compose project for service: %w", err)
	}

	opts := compose.CommandOptions{
		Env: append(
			s.env,
			s.variant.Env...,
		),
		ExtraArgs: []string{"-s", signal},
	}
	if s.agentInfo.Name != "" {
		opts.Services = append(opts.Services, s.agentInfo.Name)
	}

	err = p.Kill(ctx, opts)
	if err != nil {
		return fmt.Errorf("could not send %q signal: %w", signal, err)
	}
	return nil
}

// ExitCode returns true if the agent is exited and its exit code.
func (s *dockerComposeDeployedAgent) ExitCode(ctx context.Context, agent string) (bool, int, error) {
	p, err := compose.NewProject(s.project, s.ymlPaths...)
	if err != nil {
		return false, -1, fmt.Errorf("could not create Docker Compose project for agent: %w", err)
	}

	opts := compose.CommandOptions{
		Env: append(
			s.env,
			s.variant.Env...,
		),
	}

	return p.ServiceExitCode(ctx, agent, opts)
}

// TearDown tears down the agent.
func (s *dockerComposeDeployedAgent) TearDown(ctx context.Context) error {
	logger.Debugf("tearing down agent using Docker Compose runner")
	defer func() {
		err := files.RemoveContent(s.agentInfo.Logs.Folder.Local)
		if err != nil {
			logger.Errorf("could not remove the agent logs (path: %s)", s.agentInfo.Logs.Folder.Local)
		}
		// Remove the outputs generated by the service container
		if err = os.RemoveAll(s.agentInfo.OutputDir); err != nil {
			logger.Errorf("could not remove the temporary output files %w", err)
		}

		// Remove the configuration dir (e.g. compose scenario files)
		if err = os.RemoveAll(s.agentInfo.ConfigDir); err != nil {
			logger.Errorf("could not remove the agent configuration directory %w", err)
		}
	}()

	p, err := compose.NewProject(s.project, s.ymlPaths...)
	if err != nil {
		return fmt.Errorf("could not create Docker Compose project for service: %w", err)
	}

	opts := compose.CommandOptions{
		Env: append(
			s.env,
			s.variant.Env...,
		),
	}
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
func (s *dockerComposeDeployedAgent) SetInfo(ctxt AgentInfo) error {
	s.agentInfo = ctxt
	return nil
}

var _ DeployedAgent = new(dockerComposeDeployedAgent)

func processAgentContainerLogs(ctx context.Context, p *compose.Project, opts compose.CommandOptions, agentName string) {
	content, err := p.Logs(ctx, opts)
	if err != nil {
		logger.Errorf("can't export service logs: %v", err)
		return
	}

	if len(content) == 0 {
		logger.Info("service container hasn't written anything logs.")
		return
	}

	err = writeAgentContainerLogs(agentName, content)
	if err != nil {
		logger.Errorf("can't write service container logs: %v", err)
	}
}

func writeAgentContainerLogs(agentName string, content []byte) error {
	buildDir, err := builder.BuildDirectory()
	if err != nil {
		return fmt.Errorf("locating build directory failed: %w", err)
	}

	containerLogsDir := filepath.Join(buildDir, "container-logs")
	err = os.MkdirAll(containerLogsDir, 0o755)
	if err != nil {
		return fmt.Errorf("can't create directory for agent container logs (path: %s): %w", containerLogsDir, err)
	}

	containerLogsFilepath := filepath.Join(containerLogsDir, fmt.Sprintf("%s-%d.log", agentName, time.Now().UnixNano()))
	logger.Infof("Write container logs to file: %s", containerLogsFilepath)
	err = os.WriteFile(containerLogsFilepath, content, 0o644)
	if err != nil {
		return fmt.Errorf("can't write container logs to file (path: %s): %w", containerLogsFilepath, err)
	}
	return nil
}
