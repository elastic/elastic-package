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
