// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package specialists

import (
	"context"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
)

const (
	validatorAgentName        = "validator"
	validatorAgentDescription = "Validates documentation content for correctness and consistency"
)

// ValidatorAgent validates documentation content for technical correctness.
type ValidatorAgent struct{}

// NewValidatorAgent creates a new validator agent.
func NewValidatorAgent() *ValidatorAgent {
	return &ValidatorAgent{}
}

// Name returns the agent name.
func (v *ValidatorAgent) Name() string {
	return validatorAgentName
}

// Description returns the agent description.
func (v *ValidatorAgent) Description() string {
	return validatorAgentDescription
}

// validatorInstruction is the system instruction for the validator agent
const validatorInstruction = `You are a documentation validator for Elastic integration packages.
Your task is to validate the technical correctness of documentation content.

## Getting Input
Use the read_state tool with key "section_content" to get the generated documentation to validate.

## Validation Checks
Check the content for:

## Issues (mark as invalid)
1. Placeholder markers like << >> or {{ }} that weren't replaced
2. Empty code blocks (triple backticks with no content)
3. Syntactically incorrect code snippets
4. Invalid configuration examples (malformed YAML, JSON, etc.)
5. Incorrect references to fields, settings, or features
6. Factually incorrect version or compatibility information

## Warnings (valid but should be addressed)
1. TODO or FIXME markers in the content
2. Code snippets without language specification
3. Potentially outdated technical information
4. Missing error handling in code examples

## Storing Output
Use the write_state tool to store your validation results:
- key: "validation_result"
- value: A JSON object like: {"valid": true/false, "issues": [...], "warnings": [...]}

If validation fails (issues found):
- Use write_state with key "approved" and value "false"
- Use write_state with key "feedback" with the issues that need to be fixed

Set "valid" to false if ANY issues are found. Warnings alone do not invalidate content.
Be thorough but avoid false positives. Only flag genuine issues.

## IMPORTANT
You MUST use the read_state and write_state tools. Do not just output text directly.
`

// ValidationResult represents the result of validation
type ValidationResult struct {
	Valid    bool     `json:"valid"`
	Issues   []string `json:"issues"`
	Warnings []string `json:"warnings"`
}

// Build creates the underlying ADK agent.
func (v *ValidatorAgent) Build(ctx context.Context, cfg AgentConfig) (agent.Agent, error) {
	// Combine state tools with provided tools
	allTools := append(StateTools(), cfg.Tools...)
	return llmagent.New(llmagent.Config{
		Name:        validatorAgentName,
		Description: validatorAgentDescription,
		Model:       cfg.Model,
		Instruction: validatorInstruction,
		Tools:       allTools,
		Toolsets:    cfg.Toolsets,
	})
}
