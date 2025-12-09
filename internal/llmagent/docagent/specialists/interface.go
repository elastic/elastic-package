// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

// Package specialists provides specialized documentation agents for multi-agent workflows.
package specialists

import (
	"context"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/model"
	"google.golang.org/adk/tool"
)

// Session state keys for inter-agent communication.
// Using "temp:" prefix ensures keys are scoped to the current invocation.
const (
	// StateKeyContent holds the generated section content
	StateKeyContent = "temp:section_content"
	// StateKeyFeedback holds critic feedback for the generator
	StateKeyFeedback = "temp:feedback"
	// StateKeyValidation holds validation results
	StateKeyValidation = "temp:validation_result"
	// StateKeyURLCheck holds URL validation results
	StateKeyURLCheck = "temp:url_check_result"
	// StateKeyApproved signals workflow completion
	StateKeyApproved = "temp:approved"
	// StateKeyIteration tracks current iteration number
	StateKeyIteration = "temp:iteration"
	// StateKeySectionContext holds the input section context
	StateKeySectionContext = "temp:section_context"
)

// SectionContext holds the context needed for section generation
type SectionContext struct {
	SectionTitle    string
	SectionLevel    int
	TemplateContent string
	ExampleContent  string
	ExistingContent string
	PackageName     string
	PackageTitle    string
}

// AgentConfig holds common configuration for building agents
type AgentConfig struct {
	// Model is the LLM model to use
	Model model.LLM
	// Tools available to the agent
	Tools []tool.Tool
	// Toolsets available to the agent
	Toolsets []tool.Toolset
}

// SectionAgent defines the interface for documentation workflow agents.
// Implement this interface to create new specialized agents that can
// participate in the documentation generation workflow.
type SectionAgent interface {
	// Name returns the unique agent name
	Name() string
	// Description returns a brief description of what this agent does
	Description() string
	// Build creates the underlying ADK agent with the given configuration
	Build(ctx context.Context, cfg AgentConfig) (agent.Agent, error)
}

// AgentResult represents the result of an agent's execution
type AgentResult struct {
	// Content is the output content (if any)
	Content string
	// Approved indicates if the workflow should complete
	Approved bool
	// Feedback contains any feedback for other agents
	Feedback string
	// Error contains any error message
	Error string
}
