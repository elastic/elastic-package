// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package servicedeployer

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"time"

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
type DockerComposeServiceDeployer struct {
	profile  *profile.Profile
	ymlPaths []string
	variant  ServiceVariant

	deployIndependentAgent bool

	runTearDown  bool
	runTestsOnly bool

	logger *slog.Logger
}

type DockerComposeServiceDeployerOptions struct {
	Profile  *profile.Profile
	YmlPaths []string
	Variant  ServiceVariant

	DeployIndependentAgent bool

	RunTearDown  bool
	RunTestsOnly bool

	Logger *slog.Logger
}

type dockerComposeDeployedService struct {
	svcInfo ServiceInfo

	shutdownTimeout time.Duration

	ymlPaths []string
	project  string
	variant  ServiceVariant
	env      []string

	logger *slog.Logger
}

var _ ServiceDeployer = new(DockerComposeServiceDeployer)

// NewDockerComposeServiceDeployer returns a new instance of a DockerComposeServiceDeployer.
func NewDockerComposeServiceDeployer(options DockerComposeServiceDeployerOptions) (*DockerComposeServiceDeployer, error) {
	log := logger.Logger
	if options.Logger != nil {
		log = options.Logger
	}
	log = log.With(slog.String("service.deployer", "compose"))
	return &DockerComposeServiceDeployer{
		profile:                options.Profile,
		ymlPaths:               options.YmlPaths,
		variant:                options.Variant,
		runTearDown:            options.RunTearDown,
		runTestsOnly:           options.RunTestsOnly,
		deployIndependentAgent: options.DeployIndependentAgent,
		logger:                 log,
	}, nil
}

// SetUp sets up the service and returns any relevant information.
func (d *DockerComposeServiceDeployer) SetUp(ctx context.Context, svcInfo ServiceInfo) (DeployedService, error) {
	d.logger.Debug("setting up service using Docker Compose service deployer")
	service := dockerComposeDeployedService{
		ymlPaths: d.ymlPaths,
		project:  fmt.Sprintf("elastic-package-service-%s", svcInfo.Test.RunID),
		variant:  d.variant,
		env: []string{
			fmt.Sprintf("%s=%s", serviceLogsDirEnv, svcInfo.Logs.Folder.Local),
		},
		logger: d.logger,
	}

	p, err := compose.NewProject(compose.ProjectOptions{
		Name:   service.project,
		Paths:  service.ymlPaths,
		Logger: d.logger,
	})
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
		d.logger.Debug("Skipping removing service logs folder folder", slog.String("folder", svcInfo.Logs.Folder.Local))
	} else {
		err = files.RemoveContent(svcInfo.Logs.Folder.Local)
		if err != nil {
			return nil, fmt.Errorf("removing service logs failed: %w", err)
		}
	}

	// Boot up service
	if d.variant.active() {
		logger.Infof("Using service variant: %s", d.variant.String())
	}

	opts := compose.CommandOptions{
		Env: append(
			service.env,
			d.variant.Env...),
		ExtraArgs: []string{"--build", "-d"},
	}

	serviceName := svcInfo.Name
	if d.runTearDown || d.runTestsOnly {
		logger.Debug("Skipping bringing up docker-compose custom agent project")
	} else {
		err = p.Up(ctx, opts)
		if err != nil {
			return nil, fmt.Errorf("could not boot up service using Docker Compose: %w", err)
		}
	}

	err = p.WaitForHealthy(ctx, opts)
	if err != nil {
		processServiceContainerLogs(context.WithoutCancel(ctx), p, compose.CommandOptions{
			Env: opts.Env,
		}, svcInfo.Name, d.logger)
		return nil, fmt.Errorf("service is unhealthy: %w", err)
	}

	// Added a specific alias when connecting the service to the network.
	// - There could be container names too long that could not be resolved by the local DNS
	// - Not used serviceName directly as alias container, since there could be packages defining
	//   kibana or elasticsearch services and those DNS names are already present in the Elastic stack.
	//   This is mainly applicable when the Elastic Agent of the stack is used for testing.
	// - Keep the same alias for both implementations for consistency
	aliasContainer := fmt.Sprintf("svc-%s", serviceName)
	dk := docker.NewDocker(docker.WithLogger(d.logger))
	if d.runTearDown || d.runTestsOnly {
		logger.Debug("Skipping connect container to network (non setup steps)")
	} else {
		aliases := []string{
			aliasContainer,
		}
		if d.deployIndependentAgent {
			// Connect service network with agent network
			err = dk.ConnectToNetworkWithAlias(p.ContainerName(serviceName), svcInfo.AgentNetworkName, aliases)

			if err != nil {
				return nil, fmt.Errorf("can't attach service container to the agent network: %w", err)
			}
		} else {
			// Connect service network with stack network (for the purpose of metrics collection)
			err = dk.ConnectToNetworkWithAlias(p.ContainerName(serviceName), stack.Network(d.profile), aliases)
			if err != nil {
				return nil, fmt.Errorf("can't attach service container to the stack network: %w", err)
			}
		}
	}

	// Build service container name
	svcInfo.Hostname = aliasContainer

	d.logger.Debug("adding service container internal ports to context", slog.String("service", p.ContainerName(serviceName)))
	serviceComposeConfig, err := p.Config(ctx, compose.CommandOptions{
		Env: service.env,
	})
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

	svcInfo.Agent.Host.NamePrefix = "docker-fleet-agent"
	service.svcInfo = svcInfo
	return &service, nil
}

// Signal sends a signal to the service.
func (s *dockerComposeDeployedService) Signal(ctx context.Context, signal string) error {
	p, err := compose.NewProject(compose.ProjectOptions{
		Name:   s.project,
		Paths:  s.ymlPaths,
		Logger: s.logger,
	})
	if err != nil {
		return fmt.Errorf("could not create Docker Compose project for service: %w", err)
	}

	opts := compose.CommandOptions{
		Env: append(
			s.env,
			s.variant.Env...),
		ExtraArgs: []string{"-s", signal},
	}
	if s.svcInfo.Name != "" {
		opts.Services = append(opts.Services, s.svcInfo.Name)
	}

	err = p.Kill(ctx, opts)
	if err != nil {
		return fmt.Errorf("could not send %q signal: %w", signal, err)
	}
	return nil
}

// ExitCode returns true if the service is exited and its exit code.
func (s *dockerComposeDeployedService) ExitCode(ctx context.Context, service string) (bool, int, error) {
	p, err := compose.NewProject(compose.ProjectOptions{
		Name:   s.project,
		Paths:  s.ymlPaths,
		Logger: s.logger,
	})
	if err != nil {
		return false, -1, fmt.Errorf("could not create Docker Compose project for service: %w", err)
	}

	opts := compose.CommandOptions{
		Env: append(
			s.env,
			s.variant.Env...),
	}

	return p.ServiceExitCode(ctx, service, opts)
}

// TearDown tears down the service.
func (s *dockerComposeDeployedService) TearDown(ctx context.Context) error {
	s.logger.Debug("tearing down service using Docker Compose runner")
	defer func() {
		err := files.RemoveContent(s.svcInfo.Logs.Folder.Local)
		if err != nil {
			s.logger.Error("could not remove the service logs", slog.String("path", s.svcInfo.Logs.Folder.Local))
		}
		// Remove the outputs generated by the service container
		if err = os.RemoveAll(s.svcInfo.OutputDir); err != nil {
			s.logger.Error("could not remove the temporary output files", slog.Any("error", err))
		}
	}()

	p, err := compose.NewProject(compose.ProjectOptions{
		Name:   s.project,
		Paths:  s.ymlPaths,
		Logger: s.logger,
	})
	if err != nil {
		return fmt.Errorf("could not create Docker Compose project for service: %w", err)
	}

	opts := compose.CommandOptions{
		Env: append(
			s.env,
			s.variant.Env...),
	}

	extraArgs := []string{}
	// if not set "-t" , default shutdown timeout is 10 seconds
	// https://docs.docker.com/compose/faq/#why-do-my-services-take-10-seconds-to-recreate-or-stop
	if seconds := s.shutdownTimeout.Seconds(); seconds > 0 {
		extraArgs = append(extraArgs, "-t", fmt.Sprintf("%d", int(math.Round(seconds))))
	}
	if err := p.Stop(ctx, compose.CommandOptions{
		Env:       opts.Env,
		ExtraArgs: extraArgs,
	}); err != nil {
		return fmt.Errorf("could not stop service using Docker Compose: %w", err)
	}

	processServiceContainerLogs(ctx, p, opts, s.svcInfo.Name, s.logger)

	if err := p.Down(ctx, compose.CommandOptions{
		Env:       opts.Env,
		ExtraArgs: []string{"--volumes"}, // Remove associated volumes.
	}); err != nil {
		return fmt.Errorf("could not shut down service using Docker Compose: %w", err)
	}
	return nil
}

// Info returns the current context for the service.
func (s *dockerComposeDeployedService) Info() ServiceInfo {
	return s.svcInfo
}

// SetInfo sets the current context for the service.
func (s *dockerComposeDeployedService) SetInfo(ctxt ServiceInfo) error {
	s.svcInfo = ctxt
	return nil
}

func processServiceContainerLogs(ctx context.Context, p *compose.Project, opts compose.CommandOptions, serviceName string, l *slog.Logger) {
	content, err := p.Logs(ctx, opts)
	if err != nil {
		l.Error("can't export service logs", slog.Any("error", err))
		return
	}

	if len(content) == 0 {
		l.Info("service container hasn't written anything logs.")
		return
	}

	err = writeServiceContainerLogs(serviceName, content, l)
	if err != nil {
		l.Error("can't write service container logs", slog.Any("error", err))
	}
}

func writeServiceContainerLogs(serviceName string, content []byte, l *slog.Logger) error {
	buildDir, err := builder.BuildDirectory()
	if err != nil {
		return fmt.Errorf("locating build directory failed: %w", err)
	}

	containerLogsDir := filepath.Join(buildDir, "container-logs")
	err = os.MkdirAll(containerLogsDir, 0755)
	if err != nil {
		return fmt.Errorf("can't create directory for service container logs (path: %s): %w", containerLogsDir, err)
	}

	containerLogsFilepath := filepath.Join(containerLogsDir, fmt.Sprintf("%s-%d.log", serviceName, time.Now().UnixNano()))
	l.Info("Write container logs to file", slog.String("path", containerLogsFilepath))
	err = os.WriteFile(containerLogsFilepath, content, 0644)
	if err != nil {
		return fmt.Errorf("can't write container logs to file (path: %s): %w", containerLogsFilepath, err)
	}
	return nil
}
