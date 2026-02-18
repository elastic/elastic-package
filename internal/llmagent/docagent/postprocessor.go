// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package docagent

import (
	"regexp"
	"strings"

	"github.com/elastic/elastic-package/internal/llmagent/docagent/agents/validators"
	"github.com/elastic/elastic-package/internal/logger"
)

// EnsureDataStreamTemplates ensures all data streams have {{event}} and {{fields}} templates
// in the Reference section. If templates are missing, they are inserted programmatically.
// This provides a safety net for cases where the LLM fails to include the required templates.
func (d *DocumentationAgent) EnsureDataStreamTemplates(content string, pkgCtx *validators.PackageContext) string {
	if pkgCtx == nil || len(pkgCtx.DataStreams) == 0 {
		return content
	}

	// Check if Reference section exists
	if !hasReferenceSection(content) {
		logger.Debugf("Post-processor: No Reference section found, skipping template insertion")
		return content
	}

	modified := false
	for _, ds := range pkgCtx.DataStreams {
		// Ensure {{fields "dsName"}} exists
		if !hasFieldsTemplate(content, ds.Name) {
			content = insertFieldsTemplate(content, ds.Name)
			modified = true
			logger.Debugf("Post-processor: Added {{fields \"%s\"}} template", ds.Name)
		}

		// Ensure {{event "dsName"}} exists if data stream has sample_event.json
		if ds.HasExampleEvent && !hasEventTemplate(content, ds.Name) {
			content = insertEventTemplate(content, ds.Name)
			modified = true
			logger.Debugf("Post-processor: Added {{event \"%s\"}} template", ds.Name)
		}
		// Remove {{event "dsName"}} if data stream has no example event
		if !ds.HasExampleEvent && hasEventTemplate(content, ds.Name) {
			content = removeEventTemplate(content, ds.Name)
			modified = true
			logger.Debugf("Post-processor: Removed {{event \"%s\"}} template (no example event in data stream)", ds.Name)
		}
	}

	if modified {
		logger.Debugf("Post-processor: Completed adding missing data stream templates to Reference section")
	}

	return content
}

// hasReferenceSection checks if the document has a Reference section
func hasReferenceSection(content string) bool {
	pattern := regexp.MustCompile(`(?mi)^##\s+Reference\s*$`)
	return pattern.MatchString(content)
}

// templateIndex returns the start index of {{kind "dsName"}} in content (with flexible whitespace), or -1.
func templateIndex(content, kind, dsName string) int {
	pattern := regexp.MustCompile(`\{\{\s*` + kind + `\s+"` + regexp.QuoteMeta(dsName) + `"\s*\}\}`)
	loc := pattern.FindStringIndex(content)
	if loc == nil {
		return -1
	}
	return loc[0]
}

func hasFieldsTemplate(content, dsName string) bool {
	return templateIndex(content, "fields", dsName) >= 0
}

func hasEventTemplate(content, dsName string) bool {
	return templateIndex(content, "event", dsName) >= 0
}

// removeEventTemplate removes {{event "name"}} and surrounding newlines from content
func removeEventTemplate(content, dsName string) string {
	pattern := regexp.MustCompile(`\n*\s*\{\{\s*event\s+"` + regexp.QuoteMeta(dsName) + `"\s*\}\}\s*\n*`)
	return pattern.ReplaceAllString(content, "\n\n")
}

// hasDataStreamSubsection checks if a ### {dsName} subsection exists in the Reference section
func hasDataStreamSubsection(content, dsName string) bool {
	pattern := regexp.MustCompile(`(?mi)^###\s+` + regexp.QuoteMeta(dsName) + `\s*$`)
	return pattern.MatchString(content)
}

var nextH2Regex = regexp.MustCompile(`(?m)^##`)

// findSubsectionEnd returns the content index just before the next ## after the ### dsName subsection, or len(content).
func findSubsectionEnd(content, dsName string) (int, bool) {
	headingPattern := regexp.MustCompile(`(?mi)^###\s+` + regexp.QuoteMeta(dsName) + `\s*$`)
	loc := headingPattern.FindStringIndex(content)
	if loc == nil {
		return 0, false
	}
	nextLoc := nextH2Regex.FindStringIndex(content[loc[1]:])
	if nextLoc == nil {
		return len(content), true
	}
	return loc[1] + nextLoc[0], true
}

// insertAtEnd inserts toInsert at endPos, trimming surrounding whitespace.
func insertAtEnd(content string, endPos int, toInsert string) string {
	before := strings.TrimRight(content[:endPos], " \t\n")
	after := strings.TrimLeft(content[endPos:], " \t")
	return before + toInsert + "\n\n" + after
}

// insertFieldsTemplate inserts {{fields "name"}} into the data stream's subsection
func insertFieldsTemplate(content, dsName string) string {
	if !hasDataStreamSubsection(content, dsName) {
		return appendDataStreamSubsection(content, dsName, false, true)
	}
	endPos, ok := findSubsectionEnd(content, dsName)
	if !ok {
		return content
	}
	return insertAtEnd(content, endPos, "\n\n{{fields \""+dsName+"\"}}")
}

// insertEventTemplate inserts {{event "name"}} before {{fields "name"}} or at subsection end
func insertEventTemplate(content, dsName string) string {
	if !hasDataStreamSubsection(content, dsName) {
		return appendDataStreamSubsection(content, dsName, true, true)
	}
	if idx := templateIndex(content, "fields", dsName); idx >= 0 {
		return content[:idx] + "{{event \"" + dsName + "\"}}\n\n" + content[idx:]
	}
	endPos, ok := findSubsectionEnd(content, dsName)
	if !ok {
		return content
	}
	return insertAtEnd(content, endPos, "\n\n{{event \""+dsName+"\"}}")
}

// appendDataStreamSubsection appends a complete data stream subsection to the Reference section
func appendDataStreamSubsection(content, dsName string, needsEvent, needsFields bool) string {
	// Find ## Reference section
	refPattern := regexp.MustCompile(`(?mi)^##\s+Reference\s*$`)
	loc := refPattern.FindStringIndex(content)
	if loc == nil {
		return content // Can't find Reference section
	}

	// Find the next ## heading after Reference (not ###)
	rest := content[loc[1]:]
	// Match ## but not ### (i.e., only H2 headings)
	nextH2Pattern := regexp.MustCompile(`(?m)^##\s+[^#]`)
	nextLoc := nextH2Pattern.FindStringIndex(rest)

	insertPos := len(content)
	if nextLoc != nil {
		insertPos = loc[1] + nextLoc[0]
	}

	// Build the subsection
	var sb strings.Builder
	sb.WriteString("\n\n### ")
	sb.WriteString(dsName)
	sb.WriteString("\n\n")

	// Add description placeholder
	sb.WriteString("The `")
	sb.WriteString(dsName)
	sb.WriteString("` data stream.\n\n")

	if needsEvent {
		sb.WriteString("{{event \"")
		sb.WriteString(dsName)
		sb.WriteString("\"}}\n\n")
	}
	if needsFields {
		sb.WriteString("{{fields \"")
		sb.WriteString(dsName)
		sb.WriteString("\"}}")
	}

	beforeInsert := strings.TrimRight(content[:insertPos], " \t\n")
	afterInsert := content[insertPos:]

	return beforeInsert + sb.String() + "\n\n" + afterInsert
}

// Canonical Agentless deployment subsection block (matches package-docs-readme.md.tmpl).
const agentlessSectionBlock = `### Agentless deployment

Agentless deployments are only supported in Elastic Serverless and Elastic Cloud environments. Agentless deployments provide a means to ingest data while avoiding the orchestration, management, and maintenance needs associated with standard ingest infrastructure. Using an agentless deployment makes manual agent deployment unnecessary, allowing you to focus on your data instead of the agent that collects it.

For more information, refer to [Agentless integrations](https://www.elastic.co/guide/en/serverless/current/security-agentless-integrations.html) and [Agentless integrations FAQ](https://www.elastic.co/guide/en/serverless/current/agentless-integration-troubleshooting.html)`

var (
	howDoIDeploySectionRegex = regexp.MustCompile(`(?mi)^##\s+How do I deploy this integration\?\s*$`)
	agentBasedHeadingRegex    = regexp.MustCompile(`(?mi)^###\s+Agent-based deployment\s*$`)
	agentlessHeadingRegex    = regexp.MustCompile(`(?mi)^###\s+Agentless deployment\s*$`)
	nextH2OrH3Regex          = regexp.MustCompile(`(?m)^(?:##|###)\s+`)
)

// EnsureAgentlessSection ensures the Agentless deployment subsection is present iff the package has agentless enabled.
func (d *DocumentationAgent) EnsureAgentlessSection(content string, pkgCtx *validators.PackageContext) string {
	wantAgentless := pkgCtx != nil && pkgCtx.HasAgentlessEnabled()
	hasSection := hasAgentlessSection(content)
	if !wantAgentless {
		if hasSection {
			content = removeAgentlessSection(content)
			logger.Debugf("Post-processor: Removed Agentless deployment section (not enabled in package)")
		}
		return content
	}
	if !hasSection {
		content = insertAgentlessSection(content)
		logger.Debugf("Post-processor: Added Agentless deployment section")
	}
	return content
}

func hasAgentlessSection(content string) bool {
	return agentlessHeadingRegex.MatchString(content)
}

func removeAgentlessSection(content string) string {
	loc := agentlessHeadingRegex.FindStringIndex(content)
	if loc == nil {
		return content
	}
	start := loc[0]
	rest := content[loc[1]:]
	nextLoc := nextH2OrH3Regex.FindStringIndex(rest)
	var end int
	if nextLoc == nil {
		end = len(content)
	} else {
		end = loc[1] + nextLoc[0]
	}
	before := strings.TrimRight(content[:start], " \t\n")
	after := content[end:]
	if after != "" {
		after = "\n\n" + strings.TrimLeft(after, " \t\n")
	}
	return before + after
}

// findAfterDeploySectionHeading returns the byte index right after the "## How do I deploy this integration?" line, or -1.
func findAfterDeploySectionHeading(content string) int {
	loc := howDoIDeploySectionRegex.FindStringIndex(content)
	if loc == nil {
		return -1
	}
	after := loc[1]
	if after < len(content) && content[after] == '\n' {
		after++
	}
	return after
}

// findEndOfDeploySubsection returns the index just before the next ### or ## after the subsection at startPos, or len(content).
func findEndOfDeploySubsection(content string, startPos int) int {
	// Skip past the current subsection heading line so we find the next one
	afterLine := startPos
	if i := strings.Index(content[startPos:], "\n"); i >= 0 {
		afterLine = startPos + i + 1
	}
	rest := content[afterLine:]
	loc := nextH2OrH3Regex.FindStringIndex(rest)
	if loc == nil {
		return len(content)
	}
	return afterLine + loc[0]
}

func insertAgentlessSection(content string) string {
	afterHeading := findAfterDeploySectionHeading(content)
	if afterHeading < 0 {
		return content
	}
	agentBasedLoc := agentBasedHeadingRegex.FindStringIndex(content[afterHeading:])
	if agentBasedLoc != nil {
		// Insert after the end of the Agent-based deployment subsection
		agentBasedSubsectionStart := afterHeading + agentBasedLoc[0]
		endPos := findEndOfDeploySubsection(content, agentBasedSubsectionStart)
		return insertAtEnd(content, endPos, "\n\n"+agentlessSectionBlock)
	}
	// No Agent-based deployment: insert at start of "How do I deploy" section
	before := content[:afterHeading]
	after := content[afterHeading:]
	return before + "\n\n" + agentlessSectionBlock + "\n\n" + strings.TrimLeft(after, " \t\n")
}
