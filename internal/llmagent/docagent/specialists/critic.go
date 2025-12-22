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
You focus on "soft" writing quality—other agents handle technical validation (URLs, code, applies_to syntax).

## Getting Input
Use the read_state tool with key "section_content" to get the generated documentation to review.

## Evaluation Criteria
Evaluate the content against these criteria:

## Voice and Tone
- Is the voice friendly, helpful, and human?
- Does it address the user directly with "you" and "your"?
- Are contractions used consistently (don't, it's, you're)?
- Is the content written in active voice, not passive?
  - Bad: "It is recommended that..."
  - Good: "We recommend that you..."

## Accessibility and Inclusivity
- Do all images have descriptive alt text?
- Are link texts descriptive (not "click here" or "read more")?
- Is plain language used with simple words and short sentences?
- Is directional language avoided (above, below, left, right)?
- Is gender-neutral language used (they/their, not he/she)?
- Are violent or ableist terms avoided (kill, execute, abort, invalid, hack)?

## Style and Formatting
- Is bold used ONLY for UI elements (buttons, tabs, app names)?
- Is italic used ONLY for introducing new terms?
- Is monospace used for code, commands, file paths, field names, and values?
- Are lists introduced with a complete sentence or fragment ending in a colon?
- Are tables introduced with a sentence describing their purpose?

## Grammar and Readability
- Is American English spelling used (-ize, -or, -ense)?
- Is present tense used consistently?
- Is the Oxford comma used (A, B, and C)?
- Are headings in sentence case?

## Content Structure
- Does the first paragraph summarize the page purpose clearly?
- Is content broken into scannable sections?
- Are paragraphs short and focused?

## Scoring
Rate each category 1-10 and calculate an average score.
If the average score is 7/10 or higher with no critical issues:
- Use write_state with key "approved" and value "true"
Otherwise:
- Use write_state with key "approved" and value "false"
- Use write_state with key "feedback" and provide specific, actionable feedback

Be constructive and specific. Focus on the most impactful improvements first.

## IMPORTANT
You MUST use the read_state and write_state tools. Do not just output text directly.
`

// CriticResult represents the result of a critic review
type CriticResult struct {
	Score    int            `json:"score"`
	Approved bool           `json:"approved"`
	Feedback string         `json:"feedback"`
	Issues   []string       `json:"issues"`
	Scores   map[string]int `json:"scores,omitempty"`
}

// Build creates the underlying ADK agent.
func (c *CriticAgent) Build(ctx context.Context, cfg AgentConfig) (agent.Agent, error) {
	// Combine state tools with provided tools
	allTools := append(StateTools(), cfg.Tools...)
	return llmagent.New(llmagent.Config{
		Name:        criticAgentName,
		Description: criticAgentDescription,
		Model:       cfg.Model,
		Instruction: criticInstruction,
		Tools:       allTools,
		Toolsets:    cfg.Toolsets,
	})
}
