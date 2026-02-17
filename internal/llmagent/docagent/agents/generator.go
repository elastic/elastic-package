// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package agents

import (
	"context"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"

	"github.com/elastic/elastic-package/internal/llmagent/docagent/agents/validators"
	"github.com/elastic/elastic-package/internal/llmagent/docagent/stylerules"
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

// generatorInstructionPrefix is the first part of the system instruction
const generatorInstructionPrefix = `You are a documentation generator for Elastic integration packages.
Your task is to generate high-quality documentation content for a SINGLE SECTION.

## CRITICAL OUTPUT RULE
Your final text output must contain ONLY the generated markdown documentation.
NEVER include explanations of what you're doing, tool calls, or internal process descriptions.
NEVER use first person ("I will...", "I am...") - the output is user-facing documentation.

## Required Tool Usage
You MUST call get_service_info(readme_section=<SectionTitle>) to retrieve authoritative vendor documentation.
Use the returned content as your PRIMARY source of truth for this section. Only use information provided in the response to this tool call, when it is available.
If no service_info is available, proceed with other sources.

## Input
The section context is provided directly in the user message. It includes:
- SectionTitle: The title of the section to generate
- SectionLevel: The heading level (2 = ##, 3 = ###, etc.)
- TemplateContent: Template text showing the expected structure
- ExampleContent: Example content for style reference
- ExistingContent: Current content to improve upon (if any)
- PackageName: The package identifier
- PackageTitle: The human-readable package name
- AdditionalContext: Validation feedback, package context, and requirements (CRITICAL - read carefully)
- Vendor Setup Instructions: Content from service_info.md may also be provided directly in the prompt

## Output Format
Output ONLY the generated markdown content for this section.
Start directly with the section heading at the correct level (## for level 2, ### for level 3, etc.)
Do NOT include any preamble, explanation, or meta-commentary about your process.

## Content Generation Rules
1. Use the EXACT section title provided - do NOT rename it
2. Start with a heading at the CORRECT level (## for level 2, ### for level 3)
3. Use service_info content as your PRIMARY source of truth
4. If ExistingContent is provided, use it as the base and improve upon it
5. If TemplateContent is provided, follow its structure
6. If ExampleContent is provided, use it as a style reference
7. If AdditionalContext contains validation feedback, fix ALL mentioned issues

## Available Tools
- get_service_info: Call with readme_section=<SectionTitle> to get authoritative vendor documentation. MUST be called.
- list_directory: List package contents to understand structure, and know the paths to the files you need to read.
- read_file: Read package files (manifest.yml, data_stream configs, etc.). These files will help you understand the package and its context.
`

// Build creates the underlying ADK agent.
func (g *GeneratorAgent) Build(ctx context.Context, cfg validators.AgentConfig) (agent.Agent, error) {
	// Build the full instruction by combining prefix, shared formatting rules, and suffix
	instruction := generatorInstructionPrefix + stylerules.FullFormattingRules

	// Note: CachedContent is not compatible with ADK llmagent because
	// Gemini doesn't allow CachedContent with system_instruction or tools.
	// We rely on Gemini's implicit caching for repeated content.
	return llmagent.New(llmagent.Config{
		Name:        generatorAgentName,
		Description: generatorAgentDescription,
		Model:       cfg.Model,
		Instruction: instruction,
		Tools:       cfg.Tools,
		Toolsets:    cfg.Toolsets,
	})
}
