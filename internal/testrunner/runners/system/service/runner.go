package service

// Ctx is a generic map for holding any relevant information, i.e. context, resulting from
// service setup, e.g. connection address information.
type Ctx map[string]interface{}

// Runner defines the interface for controlling a service. It defines methods for
// controlling the lifecycle of a service.
type Runner interface {
	// SetUp implements the logic for setting up a service. It return a context.
	SetUp() (Ctx, error)

	// TearDown implements the logic for tearing down a service. It accepts a context, same as the
	// one returned from the SetUp method.
	TearDown(Ctx) error
}
