package servicedeployer

import (
	"github.com/elastic/elastic-package/internal/common"
)

// ServiceDeployer defines the interface for deploying a service. It defines methods for
// controlling the lifecycle of a service.
type ServiceDeployer interface {
	// SetUp implements the logic for setting up a service. It takes a context and returns a
	// ServiceHandler.
	SetUp(ctxt common.ServiceContext) (DeployedService, error)
}
