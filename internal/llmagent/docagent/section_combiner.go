// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package docagent

import (
	"fmt"
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

// CombineSectionsWithTitle combines sections and prepends a title section
// The title follows the template format: "# {PackageTitle} Integration for Elastic"
func CombineSectionsWithTitle(sections []Section, packageTitle string) string {
	// Build the title with AI-generated notice
	title := fmt.Sprintf("# %s Integration for Elastic\n\n> **Note**: This documentation was generated using AI and should be reviewed for accuracy.\n\n", packageTitle)

	// Combine sections
	sectionsContent := CombineSections(sections)

	// Prepend title to sections
	return title + sectionsContent
}

// EnsureDocumentTitle ensures the document starts with the correct title format.
// If the document already has an H1 title, it checks if it matches the expected format.
// Returns the content with the correct title.
func EnsureDocumentTitle(content, packageTitle string) string {
	expectedTitle := fmt.Sprintf("# %s Integration for Elastic", packageTitle)
	aiNotice := "> **Note**: This documentation was generated using AI and should be reviewed for accuracy."

	lines := strings.Split(content, "\n")

	// Check if document starts with H1
	if len(lines) > 0 && strings.HasPrefix(lines[0], "# ") {
		currentTitle := strings.TrimSpace(lines[0])
		if currentTitle == expectedTitle {
			// Title is correct, check for AI notice
			if len(lines) > 2 && strings.TrimSpace(lines[2]) == aiNotice {
				return content // Already correct
			}
			// Add AI notice after title
			newContent := expectedTitle + "\n\n" + aiNotice + "\n\n"
			// Skip the old title and any empty lines
			i := 1
			for i < len(lines) && strings.TrimSpace(lines[i]) == "" {
				i++
			}
			newContent += strings.Join(lines[i:], "\n")
			return newContent
		}
		// Title exists but is wrong format, replace it
		newContent := expectedTitle + "\n\n" + aiNotice + "\n\n"
		// Skip the old title and any empty lines
		i := 1
		for i < len(lines) && strings.TrimSpace(lines[i]) == "" {
			i++
		}
		newContent += strings.Join(lines[i:], "\n")
		return newContent
	}

	// No H1 title, prepend one
	return expectedTitle + "\n\n" + aiNotice + "\n\n" + content
}

