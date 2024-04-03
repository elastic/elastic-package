// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package agentdeployer

import (
	"context"
	"errors"
	"time"
)

var ErrNotSupported error = errors.New("not supported")

// DeployedAgent defines the interface for interacting with an agent that has been deployed.
type DeployedAgent interface {
	// TearDown implements the logic for tearing down an agent.
	TearDown(ctx context.Context) error

	// Signal sends a signal to the agent.
	Signal(ctx context.Context, signal string) error

	// Info returns the current information from the agent.
	Info() AgentInfo

	// SetInfo sets the current information about the agent.
	SetInfo(AgentInfo)

	// ExitCode returns true if the service is exited and its exit code.
	ExitCode(ctx context.Context, service string) (bool, int, error)

	// Logs returns the logs from the agent starting at the given time
	Logs(ctx context.Context, t time.Time) ([]byte, error)
}
