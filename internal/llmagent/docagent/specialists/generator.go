// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package specialists

import (
	"context"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"

	"github.com/elastic/elastic-package/internal/llmagent/docagent/specialists/validators"
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
Your task is to generate high-quality, complete README documentation.

## REQUIRED DOCUMENT STRUCTURE
You MUST use these EXACT section names in this order:

# {Package Title}

> **Note**: This documentation was generated using AI and should be reviewed for accuracy.

## Overview
### Compatibility
### How it works

## What data does this integration collect?
### Supported use cases

## What do I need to use this integration?

## How do I deploy this integration?
### Agent-based deployment
### Onboard and configure
### Validation

## Troubleshooting

## Performance and scaling

## Reference
### Inputs used
### API usage (if using APIs)

## Input
The section context is provided directly in the user message. It includes:
- PackageName: The package identifier
- PackageTitle: The human-readable package name
- ExistingContent: Current content to improve upon (if any)
- AdditionalContext: Validation feedback and requirements (CRITICAL - read carefully)
- Advanced Settings: Configuration variables with important caveats that MUST be documented

## Output
Output ONLY the complete markdown document. Do not include any explanation or commentary.

## Content Generation Rules
1. Use the EXACT section names shown above - do NOT rename them
2. Start with # {Package Title} as the H1 heading
3. IMMEDIATELY after the H1 title, include the AI-generated disclosure note: "> **Note**: This documentation was generated using AI and should be reviewed for accuracy."
4. If AdditionalContext contains validation feedback, fix ALL mentioned issues
5. If AdditionalContext contains vendor documentation links, include ALL of them in appropriate sections
6. Include all data streams from the package
7. Ensure heading hierarchy: # for title, ## for main sections, ### for subsections

## Advanced Settings Documentation
When the context includes Advanced Settings, you MUST document them properly:
1. **Security Warnings**: Include clear warnings for settings that compromise security or expose sensitive data
   - Example: "⚠️ **Warning**: Enabling request tracing compromises security and should only be used for debugging."
2. **Debug/Development Settings**: Warn that these should NOT be enabled in production
   - Document in the Troubleshooting section or a dedicated Advanced Settings subsection
3. **SSL/TLS Configuration**: Document certificate setup and configuration options
   - Include example YAML snippets showing how to configure certificates
4. **Sensitive Fields**: Mention secure credential handling
   - Reference Fleet's secret management or environment variables
5. **Complex Configurations**: Provide YAML/JSON examples for complex settings

## Guidelines
- Write clear, concise, and accurate documentation
- Follow the Elastic documentation style (friendly, direct, use "you")
- Include relevant code examples and configuration snippets where appropriate
- Use proper markdown formatting
- If using {{ }} template variables like {{event "datastream"}} or {{fields "datastream"}}, preserve them
- For code blocks, ALWAYS specify the language (e.g., bash, yaml, json after the triple backticks)

## CRITICAL
- Do NOT rename sections (e.g., don't use "## Setup" instead of "## How do I deploy this integration?")
- Do NOT skip required sections
- When including URLs from vendor documentation, copy them EXACTLY as provided - do NOT modify, shorten, or rephrase URLs
- Output the markdown content directly without code block wrappers
- Document ALL advanced settings with their warnings/caveats
`

// Build creates the underlying ADK agent.
func (g *GeneratorAgent) Build(ctx context.Context, cfg validators.AgentConfig) (agent.Agent, error) {
	// Note: CachedContent is not compatible with ADK llmagent because
	// Gemini doesn't allow CachedContent with system_instruction or tools.
	// We rely on Gemini's implicit caching for repeated content.
	return llmagent.New(llmagent.Config{
		Name:        generatorAgentName,
		Description: generatorAgentDescription,
		Model:       cfg.Model,
		Instruction: generatorInstruction,
		Tools:       cfg.Tools,
		Toolsets:    cfg.Toolsets,
	})
}
