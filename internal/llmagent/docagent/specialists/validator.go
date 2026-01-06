// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package specialists

import (
	"context"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/genai"
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

## Input
The documentation content to validate is provided directly in the user message.

## Validation Checks
Check for these issues (mark as invalid if found):
1. Empty code blocks (triple backticks with no content)
2. Syntactically incorrect code snippets (malformed YAML, JSON, etc.)
3. Incorrect references to fields, settings, or features

Check for these warnings (valid but note them):
1. TODO or FIXME markers
2. Code blocks without language specification

## Output Format
Output a JSON object with this exact structure:
{"valid": true/false, "issues": ["issue1", "issue2"], "warnings": ["warning1"]}

Set valid=false if ANY issues are found. Warnings alone do not invalidate.
Be thorough but avoid false positives. Only flag genuine issues.

## IMPORTANT
Output ONLY the JSON object. No other text.`

// ValidationResult represents the result of validation
type ValidationResult struct {
	Valid    bool     `json:"valid"`
	Issues   []string `json:"issues"`
	Warnings []string `json:"warnings"`
}

// Build creates the underlying ADK agent.
func (v *ValidatorAgent) Build(ctx context.Context, cfg AgentConfig) (agent.Agent, error) {
	// JSON response mode is incompatible with function calling on some models
	// (e.g., gemini-2.5-pro). Disable auto-flow features that add transfer tools.
	return llmagent.New(llmagent.Config{
		Name:                     validatorAgentName,
		Description:              validatorAgentDescription,
		Model:                    cfg.Model,
		Instruction:              validatorInstruction,
		DisallowTransferToParent: true,
		DisallowTransferToPeers:  true,
		GenerateContentConfig: &genai.GenerateContentConfig{
			ResponseMIMEType: "application/json",
		},
	})
}
