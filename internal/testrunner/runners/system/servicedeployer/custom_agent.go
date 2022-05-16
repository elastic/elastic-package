// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package servicedeployer

import (
	_ "embed"
	"os"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"

	"github.com/elastic/elastic-package/internal/install"
	"github.com/elastic/elastic-package/internal/logger"
)

//go:embed custom-agent-base-config.yml
var customAgentYml string

// CustomAgentDeployer knows how to deploy a custom elastic-agent defined via
// a Docker Compose file.
type CustomAgentDeployer struct {
	cfg string
	cd  *DockerComposeServiceDeployer
}

type deployedCustomAgent struct {
	cfg string
	*dockerComposeDeployedService
}

var _ DeployedService = &deployedCustomAgent{}

// NewCustomAgentDeployer returns a new instance of a deployedCustomAgent.
func NewCustomAgentDeployer(cfgPath string, ymlPaths []string, sv ServiceVariant) (*CustomAgentDeployer, error) {
	path, err := createCustomAgentYaml(cfgPath)
	if err != nil {
		return nil, err
	}

	appConfig, err := install.Configuration()
	if err != nil {
		return nil, errors.Wrap(err, "can't read application configuration")
	}

	cd, err := newDockerComposeServiceDeployer(
		append(ymlPaths, path),
		appConfig.StackImageRefs(install.DefaultStackVersion).AsEnv(),
		sv,
	)
	if err != nil {
		return nil, err
	}

	return &CustomAgentDeployer{
		cd:  cd,
		cfg: path,
	}, nil
}

// SetUp sets up the service and returns any relevant information.
func (d *CustomAgentDeployer) SetUp(inCtxt ServiceContext) (DeployedService, error) {
	ds, err := d.cd.SetUp(inCtxt)
	if err != nil {
		return nil, err
	}

	dds, ok := ds.(*dockerComposeDeployedService)
	if !ok {
		return ds, nil
	}

	dds.ctxt.Agent.Host.NamePrefix = inCtxt.Name

	return &deployedCustomAgent{
		dockerComposeDeployedService: dds,
		cfg:                          d.cfg,
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
		bc.Services["custom-agent"][k] = v
	}

	b, err := yaml.Marshal(bc)
	if err != nil {
		return "", errors.Wrap(err, "marshal custom-agent config")
	}

	tf, err := os.CreateTemp("", "custom-agent")
	if err != nil {
		return "", errors.Wrap(err, "create tmp file")
	}
	defer tf.Close()

	if _, err := tf.Write(b); err != nil {
		return "", errors.Wrap(err, "write custom-agent config")
	}

	return tf.Name(), nil
}
