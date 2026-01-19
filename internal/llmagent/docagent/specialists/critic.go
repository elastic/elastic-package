// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package specialists

import (
	"context"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/genai"

	"github.com/elastic/elastic-package/internal/llmagent/docagent/specialists/validators"
	"github.com/elastic/elastic-package/internal/llmagent/docagent/stylerules"
)

const (
	criticAgentName        = "critic"
	criticAgentDescription = "Reviews generated documentation for style, voice, tone, and accessibility"
)

// CriticAgent reviews documentation content and provides feedback.
type CriticAgent struct{}

// NewCriticAgent creates a new critic agent.
func NewCriticAgent() *CriticAgent {
	return &CriticAgent{}
}

// Name returns the agent name.
func (c *CriticAgent) Name() string {
	return criticAgentName
}

// Description returns the agent description.
func (c *CriticAgent) Description() string {
	return criticAgentDescription
}

// criticInstructionPrefix is the first part of the critic system instruction
const criticInstructionPrefix = `You are a documentation style critic for Elastic integration packages.
Your task is to review documentation content for voice, tone, accessibility, and readability.

## Input
The documentation content to review is provided directly in the user message.

## Evaluation Criteria
Evaluate against these criteria (rate each 1-10):

1. **Voice/Tone**: Friendly, uses "you", contractions, active voice
   - Good: "You can configure..." "Before you start, you'll need..."
   - Bad: "The user must configure..." "It is recommended that..."

2. **Accessibility**: Descriptive links, plain language, no directional terms
   - Good: "See the [installation guide](...)" 
   - Bad: "Click [here](...)" or "See the documentation above"

3. **Grammar**: American English, present tense, Oxford comma, sentence case headings
   - Sentence case: "### General debugging steps" NOT "### General Debugging Steps"

4. **Structure**: Clear summary, scannable sections, short paragraphs

`

// criticInstructionSuffix is the final part of the critic system instruction
const criticInstructionSuffix = `
## Output Format
Output a JSON object with this exact structure:
{"approved": true/false, "score": 1-10, "feedback": "specific feedback if not approved"}

Set approved=true if average score >= 7 with no critical issues.
If not approved, provide specific, actionable feedback.

## IMPORTANT
Output ONLY the JSON object. No other text.`

// CriticResult represents the result of a critic review
type CriticResult struct {
	Score    int    `json:"score"`
	Approved bool   `json:"approved"`
	Feedback string `json:"feedback"`
}

// Build creates the underlying ADK agent.
func (c *CriticAgent) Build(ctx context.Context, cfg validators.AgentConfig) (agent.Agent, error) {
	// Build the full instruction by combining prefix, shared rules, and suffix
	instruction := criticInstructionPrefix + stylerules.FullFormattingRules + "\n" + stylerules.CriticRejectionCriteria + criticInstructionSuffix

	// JSON response mode is incompatible with function calling on some models
	// (e.g., gemini-2.5-pro). Disable auto-flow features that add transfer tools.
	return llmagent.New(llmagent.Config{
		Name:                     criticAgentName,
		Description:              criticAgentDescription,
		Model:                    cfg.Model,
		Instruction:              instruction,
		DisallowTransferToParent: true,
		DisallowTransferToPeers:  true,
		GenerateContentConfig: &genai.GenerateContentConfig{
			ResponseMIMEType: "application/json",
		},
	})
}
