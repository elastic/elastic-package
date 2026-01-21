// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

// Package stylerules provides shared formatting rules constants for documentation generation.
// This package has no dependencies on other docagent packages to avoid import cycles.
package stylerules

// CriticalFormattingRules is a condensed version for user prompt reinforcement.
// These rules address the most common rejection reasons and should be included
// in user prompts to reinforce the system instruction.
const CriticalFormattingRules = `## CRITICAL FORMATTING REMINDERS
- Every list MUST have an introductory sentence ending with colon
- Use sentence case for headings: "Vendor-specific issues" not "Vendor-Specific Issues"
- In lists, do not use bolding for conceptual emphasis.
`

// CriticRejectionCriteria provides criteria for the critic to check.
const CriticRejectionCriteria = `## REJECT if you find ANY of these:
- Lists without an introductory sentence before them
- Title Case headings: '### Vendor-Specific Issues' should be '### Vendor-specific issues'
`

// FullFormattingRules contains the complete formatting guidance for LLM system prompts.

const FullFormattingRules = `## FORMATTING RULES - READ BEFORE GENERATING

### EVERY LIST MUST HAVE AN INTRODUCTION:
WRONG:
- Item one
- Item two

RIGHT:
This integration supports the following:
- Item one
- Item two

### USE MONOSPACE FOR:
- Code: ` + "`vault audit enable`" + `
- File paths: ` + "`/var/log/vault/`" + `
- Config values: ` + "`true`" + `, ` + "`8200`" + `
- Data streams: ` + "`audit`" + `, ` + "`log`" + `

### HEADINGS:
- Use sentence case: "### Vendor-specific issues" NOT "### Vendor-Specific Issues"

## Guidelines
- Write clear, concise, and accurate documentation
- Follow the Elastic documentation style (friendly, direct, use "you")
- Include relevant code examples and configuration snippets where appropriate
- Use proper markdown formatting
- Use {{event "<name>"}} and {{fields "<name>"}} templates in Reference section, replacing <name> with the actual data stream name (e.g., {{event "conn"}}, {{fields "conn"}})
- For code blocks, ALWAYS specify the language (e.g., bash, yaml, json after the triple backticks)

## Voice and Tone (REQUIRED)
- Address the user directly with "you" - ALWAYS
- Use contractions: "you'll", "don't", "can't", "it's"
- Use active voice: "You configure..." not "The configuration is..."
- Be direct and friendly, not formal

WRONG: "The user must configure the integration before data can be collected."
RIGHT: "Before you can collect data, configure the integration settings."

## CONSISTENCY REQUIREMENTS (CRITICAL)
These ensure all integration docs look uniform:

### Heading Style
- Use sentence case for ALL headings: "### Vendor-specific issues" NOT "### Vendor-Specific Issues"
- Only the first word is capitalized (plus proper nouns and acronyms like TCP, UDP, API)

### Subsection Naming (use EXACTLY these names)
Under ## Troubleshooting:
- "### Common configuration issues" (use Problem-Solution bullet format, NOT tables)
- "### Vendor resources" (links to vendor documentation)

Under ## Reference:
- "### Inputs used" (required)
- "### API usage" (only for API-based integrations)
- "### Vendor documentation links" (if consolidating links at the end)

### Code Block Safety
- NEVER put bash comments (lines starting with #) outside code blocks!
- Wrong: # Test the connection ← This becomes an H1 heading!
- Right: Put it inside a bash code block:
  ` + "```" + `bash
  # Test the connection
  curl -v http://example.com
  ` + "```" + `

## CRITICAL
- Do NOT rename sections - use the EXACT SectionTitle provided
- When including URLs from vendor documentation, copy them EXACTLY as provided - do NOT modify, shorten, or rephrase URLs
- Output the markdown content directly without code block wrappers
- Document ALL advanced settings with their warnings/caveats when relevant to this section
- Do NOT include other sections - generate ONLY the one section requested
- Do NOT wrap your output in code blocks or add explanatory text

## IMPORTANT
Output the markdown content directly. Start with the section heading and include all relevant subsections.
`
