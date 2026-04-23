// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

// Package config provides helpers for reading LLM provider configuration
// from environment variables and the elastic-package profile.
package config

import (
	"os"
	"strconv"
	"strings"

	"github.com/elastic/elastic-package/internal/environment"
	"github.com/elastic/elastic-package/internal/llmagent/tracing"
	"github.com/elastic/elastic-package/internal/profile"
)

const (
	DefaultModelID              = "gemini-3-flash-preview"
	DefaultThinkingBudget int32 = 128
	DefaultProvider             = "gemini"
)

// LLMConfig holds all configuration needed to initialise an LLM provider.
type LLMConfig struct {
	Provider       string
	APIKey         string
	ModelID        string
	ThinkingBudget *int32
}

// llmProviderEnvVar is the elastic-package–prefixed env var for selecting the LLM provider.
var llmProviderEnvVar = environment.WithElasticPackagePrefix("LLM_PROVIDER")

// configValue retrieves a value from an environment variable first, then from
// the profile config, falling back to defaultValue.
func configValue(p *profile.Profile, envVar, configKey, defaultValue string) string {
	if envVar != "" {
		if v := os.Getenv(envVar); v != "" {
			return v
		}
	}
	if p != nil {
		return p.Config(configKey, defaultValue)
	}
	return defaultValue
}

// Provider returns the LLM provider name, lower-cased, from the environment or profile.
func Provider(p *profile.Profile) string {
	v := configValue(p, llmProviderEnvVar, "llm.provider", DefaultProvider)
	if v == "" {
		return DefaultProvider
	}
	return strings.ToLower(v)
}

// Load builds an LLMConfig from environment variables and the profile.
func Load(p *profile.Profile) LLMConfig {
	provider := Provider(p)

	if provider != DefaultProvider {
		var apiKey, modelID string
		if p != nil {
			apiKey = p.Config("llm."+provider+".api_key", "")
			modelID = p.Config("llm."+provider+".model", "")
		}
		return LLMConfig{Provider: provider, APIKey: apiKey, ModelID: modelID}
	}

	apiKey := configValue(p, "GOOGLE_API_KEY", "llm.gemini.api_key", "")
	modelID := configValue(p, "GEMINI_MODEL", "llm.gemini.model", DefaultModelID)

	b := DefaultThinkingBudget
	if budgetStr := configValue(p, "GEMINI_THINKING_BUDGET", "llm.gemini.thinking_budget", ""); budgetStr != "" {
		if budget, err := strconv.ParseInt(budgetStr, 10, 32); err == nil {
			b = int32(budget)
		}
	}
	return LLMConfig{
		Provider:       provider,
		APIKey:         apiKey,
		ModelID:        modelID,
		ThinkingBudget: &b,
	}
}

// TracingConfig reads LLM tracing settings from the profile.
func TracingConfig(p *profile.Profile) tracing.Config {
	cfg := tracing.Config{
		Enabled:     false,
		Endpoint:    tracing.DefaultEndpoint,
		ProjectName: tracing.DefaultProjectName,
	}
	if p == nil {
		return cfg
	}
	enabledStr := p.Config("llm.tracing.enabled", "false")
	cfg.Enabled = enabledStr == "true" || enabledStr == "1"
	if endpoint := p.Config("llm.tracing.endpoint", ""); endpoint != "" {
		cfg.Endpoint = endpoint
	}
	cfg.APIKey = p.Config("llm.tracing.api_key", "")
	if projectName := p.Config("llm.tracing.project_name", ""); projectName != "" {
		cfg.ProjectName = projectName
	}
	return cfg
}
