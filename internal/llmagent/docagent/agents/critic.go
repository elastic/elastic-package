// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package agents

import (
	"context"
	"fmt"
	"iter"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/session"
	"google.golang.org/genai"

	"github.com/elastic/elastic-package/internal/logger"
)

const (
	criticAgentName        = "critic"
	criticAgentDescription = "Reviews generated documentation and provides feedback for improvement"
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
const criticInstruction = `You are a documentation critic for Elastic integration packages.
Your task is to review documentation content and provide constructive feedback.

Read the generated content from temp:section_content and evaluate it against these criteria:
1. Accuracy - Is the technical information correct?
2. Completeness - Does it cover all necessary aspects?
3. Clarity - Is it easy to understand?
4. Style - Does it follow Elastic documentation guidelines?
5. Structure - Is it well-organized with proper headings?

If the content meets all criteria with a score of 8/10 or higher, set temp:approved to true.
Otherwise, provide specific feedback in temp:feedback for the generator to address.

Be constructive and specific in your feedback. Focus on actionable improvements.
`

// Build creates the underlying ADK agent.
func (c *CriticAgent) Build(ctx context.Context, cfg AgentConfig) (agent.Agent, error) {
	// If we have an LLM model, create an LLM-based agent
	if cfg.Model != nil {
		return llmagent.New(llmagent.Config{
			Name:        criticAgentName,
			Description: criticAgentDescription,
			Model:       cfg.Model,
			Instruction: criticInstruction,
		})
	}

	// Otherwise create a simple pass-through agent (stub mode)
	return agent.New(agent.Config{
		Name:        criticAgentName,
		Description: criticAgentDescription,
		Run:         c.run,
	})
}

// run implements the agent logic when no LLM is available (stub mode)
func (c *CriticAgent) run(invCtx agent.InvocationContext) iter.Seq2[*session.Event, error] {
	return func(yield func(*session.Event, error) bool) {
		state := invCtx.Session().State()

		// Read content from state
		content, err := state.Get(StateKeyContent)
		if err != nil {
			logger.Debugf("Critic: no content found in state")
			event := session.NewEvent(invCtx.InvocationID())
			event.Content = genai.NewContentFromText("No content to review", genai.RoleModel)
			event.Author = criticAgentName
			event.Actions.StateDelta = map[string]any{
				StateKeyFeedback: "No content was provided for review",
				StateKeyApproved: false,
			}
			yield(event, nil)
			return
		}

		contentStr, _ := content.(string)

		// Stub implementation: auto-approve if content has reasonable length
		// In production, the LLM would perform actual review
		approved := len(contentStr) > 100
		var feedback string

		if !approved {
			feedback = "Content is too short. Please provide more detailed documentation."
		} else {
			feedback = "Content meets basic quality standards."
		}

		// Create event with state update
		event := session.NewEvent(invCtx.InvocationID())
		event.Content = genai.NewContentFromText(fmt.Sprintf("Review complete. Approved: %v", approved), genai.RoleModel)
		event.Author = criticAgentName
		event.Actions.StateDelta = map[string]any{
			StateKeyFeedback: feedback,
			StateKeyApproved: approved,
		}

		yield(event, nil)
	}
}
