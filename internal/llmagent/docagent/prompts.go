// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package docagent

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/elastic/elastic-package/internal/configuration/locations"
	"github.com/elastic/elastic-package/internal/environment"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/packages/archetype"
	"github.com/elastic/elastic-package/internal/profile"
)

const (
	promptFileInitial              = "initial_prompt.txt"
	promptFileRevision             = "revision_prompt.txt"
	promptFileSectionGeneration    = "section_generation_prompt.txt"
	promptFileModificationAnalysis = "modification_analysis_prompt.txt"
	promptFileModification         = "modification_prompt.txt"
	promptFileReformat             = "reformat_prompt.txt"
)

type PromptType int

const (
	PromptTypeInitial PromptType = iota
	PromptTypeRevision
	PromptTypeSectionGeneration
	PromptTypeModificationAnalysis
	PromptTypeModification
	PromptTypeReformat
)

// loadPromptFile loads a prompt file from external location if enabled, otherwise uses embedded content
func loadPromptFile(filename string, embeddedContent string, profile *profile.Profile) string {
	// Check if external prompt files are enabled
	envVar := environment.WithElasticPackagePrefix("LLM_EXTERNAL_PROMPTS")
	configKey := "llm.external_prompts"
	useExternal := getConfigValue(profile, envVar, configKey, "false") == "true"

	if !useExternal {
		return embeddedContent
	}

	// Check in profile directory first if profile is available
	if profile != nil {
		profilePath := filepath.Join(profile.ProfilePath, "prompts", filename)
		if content, err := os.ReadFile(profilePath); err == nil {
			logger.Debugf("Loaded external prompt file from profile: %s", profilePath)
			return string(content)
		}
	}

	// Try to load from .elastic-package directory
	loc, err := locations.NewLocationManager()
	if err != nil {
		logger.Debugf("Failed to get location manager, using embedded prompt: %v", err)
		return embeddedContent
	}

	// Check in .elastic-package directory
	elasticPackagePath := filepath.Join(loc.RootDir(), "prompts", filename)
	if content, err := os.ReadFile(elasticPackagePath); err == nil {
		logger.Debugf("Loaded external prompt file from .elastic-package: %s", elasticPackagePath)
		return string(content)
	}

	// Fall back to embedded content
	logger.Debugf("External prompt file not found, using embedded content for: %s", filename)
	fmt.Printf("⚠️  Warning: External prompt file not found, using embedded content for: %s", filename)
	return embeddedContent
}

// getConfigValue retrieves a configuration value with fallback from environment variable to profile config
func getConfigValue(profile *profile.Profile, envVar, configKey, defaultValue string) string {
	// First check environment variable
	if envValue := os.Getenv(envVar); envValue != "" {
		return envValue
	}

	// Then check profile configuration
	if profile != nil {
		return profile.Config(configKey, defaultValue)
	}

	return defaultValue
}

// buildPrompt creates a prompt based on type and context
func (d *DocumentationAgent) buildPrompt(promptType PromptType, ctx PromptContext) string {
	var promptFile, embeddedContent string
	var formatArgs []interface{}

	switch promptType {
	case PromptTypeInitial:
		promptFile = promptFileInitial
		embeddedContent = InitialPrompt
		formatArgs = d.buildInitialPromptArgs(ctx)
	case PromptTypeRevision:
		promptFile = promptFileRevision
		embeddedContent = RevisionPrompt
		formatArgs = d.buildRevisionPromptArgs(ctx)
	case PromptTypeSectionGeneration:
		promptFile = promptFileSectionGeneration
		embeddedContent = SectionGenerationPrompt
		formatArgs = d.buildSectionGenerationPromptArgs(ctx)
	case PromptTypeModificationAnalysis:
		promptFile = promptFileModificationAnalysis
		embeddedContent = ModificationAnalysisPrompt
		formatArgs = d.buildModificationAnalysisPromptArgs(ctx)
	case PromptTypeModification:
		promptFile = promptFileModification
		embeddedContent = ModificationPrompt
		formatArgs = d.buildModificationPromptArgs(ctx)
	case PromptTypeReformat:
		// Reformat uses a separate method: buildReformatPrompt()
		// This case should not be reached in normal flow
		return ""
	}

	promptContent := loadPromptFile(promptFile, embeddedContent, d.profile)
	basePrompt := fmt.Sprintf(promptContent, formatArgs...)

	return basePrompt
}

// buildInitialPromptArgs prepares arguments for initial prompt
func (d *DocumentationAgent) buildInitialPromptArgs(ctx PromptContext) []interface{} {
	return []interface{}{
		ctx.TargetDocFile, // file path in task description
		ctx.Manifest.Name,
		ctx.Manifest.Title,
		ctx.Manifest.Type,
		ctx.Manifest.Version,
		ctx.Manifest.Description,
		ctx.TargetDocFile, // file restriction directive
		ctx.TargetDocFile, // tool usage guideline
		ctx.TargetDocFile, // initial analysis step
		ctx.TargetDocFile, // write results step
	}
}

// buildRevisionPromptArgs prepares arguments for revision prompt
func (d *DocumentationAgent) buildRevisionPromptArgs(ctx PromptContext) []interface{} {
	return []interface{}{
		ctx.TargetDocFile, // target documentation file label
		ctx.Manifest.Name,
		ctx.Manifest.Title,
		ctx.Manifest.Type,
		ctx.Manifest.Version,
		ctx.Manifest.Description,
		ctx.TargetDocFile, // file restriction directive
		ctx.TargetDocFile, // read current content directive
		ctx.TargetDocFile, // tool usage guideline
		ctx.TargetDocFile, // step 1 - read current file
		ctx.TargetDocFile, // step 7 - write documentation
		ctx.Changes,       // user-requested changes
	}
}

// buildSectionGenerationPromptArgs prepares arguments for section generation prompt
func (d *DocumentationAgent) buildSectionGenerationPromptArgs(ctx PromptContext) []interface{} {
	levelStr := "##"
	if ctx.SectionLevel == 3 {
		levelStr = "###"
	}
	levelName := "Level 2"
	if ctx.SectionLevel == 3 {
		levelName = "Level 3"
	}

	// Build preserve content section
	preserveSection := ""
	if ctx.PreserveContent != "" {
		preserveSection = fmt.Sprintf("\nPRESERVE Content (Must Include Verbatim):\n---\n%s\n---\n\n", ctx.PreserveContent)
	}

	return []interface{}{
		ctx.SectionTitle,         // section title in task description
		ctx.SectionLevel,         // section level number
		ctx.TargetDocFile,        // target file name
		ctx.SectionTitle,         // section title (repeated)
		levelName,                // level name (Level 2 or Level 3)
		levelStr,                 // level prefix (## or ###)
		ctx.Manifest.Name,        // package name
		ctx.Manifest.Title,       // package title
		ctx.Manifest.Type,        // package type
		ctx.Manifest.Version,     // package version
		ctx.Manifest.Description, // package description
		ctx.SectionTitle,         // section title for get_example in tool guidelines
		ctx.SectionTitle,         // section title for get_service_info in tool guidelines
		ctx.TemplateSection,      // template section content
		preserveSection,          // preserve content if any
		ctx.SectionTitle,         // section title for get_example in step 1
		ctx.SectionTitle,         // section title for get_service_info in step 3
		levelStr,                 // level prefix for step 4
		ctx.SectionTitle,         // section title for step 4
		levelStr,                 // level prefix for step 5
		ctx.SectionTitle,         // section title for step 5
	}
}

// buildModificationAnalysisPromptArgs prepares arguments for modification analysis prompt
func (d *DocumentationAgent) buildModificationAnalysisPromptArgs(ctx PromptContext) []interface{} {
	return []interface{}{
		ctx.TargetDocFile,        // target file
		ctx.Manifest.Name,        // package name
		ctx.Manifest.Title,       // package title
		ctx.Manifest.Type,        // package type
		ctx.Manifest.Version,     // package version
		ctx.Manifest.Description, // package description
		ctx.SectionTitle,         // section list (stored temporarily in SectionTitle field)
		ctx.Changes,              // modification request
	}
}

// buildModificationPromptArgs prepares arguments for modification prompt
func (d *DocumentationAgent) buildModificationPromptArgs(ctx PromptContext) []interface{} {
	levelStr := "##"
	if ctx.SectionLevel == 3 {
		levelStr = "###"
	}

	// Build preserve content section
	preserveSection := ""
	if ctx.PreserveContent != "" {
		preserveSection = fmt.Sprintf("PRESERVE Content (Must Include Verbatim):\n---\n%s\n---\n\n", ctx.PreserveContent)
	}

	return []interface{}{
		ctx.TargetDocFile,        // target file
		ctx.SectionTitle,         // section title
		ctx.SectionLevel,         // section level number
		ctx.Manifest.Name,        // package name
		ctx.Manifest.Title,       // package title
		ctx.Manifest.Type,        // package type
		ctx.Manifest.Version,     // package version
		ctx.Manifest.Description, // package description
		ctx.TemplateSection,      // current section content
		ctx.Changes,              // modification request
		preserveSection,          // preserve content if any
		levelStr,                 // level prefix for header instruction
		ctx.SectionTitle,         // section title for header instruction
		levelStr,                 // level prefix for final reminder
		ctx.SectionTitle,         // section title for final reminder
	}
}

// buildReformatPrompt builds a single-call reformat prompt with document content included
func (d *DocumentationAgent) buildReformatPrompt(documentContent string) string {
	// Get minimal template sections (headers only, no comments)
	minimalSections := getMinimalTemplateSections()

	// Load the prompt template
	promptContent := loadPromptFile(promptFileReformat, ReformatPrompt, d.profile)

	// Format with minimal sections and document content
	return fmt.Sprintf(promptContent, minimalSections, documentContent)
}

// Helper to create context with service info
func (d *DocumentationAgent) createPromptContext(manifest *packages.PackageManifest, changes string) PromptContext {
	return PromptContext{
		Manifest:      manifest,
		TargetDocFile: d.targetDocFile,
		Changes:       changes,
	}
}

// extractTemplateSections extracts only markdown headers and minimal structure from the template,
// removing Go template comments ({{/* ... */}}) and placeholder content.
// This reduces prompt size by ~60% while preserving the section structure.
func extractTemplateSections(fullTemplate string) string {
	// Remove multi-line Go template comments: {{/* ... */}}
	commentRegex := regexp.MustCompile(`\{\{/\*[\s\S]*?\*/\}\}`)
	stripped := commentRegex.ReplaceAllString(fullTemplate, "")

	// Remove template directives like {{- generatedHeader }}
	directiveRegex := regexp.MustCompile(`\{\{[^}]*\}\}`)
	stripped = directiveRegex.ReplaceAllString(stripped, "")

	// Process line by line to keep only headers and remove placeholder content
	lines := strings.Split(stripped, "\n")
	var result []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Keep markdown headers
		if strings.HasPrefix(trimmed, "#") {
			result = append(result, trimmed)
		}
	}

	return strings.Join(result, "\n")
}

// getMinimalTemplateSections returns a minimal section list for the reformat prompt
func getMinimalTemplateSections() string {
	fullTemplate := archetype.GetPackageDocsReadmeTemplate()
	return extractTemplateSections(fullTemplate)
}
