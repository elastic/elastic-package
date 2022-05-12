// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package servicedeployer

import (
	_ "embed"
	"fmt"
	"os"

	"github.com/elastic/elastic-package/internal/compose"
	"github.com/elastic/elastic-package/internal/docker"
	"github.com/elastic/elastic-package/internal/files"
	"github.com/elastic/elastic-package/internal/install"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/stack"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

//go:embed custom-agent-base-config.yml
var customAgentYml string

// CustomAgentDeployer knows how to deploy a custom elastic-agent defined via
// a Docker Compose file.
type CustomAgentDeployer struct {
	cfg      string
	ymlPaths []string
}

type deployedCustomAgent struct {
	ctxt ServiceContext

	ymlPaths []string
	project  string
	cfg      string
	env      []string
}

// NewCustomAgentDeployer returns a new instance of a deployedCustomAgent.
func NewCustomAgentDeployer(cfgPath string, ymlPaths []string) (*CustomAgentDeployer, error) {
	path, err := createCustomAgentYaml(cfgPath)
	if err != nil {
		return nil, err
	}

	return &CustomAgentDeployer{
		ymlPaths: append(ymlPaths, path),
		cfg:      path,
	}, nil
}

// SetUp sets up the service and returns any relevant information.
func (d *CustomAgentDeployer) SetUp(inCtxt ServiceContext) (DeployedService, error) {
	logger.Debug("setting up service using Docker Compose service deployer")

	appConfig, err := install.Configuration()
	if err != nil {
		return nil, errors.Wrap(err, "can't read application configuration")
	}

	env := append(
		appConfig.StackImageRefs(install.DefaultStackVersion).AsEnv(),
		fmt.Sprintf("%s=%s", serviceLogsDirEnv, inCtxt.Logs.Folder.Local),
	)

	service := deployedCustomAgent{
		ymlPaths: d.ymlPaths,
		project:  "elastic-package-service",
		env:      env,
		cfg:      d.cfg,
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
	opts := compose.CommandOptions{
		Env:       env,
		ExtraArgs: []string{"--build", "-d"},
	}

	if err := p.Up(opts); err != nil {
		return nil, errors.Wrap(err, "could not boot up service using Docker Compose")
	}

	// Connect service network with stack network (for the purpose of metrics collection)
	err = docker.ConnectToNetwork(p.ContainerName(inCtxt.Name), stack.Network())
	if err != nil {
		return nil, errors.Wrapf(err, "can't attach service container to the stack network")
	}

	err = p.WaitForHealthy(opts)
	if err != nil {
		processServiceContainerLogs(p, compose.CommandOptions{
			Env: opts.Env,
		}, outCtxt.Name)
		return nil, errors.Wrap(err, "service is unhealthy")
	}

	// Build service container name
	outCtxt.Hostname = p.ContainerName(inCtxt.Name)

	logger.Debugf("adding service container %s internal ports to context", p.ContainerName(inCtxt.Name))
	serviceComposeConfig, err := p.Config(compose.CommandOptions{
		Env: opts.Env,
	})
	if err != nil {
		return nil, errors.Wrap(err, "could not get Docker Compose configuration for service")
	}

	s := serviceComposeConfig.Services[inCtxt.Name]
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

// Signal sends a signal to the service.
func (s *deployedCustomAgent) Signal(signal string) error {
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
func (s *deployedCustomAgent) TearDown() error {
	logger.Debugf("tearing down service using Docker Compose runner")
	defer func() {
		err := files.RemoveContent(s.ctxt.Logs.Folder.Local)
		if err != nil {
			logger.Errorf("could not remove the service logs (path: %s)", s.ctxt.Logs.Folder.Local)
		}

		if err := os.Remove(s.cfg); err != nil {
			logger.Errorf("cleaning up tmp file (path: %s): %v", s.cfg, err)
		}
	}()

	p, err := compose.NewProject(s.project, s.ymlPaths...)
	if err != nil {
		return errors.Wrap(err, "could not create Docker Compose project for service")
	}

	opts := compose.CommandOptions{
		Env: s.env,
	}
	processServiceContainerLogs(p, opts, s.ctxt.Name)

	if err := p.Down(compose.CommandOptions{
		Env:       s.env,
		ExtraArgs: []string{"--volumes"}, // Remove associated volumes.
	}); err != nil {
		return errors.Wrap(err, "could not shut down service using Docker Compose")
	}
	return nil
}

// Context returns the current context for the service.
func (s *deployedCustomAgent) Context() ServiceContext {
	return s.ctxt
}

// SetContext sets the current context for the service.
func (s *deployedCustomAgent) SetContext(ctxt ServiceContext) error {
	s.ctxt = ctxt
	return nil
}

func createCustomAgentYaml(cfgPath string) (string, error) {
	bc := struct {
		Version  string                            `yaml:"version"`
		Services map[string]map[string]interface{} `yaml:"services"`
	}{}
	if err := yaml.Unmarshal([]byte(customAgentYml), &bc); err != nil {
		return "", errors.Wrap(err, "unmarshal base custom-agent config")
	}

	cb, err := os.ReadFile(cfgPath)
	if err != nil {
		return "", errors.Wrap(err, "open custom-agent config")
	}

	cv := map[string]interface{}{}
	if err := yaml.Unmarshal(cb, &cv); err != nil {
		return "", errors.Wrap(err, "unmarshal custom-agent config")
	}

	for k, v := range cv {
		bc.Services["custom-agent"][k] = v
	}

	b, err := yaml.Marshal(bc)
	if err != nil {
		return "", errors.Wrap(err, "marshal custom-agent config")
	}

	dir, err := os.MkdirTemp("", "elastic-package")
	if err != nil {
		return "", errors.Wrap(err, "create tmp dir")
	}

	tf, err := os.CreateTemp(dir, "custom-agent")
	if err != nil {
		return "", errors.Wrap(err, "create tmp file")
	}
	defer tf.Close()

	if _, err := tf.Write(b); err != nil {
		return "", errors.Wrap(err, "write custom-agent config")
	}

	return tf.Name(), nil
}
