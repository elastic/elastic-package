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
2. **Accessibility**: Descriptive links, plain language, no directional terms
3. **Style**: Bold for UI only, monospace for code, proper list introductions
4. **Grammar**: American English, present tense, Oxford comma, sentence case headings
5. **Structure**: Clear summary, scannable sections, short paragraphs

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
func (c *CriticAgent) Build(ctx context.Context, cfg AgentConfig) (agent.Agent, error) {
	// Note: CachedContent is not compatible with ADK llmagent because
	// Gemini doesn't allow CachedContent with system_instruction or tools.
	return llmagent.New(llmagent.Config{
		Name:        criticAgentName,
		Description: criticAgentDescription,
		Model:       cfg.Model,
		Instruction: criticInstruction,
		Tools:       cfg.Tools,
		Toolsets:    cfg.Toolsets,
		GenerateContentConfig: &genai.GenerateContentConfig{
			ResponseMIMEType: "application/json",
		},
	})
}
