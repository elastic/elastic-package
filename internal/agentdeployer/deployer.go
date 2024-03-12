// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package agentdeployer

// AgentDeployer defines the interface for deploying an agent. It defines methods for
// controlling the lifecycle of an agent.
type AgentDeployer interface {
	// SetUp implements the logic for setting up an agent. It takes a context and returns a
	// AgentHandler.
	SetUp(ctxt AgentInfo) (DeployedAgent, error)
}
