// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package servicedeployer

// CustomAgentDeployer knows how to deploy a custom elastic-agent defined via
// a Docker Compose file.
type CustomAgentDeployer struct {
	cd *DockerComposeServiceDeployer
}

// NewCustomAgentDeployer returns a new instance of a deployedCustomAgent.
func NewCustomAgentDeployer(ymlPaths []string) (*CustomAgentDeployer, error) {
	cd, _ := NewDockerComposeServiceDeployer(ymlPaths, ServiceVariant{})
	return &CustomAgentDeployer{
		cd: cd,
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

	return dds, nil
}
