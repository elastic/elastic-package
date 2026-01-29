// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package parsing

import (
	"fmt"
	"strings"
)

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
