// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package servicedeployer

import (
	"fmt"
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

	runTearDown  bool
	runTestsOnly bool
}

type DockerComposeServiceDeployerOptions struct {
	Profile  *profile.Profile
	YmlPaths []string
	Variant  ServiceVariant

	RunTearDown  bool
	RunTestsOnly bool
}

type dockerComposeDeployedService struct {
	ctxt ServiceContext

	ymlPaths []string
	project  string
	variant  ServiceVariant
	env      []string
}

var _ ServiceDeployer = new(DockerComposeServiceDeployer)

// NewDockerComposeServiceDeployer returns a new instance of a DockerComposeServiceDeployer.
func NewDockerComposeServiceDeployer(options DockerComposeServiceDeployerOptions) (*DockerComposeServiceDeployer, error) {
	return &DockerComposeServiceDeployer{
		profile:      options.Profile,
		ymlPaths:     options.YmlPaths,
		variant:      options.Variant,
		runTearDown:  options.RunTearDown,
		runTestsOnly: options.RunTestsOnly,
	}, nil
}

// SetUp sets up the service and returns any relevant information.
func (d *DockerComposeServiceDeployer) SetUp(inCtxt ServiceContext) (DeployedService, error) {
	logger.Debug("setting up service using Docker Compose service deployer")
	service := dockerComposeDeployedService{
		ymlPaths: d.ymlPaths,
		project:  "elastic-package-service",
		variant:  d.variant,
		env:      []string{fmt.Sprintf("%s=%s", serviceLogsDirEnv, inCtxt.Logs.Folder.Local)},
	}
	outCtxt := inCtxt

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
		logger.Debug("Skipping removing service logs folder folder %s", outCtxt.Logs.Folder.Local)
	} else {
		err = files.RemoveContent(outCtxt.Logs.Folder.Local)
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

	serviceName := inCtxt.Name
	if d.runTearDown || d.runTestsOnly {
		logger.Debug("Skipping bringing up docker-compose custom agent project")
	} else {
		err = p.Up(opts)
		if err != nil {
			return nil, fmt.Errorf("could not boot up service using Docker Compose: %w", err)
		}
	}

	err = p.WaitForHealthy(opts)
	if err != nil {
		processServiceContainerLogs(p, compose.CommandOptions{
			Env: opts.Env,
		}, outCtxt.Name)
		return nil, fmt.Errorf("service is unhealthy: %w", err)
	}

	if d.runTearDown || d.runTestsOnly {
		logger.Debug("Skipping connect container to network (non setup steps)")
	} else {
		// Connect service network with stack network (for the purpose of metrics collection)
		err = docker.ConnectToNetwork(p.ContainerName(serviceName), stack.Network(d.profile))
		if err != nil {
			return nil, fmt.Errorf("can't attach service container to the stack network: %w", err)
		}
	}

	// Build service container name
	outCtxt.Hostname = p.ContainerName(serviceName)

	logger.Debugf("adding service container %s internal ports to context", p.ContainerName(serviceName))
	serviceComposeConfig, err := p.Config(compose.CommandOptions{
		Env: []string{fmt.Sprintf("%s=%s", serviceLogsDirEnv, outCtxt.Logs.Folder.Local)},
	})
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

	outCtxt.Agent.Host.NamePrefix = "docker-fleet-agent"
	service.ctxt = outCtxt
	return &service, nil
}

// Signal sends a signal to the service.
func (s *dockerComposeDeployedService) Signal(signal string) error {
	p, err := compose.NewProject(s.project, s.ymlPaths...)
	if err != nil {
		return fmt.Errorf("could not create Docker Compose project for service: %w", err)
	}

	opts := compose.CommandOptions{
		Env: append(
			s.env,
			s.variant.Env...),
		ExtraArgs: []string{"-s", signal},
	}
	if s.ctxt.Name != "" {
		opts.Services = append(opts.Services, s.ctxt.Name)
	}

	err = p.Kill(opts)
	if err != nil {
		return fmt.Errorf("could not send %q signal: %w", signal, err)
	}
	return nil
}

// ExitCode returns true if the service is exited and its exit code.
func (s *dockerComposeDeployedService) ExitCode(service string) (bool, int, error) {
	p, err := compose.NewProject(s.project, s.ymlPaths...)
	if err != nil {
		return false, -1, fmt.Errorf("could not create Docker Compose project for service: %w", err)
	}

	opts := compose.CommandOptions{
		Env: append(
			s.env,
			s.variant.Env...),
	}

	return p.ServiceExitCode(service, opts)
}

// TearDown tears down the service.
func (s *dockerComposeDeployedService) TearDown() error {
	logger.Debugf("tearing down service using Docker Compose runner")
	defer func() {
		err := files.RemoveContent(s.ctxt.Logs.Folder.Local)
		if err != nil {
			logger.Errorf("could not remove the service logs (path: %s)", s.ctxt.Logs.Folder.Local)
		}
		// Remove the outputs generated by the service container
		if err = os.RemoveAll(s.ctxt.OutputDir); err != nil {
			logger.Errorf("could not remove the temporary output files %w", err)
		}
	}()

	p, err := compose.NewProject(s.project, s.ymlPaths...)
	if err != nil {
		return fmt.Errorf("could not create Docker Compose project for service: %w", err)
	}

	opts := compose.CommandOptions{
		Env: append(
			s.env,
			s.variant.Env...),
	}
	if err := p.Stop(compose.CommandOptions{
		Env:       opts.Env,
		ExtraArgs: []string{"-t", "300"}, // default shutdown timeout 10 seconds
	}); err != nil {
		return fmt.Errorf("could not stop service using Docker Compose: %w", err)
	}

	processServiceContainerLogs(p, opts, s.ctxt.Name)

	if err := p.Down(compose.CommandOptions{
		Env:       opts.Env,
		ExtraArgs: []string{"--volumes"}, // Remove associated volumes.
	}); err != nil {
		return fmt.Errorf("could not shut down service using Docker Compose: %w", err)
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

var _ DeployedService = new(dockerComposeDeployedService)

func processServiceContainerLogs(p *compose.Project, opts compose.CommandOptions, serviceName string) {
	content, err := p.Logs(opts)
	if err != nil {
		logger.Errorf("can't export service logs: %v", err)
		return
	}

	if len(content) == 0 {
		logger.Info("service container hasn't written anything logs.")
		return
	}

	err = writeServiceContainerLogs(serviceName, content)
	if err != nil {
		logger.Errorf("can't write service container logs: %v", err)
	}
}

func writeServiceContainerLogs(serviceName string, content []byte) error {
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
	logger.Infof("Write container logs to file: %s", containerLogsFilepath)
	err = os.WriteFile(containerLogsFilepath, content, 0644)
	if err != nil {
		return fmt.Errorf("can't write container logs to file (path: %s): %w", containerLogsFilepath, err)
	}
	return nil
}
