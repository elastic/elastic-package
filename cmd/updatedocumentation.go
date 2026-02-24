// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/files"
	"github.com/elastic/elastic-package/internal/llmagent/docagent"
	"github.com/elastic/elastic-package/internal/llmagent/tracing"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/profile"
	"github.com/elastic/elastic-package/internal/tui"
)

const updateDocumentationLongDescription = `Use this command to update package documentation using an AI agent or to get manual instructions for update.

The AI agent analyzes your package structure, data streams, and configuration, and generates a new documentation file based on the template and the package context.

For packages with multiple documentation files, use the --doc-file flag to specify the file to update (defaults to README.md).

If no LLM provider is configured, this command will print instructions for updating the documentation manually.

Configuration options for LLM providers (environment variables or profile config):
- LLM_PROVIDER / llm.provider: Provider name (only Gemini provider currently supported).
- Gemini: GOOGLE_API_KEY / llm.gemini.api_key, GEMINI_MODEL / llm.gemini.model, GEMINI_THINKING_BUDGET / llm.gemini.thinking_budget`

// getConfigValue retrieves a configuration value with fallback from environment variable to profile config
func getConfigValue(profile *profile.Profile, envVar, configKey, defaultValue string) string {
	if envValue := os.Getenv(envVar); envValue != "" {
		return envValue
	}
	if profile != nil {
		return profile.Config(configKey, defaultValue)
	}
	return defaultValue
}

// discoverDocumentationTemplates finds all .md files in _dev/build/docs/
func discoverDocumentationTemplates(packageRoot string) ([]string, error) {
	docsDir := filepath.Join(packageRoot, "_dev", "build", "docs")

	entries, err := os.ReadDir(docsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{"README.md"}, nil
		}
		return nil, fmt.Errorf("failed to read docs directory: %w", err)
	}

	var mdFiles []string
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".md" {
			mdFiles = append(mdFiles, entry.Name())
		}
	}

	if len(mdFiles) == 0 {
		return []string{"README.md"}, nil
	}

	sort.Slice(mdFiles, func(i, j int) bool {
		if mdFiles[i] == "README.md" {
			return true
		}
		if mdFiles[j] == "README.md" {
			return false
		}
		return mdFiles[i] < mdFiles[j]
	})

	return mdFiles, nil
}

// selectDocumentationFile determines which documentation file to update
func selectDocumentationFile(cmd *cobra.Command, packageRoot string) (string, error) {
	docFile, err := cmd.Flags().GetString("doc-file")
	if err != nil {
		return "", fmt.Errorf("failed to get doc-file flag: %w", err)
	}

	if docFile != "" {
		if filepath.Ext(docFile) != ".md" {
			return "", fmt.Errorf("doc-file must be a .md file, got: %s", docFile)
		}
		if filepath.Base(docFile) != docFile {
			return "", fmt.Errorf("doc-file must be a filename only (no path), got: %s", docFile)
		}
		return docFile, nil
	}

	mdFiles, err := discoverDocumentationTemplates(packageRoot)
	if err != nil {
		return "", err
	}

	if len(mdFiles) == 1 {
		return mdFiles[0], nil
	}

	cmd.Println(tui.Warning(fmt.Sprintf(
		"Multiple documentation files found (%s). Using README.md by default. Use --doc-file to specify a different file.",
		strings.Join(mdFiles, ", "),
	)))
	return "README.md", nil
}

// printNoProviderInstructions displays instructions when no LLM provider is configured
func printNoProviderInstructions(cmd *cobra.Command) {
	cmd.Println(tui.Warning("AI agent is not available (no LLM provider API key set)."))
	cmd.Println()
	cmd.Println(tui.Info("To update the documentation manually:"))
	cmd.Println(tui.Info("  1. Edit markdown files in `_dev/build/docs/` (e.g., README.md). Please follow the documentation guidelines from https://www.elastic.co/docs/extend/integrations/documentation-guidelines."))
	cmd.Println(tui.Info("  2. Run `elastic-package build`"))
	cmd.Println()
	cmd.Println(tui.Info("For AI-powered documentation updates, configure an LLM provider (Gemini only provider currently supported):"))
	cmd.Println(tui.Info("  - Set llm.provider in your profile or LLM_PROVIDER env var"))
	cmd.Println(tui.Info("  - For Gemini: set GOOGLE_API_KEY or llm.gemini.api_key in your elastic-package profile config"))
}

const (
	defaultModelID              = "gemini-3-flash-preview"
	defaultThinkingBudget int32 = 128
	defaultProvider             = "gemini"
)

// getProvider returns the LLM provider name from environment or profile.
func getProvider(profile *profile.Profile) string {
	p := getConfigValue(profile, "LLM_PROVIDER", "llm.provider", defaultProvider)
	if p == "" {
		return defaultProvider
	}
	return strings.ToLower(p)
}

// getLLMConfig returns provider, api key, model ID, and optional thinking budget.
func getLLMConfig(profile *profile.Profile) (provider, apiKey, modelID string, thinkingBudget *int32) {
	provider = getProvider(profile)
	if provider == "" {
		provider = defaultProvider
	}
	if provider != defaultProvider {
		apiKey = getConfigValue(profile, "", "llm."+provider+".api_key", "")
		modelID = getConfigValue(profile, "", "llm."+provider+".model", "")
		return provider, apiKey, modelID, nil
	}
	apiKey = getConfigValue(profile, "GOOGLE_API_KEY", "llm.gemini.api_key", "")
	modelID = getConfigValue(profile, "GEMINI_MODEL", "llm.gemini.model", defaultModelID)
	b := defaultThinkingBudget
	if budgetStr := getConfigValue(profile, "GEMINI_THINKING_BUDGET", "llm.gemini.thinking_budget", ""); budgetStr != "" {
		if budget, err := strconv.ParseInt(budgetStr, 10, 32); err == nil {
			b = int32(budget)
		}
	}
	thinkingBudget = &b
	return provider, apiKey, modelID, thinkingBudget
}

// getTracingConfig gets tracing configuration from profile config (llm.tracing.*) only.
func getTracingConfig(profile *profile.Profile) tracing.Config {
	cfg := tracing.Config{
		Enabled:     false,
		Endpoint:    tracing.DefaultEndpoint,
		ProjectName: tracing.DefaultProjectName,
	}
	if profile == nil {
		return cfg
	}
	enabledStr := profile.Config("llm.tracing.enabled", "false")
	cfg.Enabled = enabledStr == "true" || enabledStr == "1"
	if endpoint := profile.Config("llm.tracing.endpoint", ""); endpoint != "" {
		cfg.Endpoint = endpoint
	}
	cfg.APIKey = profile.Config("llm.tracing.api_key", "")
	if projectName := profile.Config("llm.tracing.project_name", ""); projectName != "" {
		cfg.ProjectName = projectName
	}
	return cfg
}

func updateDocumentationCommandAction(cmd *cobra.Command, _ []string) error {
	profile, err := cobraext.GetProfileFlag(cmd)
	if err != nil {
		return fmt.Errorf("failed to get profile: %w", err)
	}

	provider, apiKey, modelID, thinkingBudget := getLLMConfig(profile)
	return handleStandardMode(cmd, profile, provider, apiKey, modelID, thinkingBudget)
}

// handleStandardMode handles the standard documentation update workflow
func handleStandardMode(cmd *cobra.Command, profile *profile.Profile, provider, apiKey, modelID string, thinkingBudget *int32) error {
	packageRoot, err := packages.FindPackageRoot()
	if err != nil {
		return fmt.Errorf("locating package root failed: %w", err)
	}

	if apiKey == "" {
		printNoProviderInstructions(cmd)
		return nil
	}

	if thinkingBudget != nil {
		cmd.Printf("Using LLM model \"%s\" (thinking budget: %d)\n", modelID, *thinkingBudget)
	} else {
		cmd.Printf("Using LLM model \"%s\"\n", modelID)
	}

	targetDocFile, err := selectDocumentationFile(cmd, packageRoot)
	if err != nil {
		return fmt.Errorf("failed to select documentation file: %w", err)
	}

	repositoryRoot, err := files.FindRepositoryRootFrom(packageRoot)
	if err != nil {
		return fmt.Errorf("failed to find repository root: %w", err)
	}
	defer repositoryRoot.Close()

	tracingConfig := getTracingConfig(profile)

	docAgent, err := docagent.NewDocumentationAgent(cmd.Context(), docagent.AgentConfig{
		Provider:       provider,
		APIKey:         apiKey,
		ModelID:        modelID,
		PackageRoot:    packageRoot,
		RepositoryRoot: repositoryRoot,
		DocFile:        targetDocFile,
		Profile:        profile,
		ThinkingBudget: thinkingBudget,
		TracingConfig:  tracingConfig,
	})
	if err != nil {
		return fmt.Errorf("failed to create documentation agent: %w", err)
	}

	defer func() {
		if err := tracing.Shutdown(cmd.Context()); err != nil {
			cmd.PrintErrf("Warning: failed to shutdown tracing: %v\n", err)
		}
	}()

	cmd.Println("Updating documentation using AI agent...")
	if err := docAgent.UpdateDocumentation(cmd.Context(), true); err != nil {
		return fmt.Errorf("documentation update failed: %w", err)
	}
	return nil
}
