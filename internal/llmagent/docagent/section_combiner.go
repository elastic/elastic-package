// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package docagent

import (
	"strings"
)

// CombineSections takes a list of generated sections and combines them into a single document
// This flattens hierarchical sections (level 2 with level 3 subsections) into a single markdown document
func CombineSections(sections []Section) string {
	var result strings.Builder

	for i, section := range sections {
		// Use GetAllContent() to include subsections
		content := section.GetAllContent()

		// Ensure the content is properly formatted (trim trailing whitespace only)
		content = strings.TrimRight(content, " \t\n")

		// Add the content
		result.WriteString(content)

		// Add consistent spacing between sections
		if i < len(sections)-1 {
			result.WriteString("\n\n")
		}
	}

	// Ensure file ends with a single newline
	finalContent := result.String()
	if len(finalContent) > 0 && !strings.HasSuffix(finalContent, "\n") {
		finalContent += "\n"
	}

	return finalContent
}

// NormalizeMarkdown ensures consistent markdown formatting
func NormalizeMarkdown(content string) string {
	lines := strings.Split(content, "\n")
	var normalized []string

	for i, line := range lines {
		// Trim trailing whitespace
		line = strings.TrimRight(line, " \t")

		// Ensure proper spacing around headers
		if strings.HasPrefix(line, "#") {
			// Add blank line before header (except for first line)
			if i > 0 && normalized[len(normalized)-1] != "" {
				normalized = append(normalized, "")
			}
			normalized = append(normalized, line)
		} else {
			normalized = append(normalized, line)
		}
	}

	return strings.Join(normalized, "\n")
}

// PreserveSectionFormatting ensures that special directives and PRESERVE blocks are maintained
func PreserveSectionFormatting(content string) string {
	// This function ensures that template directives like {{ fields "name" }} are preserved
	// They should already be in the content, but this is a safety check

	// For now, we just return the content as-is since the section generation
	// should already preserve these elements
	return content
}
