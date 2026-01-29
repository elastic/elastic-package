// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package docagent

import (
	"regexp"
	"strings"

	"github.com/elastic/elastic-package/internal/llmagent/docagent/specialists/validators"
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

// hasFieldsTemplate checks if {{fields "name"}} exists (with flexible whitespace)
func hasFieldsTemplate(content, dsName string) bool {
	pattern := regexp.MustCompile(`\{\{\s*fields\s+"` + regexp.QuoteMeta(dsName) + `"\s*\}\}`)
	return pattern.MatchString(content)
}

// hasEventTemplate checks if {{event "name"}} exists (with flexible whitespace)
func hasEventTemplate(content, dsName string) bool {
	pattern := regexp.MustCompile(`\{\{\s*event\s+"` + regexp.QuoteMeta(dsName) + `"\s*\}\}`)
	return pattern.MatchString(content)
}

// hasDataStreamSubsection checks if a ### {dsName} subsection exists in the Reference section
func hasDataStreamSubsection(content, dsName string) bool {
	// Match case-insensitive subsection heading
	pattern := regexp.MustCompile(`(?mi)^###\s+` + regexp.QuoteMeta(dsName) + `\s*$`)
	return pattern.MatchString(content)
}

// insertFieldsTemplate inserts {{fields "name"}} into the data stream's subsection
func insertFieldsTemplate(content, dsName string) string {
	// First check if the data stream subsection exists
	if !hasDataStreamSubsection(content, dsName) {
		// Subsection doesn't exist - create it with the template
		return appendDataStreamSubsection(content, dsName, false, true)
	}

	// Find the data stream subsection heading
	headingPattern := regexp.MustCompile(`(?mi)^###\s+` + regexp.QuoteMeta(dsName) + `\s*$`)
	loc := headingPattern.FindStringIndex(content)
	if loc == nil {
		return content
	}

	// Find where to insert (before next ### or ## heading, or end of content)
	rest := content[loc[1]:]
	nextHeadingPattern := regexp.MustCompile(`(?m)^##`)
	nextLoc := nextHeadingPattern.FindStringIndex(rest)

	insertPos := len(content)
	if nextLoc != nil {
		insertPos = loc[1] + nextLoc[0]
	}

	// Check if there's already content before the next heading (avoid double newlines)
	beforeInsert := strings.TrimRight(content[:insertPos], " \t\n")
	afterInsert := strings.TrimLeft(content[insertPos:], " \t")

	// Insert the template before the next heading
	template := "\n\n{{fields \"" + dsName + "\"}}"

	return beforeInsert + template + "\n\n" + afterInsert
}

// insertEventTemplate inserts {{event "name"}} before {{fields "name"}}
func insertEventTemplate(content, dsName string) string {
	// First check if the data stream subsection exists
	if !hasDataStreamSubsection(content, dsName) {
		// Subsection doesn't exist - create it with both templates
		return appendDataStreamSubsection(content, dsName, true, true)
	}

	// Check if {{fields}} template exists - insert event before it
	fieldsPattern := regexp.MustCompile(`\{\{\s*fields\s+"` + regexp.QuoteMeta(dsName) + `"\s*\}\}`)
	loc := fieldsPattern.FindStringIndex(content)
	if loc != nil {
		// Insert event template before fields template
		template := "{{event \"" + dsName + "\"}}\n\n"
		return content[:loc[0]] + template + content[loc[0]:]
	}

	// No fields template found - find the subsection and insert both
	headingPattern := regexp.MustCompile(`(?mi)^###\s+` + regexp.QuoteMeta(dsName) + `\s*$`)
	headingLoc := headingPattern.FindStringIndex(content)
	if headingLoc == nil {
		return content
	}

	// Find where to insert (before next ### or ## heading)
	rest := content[headingLoc[1]:]
	nextHeadingPattern := regexp.MustCompile(`(?m)^##`)
	nextLoc := nextHeadingPattern.FindStringIndex(rest)

	insertPos := len(content)
	if nextLoc != nil {
		insertPos = headingLoc[1] + nextLoc[0]
	}

	beforeInsert := strings.TrimRight(content[:insertPos], " \t\n")
	afterInsert := strings.TrimLeft(content[insertPos:], " \t")

	template := "\n\n{{event \"" + dsName + "\"}}"

	return beforeInsert + template + "\n\n" + afterInsert
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

