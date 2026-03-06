// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package prompts

// CriticalFormattingRules is a condensed version for user prompt reinforcement.
// These rules address the most common rejection reasons and should be included
// in user prompts to reinforce the system instruction.
const CriticalFormattingRules = `## CRITICAL FORMATTING REMINDERS
- Every list MUST have an introductory sentence ending with colon
- Use sentence case for headings: "Vendor-specific issues" not "Vendor-Specific Issues"
- In lists, do not use bolding for conceptual emphasis.
- NEVER use anchor links like [text](#section-name) - these break when published
- NEVER use docs-content:// protocol links - use full https://www.elastic.co/... URLs
- No exclamation points in body text
- No ellipses (...)
- Do not use first person (I, me, my) - address the user as "you"
`

// CriticRejectionCriteria provides criteria for the critic to check.
const CriticRejectionCriteria = `## REJECT if you find ANY of these:
- Lists without an introductory sentence before them
- Title Case headings: '### Vendor-Specific Issues' should be '### Vendor-specific issues'
- Anchor links like [Reference](#reference) - these break when published
- Internal docs-content:// links - must use full https://www.elastic.co/... URLs
- First person pronouns (I, me, my) - address the user as "you"
- Exclamation points in body text
- Ellipses (...)
- Version terms: use "later"/"earlier" instead of "newer"/"older"/"higher"/"lower"
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

### FORBIDDEN LINK STYLES (will break when published):
- Anchor links: NEVER use [text](#section-name) - these internal anchors break in the published docs
- docs-content:// protocol: NEVER use docs-content://path - use full URLs like https://www.elastic.co/guide/...
- To reference another section, describe it by name without linking: "see the Reference section below"

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
- Do NOT use first person (I, me, my) - address the user as "you"
- Use exclamation points sparingly - prefer periods
- Do not use ellipses (...)

WRONG: "The user must configure the integration before data can be collected."
RIGHT: "Before you can collect data, configure the integration settings."

### WORDINESS - prefer concise phrasing:
- "in order to" → "to"
- "utilize" / "make use of" → "use"
- "due to the fact that" / "because of the fact that" → "because"
- "at the present time" / "at this point in time" → "now"
- "has the ability to" / "has the capacity to" → "can"
- "in the event that" → "if"
- "prior to" / "previous to" → "before"
- "subsequent to" → "after"

### DONT USE these words/phrases:
- "just", "please", "and/or", "note that", "realtime" (use "real time" or "real-time")
- "thus", "very", "quite", "at this point", "a.k.a." / "aka"

### LATIN TERMS - always replace:
- "e.g." / "eg" → "for example"
- "i.e." / "ie" → "that is"
- "via" → "using" or "through"
- "vs" → "versus"
- "ad hoc" → "if needed"
- "vice versa" → "and the reverse"

### VERSIONS - use "later"/"earlier":
- "and higher" / "and newer" / "or higher" / "or newer" → "and later" / "or later"
- "and lower" / "and older" / "or lower" / "or older" → "and earlier" / "or earlier"
- "newer version" / "higher version" → "later version"
- "older version" / "lower version" → "earlier version"

### NEGATIONS:
- Prefer stating what something IS rather than what it is NOT
- Avoid "cannot X without" — rephrase positively (for example, "you must Y before X")

### DEVICE-AGNOSTIC LANGUAGE:
- Use "select" instead of "tap" or "click"
- Use "select and hold" instead of "long press"
- Use "zoom out" instead of "pinch"

## CONSISTENCY REQUIREMENTS (CRITICAL)
These ensure all integration docs look uniform:

### Heading Style
- Use sentence case for ALL headings: "### Vendor-specific issues" NOT "### Vendor-Specific Issues"
- Only the first word is capitalized (plus proper nouns and acronyms like TCP, UDP, API)

### Subsection Naming (use EXACTLY these names)
Under ## How do I deploy this integration? > ### Set up steps in [Product Name]:
- "#### Vendor resources" (links to vendor documentation at the end of vendor setup)

Under ## Troubleshooting:
- "### Common configuration issues" (use Problem-Solution bullet format, NOT tables)

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
