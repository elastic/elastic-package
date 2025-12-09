// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package specialists

import (
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"strings"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/session"
	"google.golang.org/genai"

	"github.com/elastic/elastic-package/internal/logger"
)

const (
	generatorAgentName        = "generator"
	generatorAgentDescription = "Generates documentation content for a section based on package context and templates"
)

// GeneratorAgent generates documentation content for sections.
type GeneratorAgent struct{}

// NewGeneratorAgent creates a new generator agent.
func NewGeneratorAgent() *GeneratorAgent {
	return &GeneratorAgent{}
}

// Name returns the agent name.
func (g *GeneratorAgent) Name() string {
	return generatorAgentName
}

// Description returns the agent description.
func (g *GeneratorAgent) Description() string {
	return generatorAgentDescription
}

// generatorInstruction is the system instruction for the generator agent
const generatorInstruction = `You are a documentation generator for Elastic integration packages.
Your task is to generate high-quality documentation content for a specific section.

Read the section context from the session state (temp:section_context) and any feedback
from previous iterations (temp:feedback).

Generate the documentation content and store it in temp:section_content.

Guidelines:
- Follow the template structure provided in the section context
- Use the example content as a reference for style and tone
- If there's existing content, improve upon it while maintaining accuracy
- If there's feedback from the critic, address the specific concerns
- Write clear, concise, and accurate documentation
- Include relevant code examples and configuration snippets where appropriate
`

// Build creates the underlying ADK agent.
func (g *GeneratorAgent) Build(ctx context.Context, cfg AgentConfig) (agent.Agent, error) {
	// If we have an LLM model, create an LLM-based agent
	if cfg.Model != nil {
		return llmagent.New(llmagent.Config{
			Name:        generatorAgentName,
			Description: generatorAgentDescription,
			Model:       cfg.Model,
			Instruction: generatorInstruction,
			Tools:       cfg.Tools,
			Toolsets:    cfg.Toolsets,
		})
	}

	// Otherwise create a simple pass-through agent for testing
	return agent.New(agent.Config{
		Name:        generatorAgentName,
		Description: generatorAgentDescription,
		Run:         g.run,
	})
}

// run implements the agent logic when no LLM is available (stub mode)
func (g *GeneratorAgent) run(invCtx agent.InvocationContext) iter.Seq2[*session.Event, error] {
	return func(yield func(*session.Event, error) bool) {
		state := invCtx.Session().State()

		// Read section context from state
		var sectionCtx SectionContext
		if ctxData, err := state.Get(StateKeySectionContext); err == nil {
			if ctxJSON, ok := ctxData.(string); ok {
				if err := json.Unmarshal([]byte(ctxJSON), &sectionCtx); err != nil {
					logger.Debugf("Generator: failed to parse section context: %v", err)
				}
			}
		}

		// Check for feedback from previous iterations
		var feedback string
		if fb, err := state.Get(StateKeyFeedback); err == nil {
			feedback, _ = fb.(string)
		}

		// Generate content (stub implementation - in real use, LLM would do this)
		content := g.generateStubContent(sectionCtx, feedback)

		// Create event with state update
		event := session.NewEvent(invCtx.InvocationID())
		event.Content = genai.NewContentFromText(content, genai.RoleModel)
		event.Author = generatorAgentName
		event.Actions.StateDelta = map[string]any{
			StateKeyContent: content,
		}

		yield(event, nil)
	}
}

// generateStubContent creates placeholder content for testing
func (g *GeneratorAgent) generateStubContent(ctx SectionContext, feedback string) string {
	var sb strings.Builder

	// Create header based on level
	headerPrefix := strings.Repeat("#", ctx.SectionLevel)
	if headerPrefix == "" {
		headerPrefix = "##"
	}

	sb.WriteString(fmt.Sprintf("%s %s\n\n", headerPrefix, ctx.SectionTitle))

	if ctx.ExistingContent != "" {
		// Use existing content as base
		sb.WriteString(ctx.ExistingContent)
	} else if ctx.TemplateContent != "" {
		// Use template content
		sb.WriteString(ctx.TemplateContent)
	} else {
		sb.WriteString(fmt.Sprintf("Documentation for %s section.\n", ctx.SectionTitle))
	}

	if feedback != "" {
		sb.WriteString(fmt.Sprintf("\n\n<!-- Feedback addressed: %s -->\n", feedback))
	}

	return sb.String()
}
