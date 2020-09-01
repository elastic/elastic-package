// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package servicedeployer

import (
	"github.com/elastic/elastic-package/internal/common"
)

// DeployedService defines the interface for interacting with a service that has been deployed.
type DeployedService interface {
	// TearDown implements the logic for tearing down a service.
	TearDown() error

	// GetContext returns the current context from the service.
	GetContext() common.ServiceContext

	// SetContext sets the current context for the service.
	SetContext(str common.ServiceContext) error
}
