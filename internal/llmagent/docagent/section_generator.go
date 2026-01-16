// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package docagent

import (
	"fmt"
	"strings"

	"github.com/elastic/elastic-package/internal/llmagent/docagent/specialists/validators"
	"github.com/elastic/elastic-package/internal/logger"
)

// SectionGenerationContext holds all the context needed to generate a single section
type SectionGenerationContext struct {
	Section         Section
	TemplateSection *Section
	ExampleSection  *Section
	PackageInfo     PromptContext
	ExistingContent string
	PackageContext  *validators.PackageContext // For section-specific instructions
}

// emptySectionPlaceholder is the placeholder text for sections that couldn't be populated
const emptySectionPlaceholder = "<< SECTION NOT POPULATED! Add appropriate text, or remove the section. >>"

// extractGeneratedSectionContent extracts the generated section content from the LLM response
func (d *DocumentationAgent) extractGeneratedSectionContent(result *TaskResult, sectionTitle string) string {
	// Look through the conversation for the generated content
	// The LLM might have:
	// 1. Returned the content directly in the final response
	// 2. Used a tool to write it somewhere

	// For now, we'll look for the content in the final response
	// This assumes the LLM returns the markdown content directly
	content := result.FinalContent

	// Handle empty response - return placeholder with section header
	if strings.TrimSpace(content) == "" {
		logger.Warnf("LLM returned empty response for section: %s", sectionTitle)
		return fmt.Sprintf("## %s\n\n%s", sectionTitle, emptySectionPlaceholder)
	}

	// If the content starts with thinking or explanatory text, try to extract just the markdown
	// Look for the section header
	lines := []string{}
	inSection := false
	for line := range strings.SplitSeq(content, "\n") {
		// Check if this line is the section header we're looking for
		if !inSection && (startsWithHeader(line, sectionTitle, 2) || startsWithHeader(line, sectionTitle, 3)) {
			inSection = true
		}

		if inSection {
			lines = append(lines, line)
		}
	}

	// If we found section content, use it; otherwise use the full content
	if len(lines) > 0 {
		return strings.Join(lines, "\n")
	}

	// If no section header was found but content exists, it might be just the content without a header
	// In this case, add the header ourselves if content is meaningful, otherwise return placeholder
	if strings.TrimSpace(content) != "" {
		return fmt.Sprintf("## %s\n\n%s", sectionTitle, content)
	}

	return fmt.Sprintf("## %s\n\n%s", sectionTitle, emptySectionPlaceholder)
}

// startsWithHeader checks if a line starts with a markdown header of the given level and title
func startsWithHeader(line, title string, level int) bool {
	prefix := ""
	for i := 0; i < level; i++ {
		prefix += "#"
	}
	prefix += " "

	if !strings.HasPrefix(line, prefix) {
		return false
	}

	lineTitle := strings.TrimSpace(line[len(prefix):])
	return strings.EqualFold(strings.ToLower(lineTitle), strings.ToLower(strings.TrimSpace(title)))
}
