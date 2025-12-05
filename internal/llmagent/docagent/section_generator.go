// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package docagent

import (
	"context"
	"fmt"

	"github.com/elastic/elastic-package/internal/llmagent/agent"
	"github.com/elastic/elastic-package/internal/llmagent/tools"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/packages/archetype"
)

// SectionGenerationContext holds all the context needed to generate a single section
type SectionGenerationContext struct {
	Section         Section
	TemplateSection *Section
	ExampleSection  *Section
	PackageInfo     PromptContext
	ExistingContent string
}

// GenerateAllSections orchestrates the generation of all sections for a document
func (d *DocumentationAgent) GenerateAllSections(ctx context.Context) ([]Section, error) {
	// Get the template content
	templateContent := archetype.GetPackageDocsReadmeTemplate()

	// Get the example content (default example from category-based system)
	exampleContent := tools.GetDefaultExampleContent()

	// Parse sections from template
	templateSections := ParseSections(templateContent)
	if len(templateSections) == 0 {
		return nil, fmt.Errorf("no sections found in template")
	}

	// Parse sections from example
	exampleSections := ParseSections(exampleContent)

	// Read existing documentation if it exists
	existingContent, _ := d.readCurrentReadme()
	var existingSections []Section
	if existingContent != "" {
		existingSections = ParseSections(existingContent)
	}

	// Generate ONLY top-level sections (subsections will be generated as part of parent)
	var generatedSections []Section

	for _, templateSection := range templateSections {
		// Skip subsections - they're generated with their parent
		if !templateSection.IsTopLevel() {
			continue
		}

		logger.Debugf("Generating section: %s (level %d) with %d subsections",
			templateSection.Title, templateSection.Level, len(templateSection.Subsections))

		// Find corresponding example section
		exampleSection := FindSectionByTitle(exampleSections, templateSection.Title)

		// Find existing section for this part
		var existingSection *Section
		if len(existingSections) > 0 {
			existingSection = FindSectionByTitle(existingSections, templateSection.Title)
		}

		// Build context for this section (includes subsection information via FullContent)
		sectionCtx := SectionGenerationContext{
			Section:         templateSection,
			TemplateSection: &templateSection,
			ExampleSection:  exampleSection,
			PackageInfo:     d.createPromptContext(d.manifest, ""),
		}

		if existingSection != nil {
			// Use FullContent to include subsections context
			sectionCtx.ExistingContent = existingSection.GetAllContent()
		}

		// Generate this section (includes subsections)
		generatedSection, err := d.generateSingleSection(ctx, sectionCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to generate section %s: %w", templateSection.Title, err)
		}

		// Parse the generated content to extract hierarchical structure
		parsedGenerated := ParseSections(generatedSection.Content)
		if len(parsedGenerated) > 0 {
			// Take the full hierarchical section (with subsections parsed)
			generatedSection = parsedGenerated[0]
		}

		generatedSections = append(generatedSections, generatedSection)
	}

	return generatedSections, nil
}

// generateSingleSection generates content for a single section using the LLM
func (d *DocumentationAgent) generateSingleSection(ctx context.Context, sectionCtx SectionGenerationContext) (Section, error) {
	// Build the prompt for this specific section
	prompt := d.buildSectionPrompt(sectionCtx)

	// Execute the task
	result, err := d.llmAgent.ExecuteTask(ctx, prompt)
	if err != nil {
		return Section{}, fmt.Errorf("agent task failed: %w", err)
	}

	// Log the result
	d.logAgentResponse(result)

	// Analyze the response
	analysis := d.responseAnalyzer.AnalyzeResponse(result.FinalContent, result.Conversation)
	if analysis.Status == responseError {
		return Section{}, fmt.Errorf("LLM reported an error: %s", analysis.Message)
	}

	// Extract the generated content from the tool results
	// The LLM should have written to a temporary location or returned the content
	generatedContent := d.extractGeneratedSectionContent(result, sectionCtx.Section.Title)

	// Create the section with generated content
	generatedSection := Section{
		Title:           sectionCtx.Section.Title,
		Level:           sectionCtx.Section.Level,
		Content:         generatedContent,
		HasPreserve:     sectionCtx.Section.HasPreserve,
		PreserveContent: sectionCtx.Section.PreserveContent,
	}

	return generatedSection, nil
}

// extractGeneratedSectionContent extracts the generated section content from the LLM response
func (d *DocumentationAgent) extractGeneratedSectionContent(result *agent.TaskResult, sectionTitle string) string {
	// Look through the conversation for the generated content
	// The LLM might have:
	// 1. Returned the content directly in the final response
	// 2. Used a tool to write it somewhere

	// For now, we'll look for the content in the final response
	// This assumes the LLM returns the markdown content directly
	content := result.FinalContent

	// If the content starts with thinking or explanatory text, try to extract just the markdown
	// Look for the section header
	lines := []string{}
	inSection := false
	for _, line := range splitLines(content) {
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
		return joinLines(lines)
	}

	return content
}

// Helper functions for content extraction

func splitLines(content string) []string {
	// Split by newlines, handling both \n and \r\n
	var lines []string
	current := ""
	for _, ch := range content {
		if ch == '\n' {
			lines = append(lines, current)
			current = ""
		} else if ch != '\r' {
			current += string(ch)
		}
	}
	if current != "" {
		lines = append(lines, current)
	}
	return lines
}

func joinLines(lines []string) string {
	result := ""
	for i, line := range lines {
		if i > 0 {
			result += "\n"
		}
		result += line
	}
	return result
}

func startsWithHeader(line, title string, level int) bool {
	prefix := ""
	for i := 0; i < level; i++ {
		prefix += "#"
	}
	prefix += " "

	if !hasPrefix(line, prefix) {
		return false
	}

	lineTitle := trimSpace(line[len(prefix):])
	return toLower(lineTitle) == toLower(trimSpace(title))
}

func hasPrefix(s, prefix string) bool {
	if len(s) < len(prefix) {
		return false
	}
	return s[:len(prefix)] == prefix
}

func trimSpace(s string) string {
	// Simple trim implementation
	start := 0
	end := len(s)

	for start < end && isSpace(s[start]) {
		start++
	}

	for end > start && isSpace(s[end-1]) {
		end--
	}

	return s[start:end]
}

func isSpace(ch byte) bool {
	return ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r'
}

func toLower(s string) string {
	result := ""
	for _, ch := range s {
		if ch >= 'A' && ch <= 'Z' {
			result += string(ch + 32)
		} else {
			result += string(ch)
		}
	}
	return result
}
