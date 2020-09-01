package servicedeployer

import (
	"github.com/elastic/elastic-package/internal/common"
)

// ServiceDeployer defines the interface for deploying a service. It defines methods for
// controlling the lifecycle of a service.
type ServiceDeployer interface {
	// SetUp implements the logic for setting up a service. It takes a context and returns a
	// ServiceHandler.
	SetUp(ctxt common.MapStr) (DeployedService, error)
}

// DeployedService defines the interface for interacting with a service that has been deployed.
type DeployedService interface {
	// TearDown implements the logic for tearing down a service.
	TearDown() error

	// GetContext returns the current context from the service.
	GetContext() common.MapStr

	// SetContext sets the current context for the service.
	SetContext(str common.MapStr) error
}
