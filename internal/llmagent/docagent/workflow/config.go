// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

// Package workflow provides multi-agent workflow orchestration for documentation generation.
package workflow

import (
	"google.golang.org/adk/model"
	"google.golang.org/adk/tool"

	"github.com/elastic/elastic-package/internal/llmagent/docagent/agents"
)

// DefaultMaxIterations is the default maximum number of workflow iterations
const DefaultMaxIterations uint = 3

// Config holds configuration for building workflows
type Config struct {
	// Registry contains the agents to use in the workflow
	Registry *agents.Registry

	// MaxIterations limits the number of refinement cycles
	// Set to 0 for unlimited iterations (until approved)
	MaxIterations uint

	// Model is the LLM model to use for agents
	Model model.LLM

	// Tools available to agents in the workflow
	Tools []tool.Tool

	// Toolsets available to agents in the workflow
	Toolsets []tool.Toolset

	// EnableCritic enables the critic agent in the workflow
	EnableCritic bool

	// EnableValidator enables the validator agent in the workflow
	EnableValidator bool

	// EnableURLValidator enables the URL validator agent in the workflow
	EnableURLValidator bool
}

// DefaultConfig returns a Config with sensible defaults
func DefaultConfig() Config {
	return Config{
		Registry:           agents.DefaultRegistry(),
		MaxIterations:      DefaultMaxIterations,
		EnableCritic:       true,
		EnableValidator:    true,
		EnableURLValidator: true,
	}
}

// WithModel sets the LLM model
func (c Config) WithModel(m model.LLM) Config {
	c.Model = m
	return c
}

// WithTools sets the tools available to agents
func (c Config) WithTools(tools []tool.Tool) Config {
	c.Tools = tools
	return c
}

// WithToolsets sets the toolsets available to agents
func (c Config) WithToolsets(toolsets []tool.Toolset) Config {
	c.Toolsets = toolsets
	return c
}

// WithMaxIterations sets the maximum number of iterations
func (c Config) WithMaxIterations(n uint) Config {
	c.MaxIterations = n
	return c
}

// WithRegistry sets a custom agent registry
func (c Config) WithRegistry(r *agents.Registry) Config {
	c.Registry = r
	return c
}

// GeneratorOnly returns a config that only uses the generator agent
func GeneratorOnly() Config {
	r := agents.NewRegistry()
	r.Register(agents.NewGeneratorAgent())
	return Config{
		Registry:      r,
		MaxIterations: 1,
	}
}
