// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package providers

import (
	"context"
)

// LLMProvider defines the interface for different LLM providers
type LLMProvider interface {
	// GenerateResponse sends a prompt to the LLM and returns the response
	GenerateResponse(ctx context.Context, prompt string, tools []Tool) (*LLMResponse, error)

	// Name returns the name of the provider
	Name() string
}

// LLMResponse represents the response from an LLM
type LLMResponse struct {
	// Content is the text response from the LLM
	Content string

	// ToolCalls are the tool calls the LLM wants to make
	ToolCalls []ToolCall

	// Finished indicates if the LLM considers the conversation complete
	Finished bool
}

// ToolCall represents a tool call request from the LLM
type ToolCall struct {
	// ID is a unique identifier for this tool call
	ID string

	// Name is the name of the tool to call
	Name string

	// Arguments are the arguments to pass to the tool (JSON string)
	Arguments string
}

// Tool represents a tool that can be called by the LLM
type Tool struct {
	// Name is the name of the tool
	Name string

	// Description describes what the tool does
	Description string

	// Parameters defines the JSON schema for the tool parameters
	Parameters map[string]interface{}

	// Handler is the function that executes the tool
	Handler ToolHandler
}

// ToolHandler is a function that executes a tool
type ToolHandler func(ctx context.Context, arguments string) (*ToolResult, error)

// ToolResult represents the result of a tool execution
type ToolResult struct {
	// Content is the result content
	Content string

	// Error indicates if there was an error
	Error string
}

// Compile-time interface checks to ensure all provider types implement the LLMProvider interface
var (
	_ LLMProvider = &GeminiProvider{}
	_ LLMProvider = &LocalProvider{}
)
