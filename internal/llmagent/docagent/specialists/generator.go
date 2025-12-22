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

## Getting Input
1. First, use the read_state tool to read "section_context" - this contains:
   - SectionTitle: The title of the section to generate
   - SectionLevel: The heading level (2 = ##, 3 = ###, etc.)
   - TemplateContent: Template text showing the expected structure
   - ExampleContent: Example content for style reference
   - ExistingContent: Current content to improve upon (if any)
   - PackageName: The package identifier
   - PackageTitle: The human-readable package name

2. Also use read_state to check for "feedback" - if present, it contains feedback from the critic agent that you should address.

## Storing Output
After generating the content, use the write_state tool to store:
- key: "section_content"
- value: The complete generated markdown content

## Content Generation Rules
1. Start with a heading at the correct level (## for level 2, ### for level 3, etc.)
2. If ExistingContent is provided, use it as the base and improve upon it
3. Otherwise, if TemplateContent is provided, follow its structure
4. Otherwise, if ExampleContent is provided, use it as a style reference
5. If there's feedback from the critic, address the specific concerns

## Guidelines
- Write clear, concise, and accurate documentation
- Follow the Elastic documentation style (friendly, direct, use "you")
- Include relevant code examples and configuration snippets where appropriate
- Use proper markdown formatting

## IMPORTANT
You MUST use the read_state and write_state tools. Do not just output text directly.
`

// Build creates the underlying ADK agent.
func (g *GeneratorAgent) Build(ctx context.Context, cfg AgentConfig) (agent.Agent, error) {
	// Combine state tools with provided tools
	allTools := append(StateTools(), cfg.Tools...)
	return llmagent.New(llmagent.Config{
		Name:        generatorAgentName,
		Description: generatorAgentDescription,
		Model:       cfg.Model,
		Instruction: generatorInstruction,
		Tools:       allTools,
		Toolsets:    cfg.Toolsets,
	})
}
