// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package specialists

import (
	"context"
	"iter"
	"strings"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/session"
	"google.golang.org/genai"

	"github.com/elastic/elastic-package/internal/logger"
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

Read the generated content from temp:section_content and verify:
1. Code snippets are syntactically correct
2. Configuration examples are valid
3. References to fields, settings, and features are accurate
4. Versions and compatibility information are correct
5. Technical terminology is used correctly

Store your validation results in temp:validation_result as a JSON object:
{
  "valid": true/false,
  "issues": ["list of issues found"],
  "warnings": ["list of warnings"]
}

Be thorough but avoid false positives. Only flag genuine issues.
`

// ValidationResult represents the result of validation
type ValidationResult struct {
	Valid    bool     `json:"valid"`
	Issues   []string `json:"issues"`
	Warnings []string `json:"warnings"`
}

// Build creates the underlying ADK agent.
func (v *ValidatorAgent) Build(ctx context.Context, cfg AgentConfig) (agent.Agent, error) {
	// If we have an LLM model, create an LLM-based agent
	if cfg.Model != nil {
		return llmagent.New(llmagent.Config{
			Name:        validatorAgentName,
			Description: validatorAgentDescription,
			Model:       cfg.Model,
			Instruction: validatorInstruction,
		})
	}

	// Otherwise create a simple pass-through agent (stub mode)
	return agent.New(agent.Config{
		Name:        validatorAgentName,
		Description: validatorAgentDescription,
		Run:         v.run,
	})
}

// run implements the agent logic when no LLM is available (stub mode)
func (v *ValidatorAgent) run(invCtx agent.InvocationContext) iter.Seq2[*session.Event, error] {
	return func(yield func(*session.Event, error) bool) {
		state := invCtx.Session().State()

		// Read content from state
		content, err := state.Get(StateKeyContent)
		if err != nil {
			logger.Debugf("Validator: no content found in state")
			event := session.NewEvent(invCtx.InvocationID())
			event.Content = genai.NewContentFromText("No content to validate", genai.RoleModel)
			event.Author = validatorAgentName
			event.Actions.StateDelta = map[string]any{
				StateKeyValidation: ValidationResult{
					Valid:  false,
					Issues: []string{"No content provided for validation"},
				},
			}
			yield(event, nil)
			return
		}

		contentStr, _ := content.(string)

		// Stub implementation: perform basic validation checks
		result := v.validateContent(contentStr)

		// Create event with state update
		event := session.NewEvent(invCtx.InvocationID())
		event.Content = genai.NewContentFromText("Validation complete", genai.RoleModel)
		event.Author = validatorAgentName
		event.Actions.StateDelta = map[string]any{
			StateKeyValidation: result,
		}

		yield(event, nil)
	}
}

// validateContent performs basic content validation (stub implementation)
func (v *ValidatorAgent) validateContent(content string) ValidationResult {
	var issues []string
	var warnings []string

	// Check for common issues (stub checks)
	if strings.Contains(content, "TODO") || strings.Contains(content, "FIXME") {
		warnings = append(warnings, "Content contains TODO/FIXME markers")
	}

	if strings.Contains(content, "<<") && strings.Contains(content, ">>") {
		issues = append(issues, "Content contains placeholder markers (<< >>)")
	}

	// Check for empty code blocks
	if strings.Contains(content, "```\n```") || strings.Contains(content, "```\n\n```") {
		issues = append(issues, "Content contains empty code blocks")
	}

	return ValidationResult{
		Valid:    len(issues) == 0,
		Issues:   issues,
		Warnings: warnings,
	}
}

