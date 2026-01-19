// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package parsing

import (
	"fmt"
	"strings"
)

// ExtractMarkdownFromCodeBlock extracts markdown content from a response that may have code fences.
// It looks for ```markdown blocks first, then falls back to generic ``` blocks.
// Returns empty string if no code block is found.
func ExtractMarkdownFromCodeBlock(response string) string {
	// Look for markdown code block
	if idx := strings.Index(response, "```markdown"); idx != -1 {
		start := idx + len("```markdown")
		if end := strings.Index(response[start:], "```"); end != -1 {
			return strings.TrimSpace(response[start : start+end])
		}
	}
	// Look for generic code block
	if idx := strings.Index(response, "```"); idx != -1 {
		start := idx + 3
		// Skip language identifier if present
		if newline := strings.Index(response[start:], "\n"); newline != -1 {
			start += newline + 1
		}
		if end := strings.Index(response[start:], "```"); end != -1 {
			return strings.TrimSpace(response[start : start+end])
		}
	}
	return ""
}

// ExtractContentFromHeading extracts content starting from the first markdown heading.
// Looks for lines starting with "# " or "---" (frontmatter).
// Returns empty string if no heading is found.
func ExtractContentFromHeading(response string) string {
	lines := strings.Split(response, "\n")
	startIdx := -1

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Found a markdown heading or frontmatter
		if strings.HasPrefix(trimmed, "# ") || strings.HasPrefix(trimmed, "---") {
			startIdx = i
			break
		}
	}

	if startIdx == -1 {
		return ""
	}

	return strings.TrimSpace(strings.Join(lines[startIdx:], "\n"))
}

// ExtractMarkdownContent extracts markdown content from an LLM response.
// If the content is wrapped in code blocks, extracts the inner content.
// Otherwise returns the original content.
func ExtractMarkdownContent(content string) string {
	// Check if content is wrapped in markdown code blocks
	if strings.HasPrefix(strings.TrimSpace(content), "```") {
		lines := strings.Split(content, "\n")
		var result strings.Builder
		inCodeBlock := false
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "```") {
				inCodeBlock = !inCodeBlock
				continue
			}
			if inCodeBlock {
				result.WriteString(line)
				result.WriteString("\n")
			}
		}
		extracted := result.String()
		if extracted != "" {
			return strings.TrimSpace(extracted)
		}
	}
	return content
}

// ExtractSectionFromLLMResponse extracts generated section content from an LLM response.
// It handles cases where the LLM includes explanatory text before the actual markdown.
// If no section header is found, it wraps the content with the section header.
// Returns a placeholder if the content is empty.
func ExtractSectionFromLLMResponse(response, sectionTitle string, emptySectionPlaceholder string) string {
	content := response

	// Handle empty response
	if strings.TrimSpace(content) == "" {
		return fmt.Sprintf("## %s\n\n%s", sectionTitle, emptySectionPlaceholder)
	}

	// If the content starts with thinking or explanatory text, try to extract just the markdown
	// Look for the section header
	var lines []string
	inSection := false
	for _, line := range strings.Split(content, "\n") {
		// Check if this line is the section header we're looking for
		if !inSection && (StartsWithHeader(line, sectionTitle, 2) || StartsWithHeader(line, sectionTitle, 3)) {
			inSection = true
		}

		if inSection {
			lines = append(lines, line)
		}
	}

	// If we found section content, use it
	if len(lines) > 0 {
		return strings.Join(lines, "\n")
	}

	// If no section header was found but content exists, add the header
	if strings.TrimSpace(content) != "" {
		return fmt.Sprintf("## %s\n\n%s", sectionTitle, content)
	}

	return fmt.Sprintf("## %s\n\n%s", sectionTitle, emptySectionPlaceholder)
}

