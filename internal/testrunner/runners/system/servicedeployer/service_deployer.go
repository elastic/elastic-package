package servicedeployer

import (
	"github.com/elastic/elastic-package/internal/common"
)

// ServiceDeployer defines the interface for deploying a service. It defines methods for
// controlling the lifecycle of a service.
type ServiceDeployer interface {
	// SetUp implements the logic for setting up a service. It takes a context and returns one that
	// may contain additional information. SetUp must not remove information from the context.
	SetUp(ctxt common.MapStr) (common.MapStr, error)

	// TearDown implements the logic for tearing down a service. It accepts a context, same as the
	// one returned from the SetUp method.
	TearDown(ctxt common.MapStr) error
}
