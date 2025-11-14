// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package docagent

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/elastic/elastic-package/internal/configuration/locations"
	"github.com/elastic/elastic-package/internal/environment"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/profile"
)

const (
	promptFileInitial  = "initial_prompt.txt"
	promptFileRevision = "revision_prompt.txt"
	promptFileLimitHit = "limit_hit_prompt.txt"
)

type PromptType int

const (
	PromptTypeInitial PromptType = iota
	PromptTypeRevision
	PromptTypeSectionBased
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
	case PromptTypeSectionBased:
		promptFile = promptFileLimitHit
		embeddedContent = LimitHitPrompt
		formatArgs = d.buildSectionBasedPromptArgs(ctx)
	}

	promptContent := loadPromptFile(promptFile, embeddedContent, d.profile)
	basePrompt := fmt.Sprintf(promptContent, formatArgs...)

	// Append service info if available
	if ctx.HasServiceInfo {
		basePrompt += fmt.Sprintf(
			"\n\nKNOWLEDGE BASE - SERVICE INFORMATION (SOURCE OF TRUTH):"+
				"\nThe following information is from docs/knowledge_base/service_info.md and should be treated as the authoritative source."+
				"\nIf you find conflicting information from other sources (web search, etc.), prefer the information below."+
				"\n\n---\n%s\n---\n",
			ctx.ServiceInfo)
	}

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

// buildSectionBasedPromptArgs prepares arguments for section-based prompt
func (d *DocumentationAgent) buildSectionBasedPromptArgs(ctx PromptContext) []interface{} {
	return []interface{}{
		ctx.TargetDocFile, // task description
		ctx.TargetDocFile, // target documentation file label
		ctx.Manifest.Name,
		ctx.Manifest.Title,
		ctx.Manifest.Type,
		ctx.Manifest.Version,
		ctx.Manifest.Description,
		ctx.TargetDocFile, // write_file tool description
		ctx.TargetDocFile, // step 2 - read current file
	}
}

// Helper to create context with service info
func (d *DocumentationAgent) createPromptContext(manifest *packages.PackageManifest, changes string) PromptContext {
	serviceInfo, hasServiceInfo := d.readServiceInfo()
	return PromptContext{
		Manifest:       manifest,
		TargetDocFile:  d.targetDocFile,
		Changes:        changes,
		ServiceInfo:    serviceInfo,
		HasServiceInfo: hasServiceInfo,
	}
}
