// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package agentdeployer

import (
	"context"
	"errors"
)

var ErrNotSupported error = errors.New("not supported")

// DeployedAgent defines the interface for interacting with a service that has been deployed.
type DeployedAgent interface {
	// TearDown implements the logic for tearing down a service.
	TearDown(ctx context.Context) error

	// Signal sends a signal to the service.
	Signal(ctx context.Context, signal string) error

	// Info returns the current context from the service.
	Info() AgentInfo

	// SetInfo sets the current context for the service.
	SetInfo(str AgentInfo) error

	// ExitCode returns true if the service is exited and its exit code.
	ExitCode(ctx context.Context, service string) (bool, int, error)
}
