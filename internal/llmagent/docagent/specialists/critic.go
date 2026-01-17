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

// criticInstruction is the system instruction for the critic agent
const criticInstruction = `You are a documentation style critic for Elastic integration packages.
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

3. **Style**: Bold for UI elements ONLY, monospace for code, proper list introductions
   - Bold is ONLY for UI elements: **Settings** > **Logging**, **Save** button
   - Bold is NOT for: list item headings, conceptual terms, emphasis, notes
   - Bad: "**No data collected**:" or "**Fault Tolerance**:" or "**Note**:"
   - Good: "No data collected:" or "Fault tolerance:" (plain text)
   - Lists must have an introductory sentence before them

4. **Grammar**: American English, present tense, Oxford comma, sentence case headings
   - Sentence case: "### General debugging steps" NOT "### General Debugging Steps"

5. **Structure**: Clear summary, scannable sections, short paragraphs

## Common Issues to Flag (these MUST cause rejection)

1. Bold for list item headings - ALWAYS reject if found:
   WRONG: This integration facilitates:
   - **Security monitoring**: Ingests audit logs...
   - **Operational visibility**: Collects logs...
   
   RIGHT: This integration facilitates:
   - Security monitoring: Ingests audit logs...
   - Operational visibility: Collects logs...

2. Other wrong bold patterns:
   - "**No data is being collected**:" → should be "No data is being collected:"
   - "**Audit device is not enabled**:" → should be "Audit device is not enabled:"
   - "**Fault Tolerance**:" → should be "Fault tolerance:"
   - "**Note**:" or "**Important**:" → should be plain text

3. Bold+monospace together: "**` + "`audit`" + `**" → should be just "` + "`audit`" + `"

4. Missing list introductions (lists need an intro sentence ending with colon)

5. Passive/formal voice instead of direct "you" address

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
	// JSON response mode is incompatible with function calling on some models
	// (e.g., gemini-2.5-pro). Disable auto-flow features that add transfer tools.
	return llmagent.New(llmagent.Config{
		Name:                     criticAgentName,
		Description:              criticAgentDescription,
		Model:                    cfg.Model,
		Instruction:              criticInstruction,
		DisallowTransferToParent: true,
		DisallowTransferToPeers:  true,
		GenerateContentConfig: &genai.GenerateContentConfig{
			ResponseMIMEType: "application/json",
		},
	})
}
