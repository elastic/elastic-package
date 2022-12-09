// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package servicedeployer

import (
	_ "embed"
	"fmt"
	"os"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/compose"
	"github.com/elastic/elastic-package/internal/configuration/locations"
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
		return nil, errors.Wrap(err, "can't read application configuration")
	}

	kibanaClient, err := kibana.NewClient()
	if err != nil {
		return nil, errors.Wrap(err, "can't create Kibana client")
	}

	stackVersion, err := kibanaClient.Version()
	if err != nil {
		return nil, errors.Wrap(err, "can't read Kibana injected metadata")
	}

	caCertPath, ok := os.LookupEnv(stack.CACertificateEnv)
	if !ok {
		return nil, errors.Wrapf(err, "can't locate CA certificate: %s environment variable not set", stack.CACertificateEnv)
	}

	// Verify the Elastic stack network
	err = stack.EnsureStackNetworkUp()
	if err != nil {
		return nil, errors.Wrap(err, "Elastic stack network is not ready")
	}

	env := append(
		appConfig.StackImageRefs(stackVersion.Version()).AsEnv(),
		fmt.Sprintf("%s=%s", serviceLogsDirEnv, inCtxt.Logs.Folder.Local),
		fmt.Sprintf("%s=%s", localCACertEnv, caCertPath),
		fmt.Sprintf("ELASTIC_AGENT_TAGS=%s", inCtxt.CustomProperties["tags"]),
		fmt.Sprintf("CONTAINER_NAME=%s", inCtxt.Name),
		fmt.Sprintf("FLEET_TOKEN_POLICY_NAME=%s", inCtxt.CustomProperties["policy_name"]),
		fmt.Sprintf("ELASTIC_PACKAGE_STACK_NETWORK=%s", stack.Network()),
	)

	ymlPaths, err := d.loadComposeDefinitions()
	if err != nil {
		return nil, err
	}

	service := dockerComposeDeployedService{
		ymlPaths: ymlPaths,
		project:  "elastic-package-agents",
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

	// Clean service logs
	err = files.RemoveContent(outCtxt.Logs.Folder.Local)
	if err != nil {
		return nil, errors.Wrap(err, "removing service logs failed foo:"+outCtxt.Logs.Folder.Local)
	}

	serviceName := inCtxt.Name
	opts := compose.CommandOptions{
		Env:       env,
		ExtraArgs: []string{"--build", "-d"},
	}
	err = p.Up(opts)
	if err != nil {
		return nil, errors.Wrap(err, "could not boot up service using Docker Compose")
	}

	err = p.WaitForHealthy(opts)
	if err != nil {
		processServiceContainerLogs(p, compose.CommandOptions{
			Env: opts.Env,
		}, outCtxt.Name)
		return nil, errors.Wrap(err, "service is unhealthy")
	}

	// Build service container name
	outCtxt.Hostname = serviceName

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
	return &service, nil
}

func (d *CustomAgentDeployer) loadComposeDefinitions() ([]string, error) {
	locationManager, err := locations.NewLocationManager()
	if err != nil {
		return nil, errors.Wrap(err, "can't locate Docker Compose file for Custom Agent deployer")
	}

	if len(d.cfg) > 0 {
		return []string{d.cfg, locationManager.DockerCustomAgentDeployerYml()}, nil
	}
	return []string{locationManager.DockerCustomAgentDeployerYml()}, nil
}
