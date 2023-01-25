// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package servicedeployer

import (
	_ "embed"
	"fmt"
	"os"

	"github.com/elastic/elastic-package/internal/compose"
	"github.com/elastic/elastic-package/internal/configuration/locations"
	"github.com/elastic/elastic-package/internal/docker"
	"github.com/elastic/elastic-package/internal/files"
	"github.com/elastic/elastic-package/internal/install"
	"github.com/elastic/elastic-package/internal/kibana"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/stack"
)

const dockerCustomAgentName = "docker-custom-agent"

// CustomAgentDeployer knows how to deploy a custom elastic-agent defined via
// a Docker Compose file.
type CustomAgentDeployer struct {
	cfg string
}

// NewCustomAgentDeployer returns a new instance of a deployedCustomAgent.
func NewCustomAgentDeployer(cfgPath string) (*CustomAgentDeployer, error) {
	return &CustomAgentDeployer{
		cfg: cfgPath,
	}, nil
}

// SetUp sets up the service and returns any relevant information.
func (d *CustomAgentDeployer) SetUp(inCtxt ServiceContext) (DeployedService, error) {
	logger.Debug("setting up service using Docker Compose service deployer")

	appConfig, err := install.Configuration()
	if err != nil {
		return nil, fmt.Errorf("can't read application configuration: %s", err)
	}

	kibanaClient, err := kibana.NewClient()
	if err != nil {
		return nil, fmt.Errorf("can't create Kibana client: %s", err)
	}

	stackVersion, err := kibanaClient.Version()
	if err != nil {
		return nil, fmt.Errorf("can't read Kibana injected metadata: %s", err)
	}

	caCertPath, ok := os.LookupEnv(stack.CACertificateEnv)
	if !ok {
		return nil, fmt.Errorf("can't locate CA certificate: %s environment variable not set: %s", stack.CACertificateEnv, err)
	}

	env := append(
		appConfig.StackImageRefs(stackVersion.Version()).AsEnv(),
		fmt.Sprintf("%s=%s", serviceLogsDirEnv, inCtxt.Logs.Folder.Local),
		fmt.Sprintf("%s=%s", localCACertEnv, caCertPath),
	)

	ymlPaths, err := d.loadComposeDefinitions()
	if err != nil {
		return nil, err
	}

	service := dockerComposeDeployedService{
		ymlPaths: ymlPaths,
		project:  "elastic-package-service",
		sv: ServiceVariant{
			Name: dockerCustomAgentName,
			Env:  env,
		},
	}

	outCtxt := inCtxt

	p, err := compose.NewProject(service.project, service.ymlPaths...)
	if err != nil {
		return nil, fmt.Errorf("could not create Docker Compose project for service: %s", err)
	}

	// Verify the Elastic stack network
	err = stack.EnsureStackNetworkUp()
	if err != nil {
		return nil, fmt.Errorf("elastic stack network is not ready: %s", err)
	}

	// Clean service logs
	err = files.RemoveContent(outCtxt.Logs.Folder.Local)
	if err != nil {
		return nil, fmt.Errorf("removing service logs failed: %s", err)
	}

	inCtxt.Name = dockerCustomAgentName
	serviceName := inCtxt.Name
	opts := compose.CommandOptions{
		Env:       env,
		ExtraArgs: []string{"--build", "-d"},
	}
	err = p.Up(opts)
	if err != nil {
		return nil, fmt.Errorf("could not boot up service using Docker Compose: %s", err)
	}

	// Connect service network with stack network (for the purpose of metrics collection)
	err = docker.ConnectToNetwork(p.ContainerName(serviceName), stack.Network())
	if err != nil {
		return nil, fmt.Errorf("can't attach service container to the stack network: %s", err)
	}

	err = p.WaitForHealthy(opts)
	if err != nil {
		processServiceContainerLogs(p, compose.CommandOptions{
			Env: opts.Env,
		}, outCtxt.Name)
		return nil, fmt.Errorf("service is unhealthy: %s", err)
	}

	// Build service container name
	outCtxt.Hostname = p.ContainerName(serviceName)

	logger.Debugf("adding service container %s internal ports to context", p.ContainerName(serviceName))
	serviceComposeConfig, err := p.Config(compose.CommandOptions{Env: env})
	if err != nil {
		return nil, fmt.Errorf("could not get Docker Compose configuration for service: %s", err)
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

func (d *CustomAgentDeployer) loadComposeDefinitions() ([]string, error) {
	locationManager, err := locations.NewLocationManager()
	if err != nil {
		return nil, fmt.Errorf("can't locate Docker Compose file for Custom Agent deployer: %s", err)
	}
	return []string{d.cfg, locationManager.DockerCustomAgentDeployerYml()}, nil
}
