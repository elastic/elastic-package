// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package prompts

import (
	"os"
	"path/filepath"

	"github.com/elastic/elastic-package/internal/configuration/locations"
	"github.com/elastic/elastic-package/internal/environment"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/profile"
)

const (
	promptFileRevision             = "revision_prompt.txt"
	promptFileSectionGeneration    = "section_generation_prompt.txt"
	promptFileModificationAnalysis = "modification_analysis_prompt.txt"
	promptFileModification         = "modification_prompt.txt"
)

// Type represents the type of prompt to load
type Type int

const (
	TypeRevision Type = iota
	TypeSectionGeneration
	TypeModificationAnalysis
	TypeModification
)

// LoadFile loads a prompt file from external location if enabled, otherwise uses embedded content
func LoadFile(filename string, embeddedContent string, profile *profile.Profile) string {
	// Check if external prompt files are enabled
	envVar := environment.WithElasticPackagePrefix("LLM_EXTERNAL_PROMPTS")
	configKey := "llm.external_prompts"
	useExternal := GetConfigValue(profile, envVar, configKey, "false") == "true"

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
	logger.Warnf("External prompt file not found, using embedded content for: %s", filename)
	return embeddedContent
}

// GetConfigValue retrieves a configuration value with fallback from environment variable to profile config
func GetConfigValue(profile *profile.Profile, envVar, configKey, defaultValue string) string {
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

// Load loads a prompt by type using the embedded content with optional external override
func Load(promptType Type, p *profile.Profile) string {
	switch promptType {
	case TypeRevision:
		return LoadFile(promptFileRevision, RevisionPrompt, p)
	case TypeSectionGeneration:
		return LoadFile(promptFileSectionGeneration, SectionGenerationPrompt, p)
	case TypeModificationAnalysis:
		return LoadFile(promptFileModificationAnalysis, ModificationAnalysisPrompt, p)
	case TypeModification:
		return LoadFile(promptFileModification, ModificationPrompt, p)
	default:
		return ""
	}
}
