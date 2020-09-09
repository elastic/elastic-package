// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package servicedeployer

// DeployedService defines the interface for interacting with a service that has been deployed.
type DeployedService interface {
	// TearDown implements the logic for tearing down a service.
	TearDown() error

	// Context returns the current context from the service.
	Context() ServiceContext

	// SetContext sets the current context for the service.
	SetContext(str ServiceContext) error
}
