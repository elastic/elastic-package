// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package servicedeployer

// ServiceDeployer defines the interface for deploying a service. It defines methods for
// controlling the lifecycle of a service.
type ServiceDeployer interface {
	// SetUp implements the logic for setting up a service. It takes a context and returns a
	// ServiceHandler.
	SetUp(ctxt ServiceContext) (DeployedService, error)
}
