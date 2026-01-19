// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package docagent

import (
	"strings"

	"github.com/elastic/elastic-package/internal/llmagent/docagent/parsing"
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
	content := result.FinalContent

	// Log warning for empty responses
	if strings.TrimSpace(content) == "" {
		logger.Warnf("LLM returned empty response for section: %s", sectionTitle)
	}

	return parsing.ExtractSectionFromLLMResponse(content, sectionTitle, emptySectionPlaceholder)
}
