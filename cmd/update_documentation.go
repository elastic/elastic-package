// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/llmagent/docagent"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/profile"
	"github.com/elastic/elastic-package/internal/tui"
)

const updateDocumentationLongDescription = `Use this command to update package documentation using an AI agent or to get manual instructions for update.

The AI agent supports two modes:
1. Rewrite mode (default): Full documentation regeneration
   - Analyzes your package structure, data streams, and configuration
   - Generates comprehensive documentation following Elastic's templates
   - Creates or updates markdown files in /_dev/build/docs/
2. Modify mode: Targeted documentation changes
   - Makes specific changes to existing documentation
   - Requires existing documentation file at /_dev/build/docs/
   - Use --modify-prompt flag for non-interactive modifications

Multi-file support:
   - Use --doc-file to specify which markdown file to update (defaults to README.md)
   - In interactive mode, you'll be prompted to select from available files
   - Supports packages with multiple documentation files (e.g., README.md, vpc.md, etc.)

Interactive workflow:
After confirming you want to use the AI agent, you'll choose between rewrite or modify mode.
You can review results and request additional changes iteratively.

Non-interactive mode:
Use --non-interactive to skip all prompts and automatically accept the first result from the LLM.
Combine with --modify-prompt "instructions" for targeted non-interactive changes.

If no LLM provider is configured, this command will print instructions for updating the documentation manually.

Configuration options for LLM providers (environment variables or profile config):
- GEMINI_API_KEY / llm.gemini.api_key: API key for Gemini
- GEMINI_MODEL / llm.gemini.model: Model ID (defaults to gemini-2.5-pro)`

const (
	modePromptRewrite = "Rewrite (full regeneration)"
	modePromptModify  = "Modify (targeted changes)"
)

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

// discoverDocumentationFiles finds all .md files in _dev/build/docs/
func discoverDocumentationFiles(packageRoot string) ([]string, error) {
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

	// If no files found, return README.md as default
	if len(mdFiles) == 0 {
		return []string{"README.md"}, nil
	}

	// Sort with README.md first, others alphabetically
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
func selectDocumentationFile(cmd *cobra.Command, packageRoot string, nonInteractive bool) (string, error) {
	// Check if --doc-file flag was provided
	docFile, err := cmd.Flags().GetString("doc-file")
	if err != nil {
		return "", fmt.Errorf("failed to get doc-file flag: %w", err)
	}

	// If flag is provided, validate and use it
	if docFile != "" {
		// Validate it's a .md file
		if filepath.Ext(docFile) != ".md" {
			return "", fmt.Errorf("doc-file must be a .md file, got: %s", docFile)
		}
		// Validate it's just a filename, not a path
		if filepath.Base(docFile) != docFile {
			return "", fmt.Errorf("doc-file must be a filename only (no path), got: %s", docFile)
		}
		return docFile, nil
	}

	// Discover available markdown files
	mdFiles, err := discoverDocumentationFiles(packageRoot)
	if err != nil {
		return "", err
	}

	// If only one file or non-interactive mode, use README.md (default)
	if len(mdFiles) == 1 || nonInteractive {
		return "README.md", nil
	}

	// Interactive mode with multiple files: prompt user to select
	selectPrompt := tui.NewSelect("Which documentation file would you like to update?", mdFiles, "README.md")

	var selectedFile string
	err = tui.AskOne(selectPrompt, &selectedFile)
	if err != nil {
		return "", fmt.Errorf("file selection failed: %w", err)
	}

	return selectedFile, nil
}

// printNoProviderInstructions displays instructions when no LLM provider is configured
func printNoProviderInstructions(cmd *cobra.Command) {
	cmd.Println(tui.Warning("AI agent is not available (no LLM provider API key set)."))
	cmd.Println()
	cmd.Println(tui.Info("To update the documentation manually:"))
	cmd.Println(tui.Info("  1. Edit markdown files in `_dev/build/docs/` (e.g., README.md). Please follow the documentation guidelines from https://www.elastic.co/docs/extend/integrations/documentation-guidelines."))
	cmd.Println(tui.Info("  2. Run `elastic-package build`"))
	cmd.Println()
	cmd.Println(tui.Info("For AI-powered documentation updates, configure Gemini:"))
	cmd.Println(tui.Info("  - Gemini: Set GEMINI_API_KEY or add llm.gemini.api_key to profile config"))
	cmd.Println()
	cmd.Println(tui.Info("Profile configuration: ~/.elastic-package/profiles/<profile>/config.yml"))
}

// getGeminiConfig gets Gemini configuration from environment or profile
func getGeminiConfig(profile *profile.Profile) (apiKey string, modelID string) {
	apiKey = getConfigValue(profile, "GEMINI_API_KEY", "llm.gemini.api_key", "")
	modelID = getConfigValue(profile, "GEMINI_MODEL", "llm.gemini.model", "gemini-2.5-pro")
	return apiKey, modelID
}

func updateDocumentationCommandAction(cmd *cobra.Command, args []string) error {
	packageRoot, err := packages.FindPackageRoot()
	if err != nil {
		return fmt.Errorf("locating package root failed: %w", err)
	}

	// Check for non-interactive flag
	nonInteractive, err := cmd.Flags().GetBool("non-interactive")
	if err != nil {
		return fmt.Errorf("failed to get non-interactive flag: %w", err)
	}

	// Check for modify-prompt flag
	modifyPrompt, err := cmd.Flags().GetString("modify-prompt")
	if err != nil {
		return fmt.Errorf("failed to get modify-prompt flag: %w", err)
	}

	// Get profile for configuration access
	profile, err := cobraext.GetProfileFlag(cmd)
	if err != nil {
		return fmt.Errorf("failed to get profile: %w", err)
	}

	// Get Gemini configuration
	apiKey, modelID := getGeminiConfig(profile)

	if apiKey == "" {
		printNoProviderInstructions(cmd)
		return nil
	}

	cmd.Printf("Using Gemini provider with model: %s\n", modelID)

	// Select which documentation file to update
	targetDocFile, err := selectDocumentationFile(cmd, packageRoot, nonInteractive)
	if err != nil {
		return fmt.Errorf("failed to select documentation file: %w", err)
	}

	if !nonInteractive && targetDocFile != "README.md" {
		cmd.Printf("Selected documentation file: %s\n", targetDocFile)
	}

	// Determine the mode based on user input
	var useModifyMode bool

	// Skip confirmation prompt in non-interactive mode
	if !nonInteractive {
		// Prompt user for confirmation
		confirmPrompt := tui.NewConfirm("Do you want to update the documentation using the AI agent?", true)

		var confirm bool
		err = tui.AskOne(confirmPrompt, &confirm, tui.Required)
		if err != nil {
			return fmt.Errorf("prompt failed: %w", err)
		}

		if !confirm {
			cmd.Println("Documentation update cancelled.")
			return nil
		}

		// If no modify-prompt flag was provided, ask user to choose mode
		if modifyPrompt == "" {
			modePrompt := tui.NewSelect("Do you want to rewrite or modify the documentation?", []string{
				modePromptRewrite,
				modePromptModify,
			}, modePromptRewrite)

			var mode string
			err = tui.AskOne(modePrompt, &mode)
			if err != nil {
				return fmt.Errorf("prompt failed: %w", err)
			}

			useModifyMode = mode == "Modify (targeted changes)"
		} else {
			useModifyMode = true
		}
	} else {
		cmd.Println("Running in non-interactive mode - proceeding automatically.")
		useModifyMode = modifyPrompt != ""
	}

	// Create the documentation agent using ADK
	docAgent, err := docagent.NewDocumentationAgent(cmd.Context(), docagent.AgentConfig{
		APIKey:      apiKey,
		ModelID:     modelID,
		PackageRoot: packageRoot,
		DocFile:     targetDocFile,
		Profile:     profile,
	})
	if err != nil {
		return fmt.Errorf("failed to create documentation agent: %w", err)
	}

	// Run the documentation update process based on selected mode
	if useModifyMode {
		err = docAgent.ModifyDocumentation(cmd.Context(), nonInteractive, modifyPrompt)
		if err != nil {
			return fmt.Errorf("documentation modification failed: %w", err)
		}
	} else {
		err = docAgent.UpdateDocumentation(cmd.Context(), nonInteractive)
		if err != nil {
			return fmt.Errorf("documentation update failed: %w", err)
		}
	}

	cmd.Println("Done")
	return nil
}
