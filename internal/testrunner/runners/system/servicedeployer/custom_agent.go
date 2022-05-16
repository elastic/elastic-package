// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package servicedeployer

import (
	_ "embed"
	"fmt"
	"os"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"

	"github.com/elastic/elastic-package/internal/compose"
	"github.com/elastic/elastic-package/internal/docker"
	"github.com/elastic/elastic-package/internal/files"
	"github.com/elastic/elastic-package/internal/install"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/stack"
)

//go:embed docker-agent-base-config.yml
var customAgentYml string

const dockerCustomAgentName = "docker-custom-agent"

// CustomAgentDeployer knows how to deploy a custom elastic-agent defined via
// a Docker Compose file.
type CustomAgentDeployer struct {
	cfg string
}

type deployedCustomAgent struct {
	cfg string
	env []string
	*dockerComposeDeployedService
}

// NewCustomAgentDeployer returns a new instance of a deployedCustomAgent.
func NewCustomAgentDeployer(cfgPath string) (*CustomAgentDeployer, error) {
	path, err := createCustomAgentYaml(cfgPath)
	if err != nil {
		return nil, err
	}

	return &CustomAgentDeployer{
		cfg: path,
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

	service := dockerComposeDeployedService{
		ymlPaths: []string{d.cfg},
		project:  "elastic-package-service",
		sv: ServiceVariant{
			Name: dockerCustomAgentName,
			Env:  env,
		},
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

	inCtxt.Name = dockerCustomAgentName
	serviceName := inCtxt.Name
	opts := compose.CommandOptions{
		Env:       env,
		ExtraArgs: []string{"--build", "-d"},
	}
	err = p.Up(opts)
	if err != nil {
		return nil, errors.Wrap(err, "could not boot up service using Docker Compose")
	}

	// Connect service network with stack network (for the purpose of metrics collection)
	err = docker.ConnectToNetwork(p.ContainerName(serviceName), stack.Network())
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
	outCtxt.Hostname = p.ContainerName(serviceName)

	logger.Debugf("adding service container %s internal ports to context", p.ContainerName(serviceName))
	serviceComposeConfig, err := p.Config(compose.CommandOptions{Env: env})
	if err != nil {
		return nil, errors.Wrap(err, "could not get Docker Compose configuration for service")
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
	return &deployedCustomAgent{
		dockerComposeDeployedService: &service,
		cfg:                          d.cfg,
		env:                          env,
	}, nil
}

// TearDown tears down the service.
func (s *deployedCustomAgent) TearDown() error {
	defer func() {
		if err := os.Remove(s.cfg); err != nil {
			logger.Errorf("cleaning up tmp file (path: %s): %v", s.cfg, err)
		}
	}()
	return s.dockerComposeDeployedService.TearDown()
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
		bc.Services[dockerCustomAgentName][k] = v
	}
	bc.Services[dockerCustomAgentName]["hostname"] = dockerCustomAgentName

	b, err := yaml.Marshal(bc)
	if err != nil {
		return "", errors.Wrap(err, "marshal custom-agent config")
	}

	tf, err := os.CreateTemp("", dockerCustomAgentName)
	if err != nil {
		return "", errors.Wrap(err, "create tmp file")
	}
	defer tf.Close()

	if _, err := tf.Write(b); err != nil {
		return "", errors.Wrap(err, "write custom-agent config")
	}

	return tf.Name(), nil
}
