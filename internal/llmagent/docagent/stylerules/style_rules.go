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
- NEVER use bold for list items: '- **Item**:' is WRONG, use '- Item:'
- Bold is ONLY for UI elements: **Settings** > **Save**, **Discover** app
- Every list MUST have an introductory sentence ending with colon
- Use sentence case for headings: "General debugging steps" not "General Debugging Steps"
`

// CriticRejectionCriteria provides criteria for the critic to check.
const CriticRejectionCriteria = `## REJECT if you find ANY of these:
- Bold used for list items: '- **Security monitoring**:' should be '- Security monitoring:'
- Bold used for notes: '**Note**:' should be 'Note:'
- Bold used for concepts: '**Fault tolerance**:' should be 'Fault tolerance:'
- Lists without an introductory sentence before them
- Title Case headings: '### General Debugging Steps' should be '### General debugging steps'
`

// FullFormattingRules contains the complete formatting guidance for LLM system prompts.
// This is extracted from agent_instructions.md to ensure consistency.
const FullFormattingRules = `## FORMATTING RULES - READ BEFORE GENERATING (CRITICAL - WILL BE REJECTED IF VIOLATED)

### NEVER USE BOLD FOR LIST ITEMS (this is the #1 reason for rejection):

WRONG - This WILL be rejected:
This integration facilitates:
- **Security monitoring**: Ingests audit logs...
- **Operational visibility**: Collects logs...
- **Performance analysis**: Gathers metrics...

RIGHT - Use plain text:
This integration facilitates:
- Security monitoring: Ingests audit logs...
- Operational visibility: Collects logs...
- Performance analysis: Gathers metrics...

### MORE WRONG PATTERNS (never use these):
- "**Syslog**:", "**TCP**:", "**Audit logs**:" → WRONG
- "**Fault tolerance**:", "**Scaling guidance**:" → WRONG  
- "**Note**:", "**Warning**:", "**Important**:" → WRONG
- "**TCP Socket Method**:", "**File Method**:" → WRONG
- "**Permissions**:", "**Network access**:" → WRONG
- "**Audit device is not enabled**:" → WRONG
- "**No data is being collected**:" → WRONG

### ONLY USE BOLD FOR UI ELEMENTS:
- Menu paths: **Settings** > **Logging**
- Buttons: Click **Save**
- Field names in UI: In the **Host** field

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
- Use sentence case: "### General debugging steps" NOT "### General Debugging Steps"
`

