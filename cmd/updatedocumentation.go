// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/files"
	llmconfig "github.com/elastic/elastic-package/internal/llmagent/config"
	"github.com/elastic/elastic-package/internal/llmagent/docagent"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/profile"
	"github.com/elastic/elastic-package/internal/tui"
)

const updateDocumentationLongDescription = `Use this command to update package documentation using an AI agent or to get manual instructions for update.

The AI agent supports two modes:
1. Rewrite mode (default): Full documentation regeneration
   - Analyzes your package structure, data streams, and configuration, and generates a new documentation file based on the template and the package context.
2. Modify mode: Targeted documentation changes
   - The LLM will perform a targeted change to the documentation, based on user-provided instructions.
   - Use --modify-prompt flag to provide instructions for non-interactive modifications

For packages with multiple documentation files, the user can specify which file to update in interactive mode, or use the --doc-file flag to specify the file to update in non-interactive mode.

If no LLM provider is configured, this command will print instructions for updating the documentation manually.

Configuration options for LLM providers (environment variables or profile config):
- ELASTIC_PACKAGE_LLM_PROVIDER / llm.provider: Provider name (only Gemini provider currently supported).
- Gemini: GOOGLE_API_KEY / llm.gemini.api_key, GEMINI_MODEL / llm.gemini.model, GEMINI_THINKING_BUDGET / llm.gemini.thinking_budget`

const (
	modePromptRewrite = "Rewrite (full regeneration)"
	modePromptModify  = "Modify (targeted changes)"
)


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

	// If flag is provided, validate its an .md file and use it
	if docFile != "" {
		if filepath.Ext(docFile) != ".md" {
			return "", fmt.Errorf("doc-file must be a .md file, got: %s", docFile)
		}
		if filepath.Base(docFile) != docFile {
			return "", fmt.Errorf("doc-file must be a filename only (no path), got: %s", docFile)
		}
		return docFile, nil
	}

	// Discover available markdown files
	mdFiles, err := discoverDocumentationTemplates(packageRoot)
	if err != nil {
		return "", err
	}

	// If only one file, use it (no prompt)
	if len(mdFiles) == 1 {
		return mdFiles[0], nil
	}

	// Non-interactive mode: warn when multiple files exist and default to README.md
	if nonInteractive {
		cmd.Println(tui.Warning(fmt.Sprintf(
			"Multiple documentation files found (%s). Using README.md by default. Use --doc-file to specify a different file.",
			strings.Join(mdFiles, ", "),
		)))
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
	cmd.Println(tui.Info("For AI-powered documentation updates, configure an LLM provider (Gemini only provider currently supported):"))
	cmd.Println(tui.Info("  - Set llm.provider in your profile or ELASTIC_PACKAGE_LLM_PROVIDER env var"))
	cmd.Println(tui.Info("  - For Gemini: set GOOGLE_API_KEY or llm.gemini.api_key in your elastic-package profile config"))
}

// standardModeFlags holds CLI flags for standard (non-evaluation) mode
type standardModeFlags struct {
	nonInteractive bool
	modifyPrompt   string
}

// getStandardModeFlags extracts CLI flags needed for standard mode
func getStandardModeFlags(cmd *cobra.Command) (standardModeFlags, error) {
	var flags standardModeFlags
	var err error

	flags.nonInteractive, err = cmd.Flags().GetBool("non-interactive")
	if err != nil {
		return flags, fmt.Errorf("failed to get non-interactive flag: %w", err)
	}

	flags.modifyPrompt, err = cmd.Flags().GetString("modify-prompt")
	if err != nil {
		return flags, fmt.Errorf("failed to get modify-prompt flag: %w", err)
	}

	return flags, nil
}

// selectUpdateMode prompts the user to select rewrite or modify mode, or determines it automatically
func selectUpdateMode(cmd *cobra.Command, nonInteractive bool, modifyPrompt string) (useModifyMode bool, err error) {
	cmd.Println("Updating documentation using AI agent...")

	if nonInteractive || modifyPrompt != "" {
		return modifyPrompt != "", nil
	}

	modePrompt := tui.NewSelect("Do you want to rewrite or modify the documentation?", []string{
		modePromptRewrite,
		modePromptModify,
	}, modePromptRewrite)

	var mode string
	if err := tui.AskOne(modePrompt, &mode); err != nil {
		return false, fmt.Errorf("prompt failed: %w", err)
	}

	return mode == modePromptModify, nil
}

// runDocumentationUpdate executes the documentation update using the agent
func runDocumentationUpdate(cmd *cobra.Command, docAgent *docagent.DocumentationAgent, useModifyMode, nonInteractive bool, modifyPrompt string) error {
	if useModifyMode {
		if err := docAgent.ModifyDocumentation(cmd.Context(), nonInteractive, modifyPrompt); err != nil {
			return fmt.Errorf("documentation modification failed: %w", err)
		}
	} else {
		if err := docAgent.UpdateDocumentation(cmd.Context(), nonInteractive); err != nil {
			return fmt.Errorf("documentation update failed: %w", err)
		}
	}
	return nil
}


func updateDocumentationCommandAction(cmd *cobra.Command, _ []string) error {
	p, err := cobraext.GetProfileFlag(cmd)
	if err != nil {
		return fmt.Errorf("failed to get profile: %w", err)
	}

	cfg := llmconfig.Load(p)
	return handleStandardMode(cmd, p, cfg)
}

// handleStandardMode handles the standard documentation update workflow
func handleStandardMode(cmd *cobra.Command, p *profile.Profile, cfg llmconfig.LLMConfig) error {
	packageRoot, err := packages.FindPackageRoot()
	if err != nil {
		return fmt.Errorf("locating package root failed: %w", err)
	}

	flags, err := getStandardModeFlags(cmd)
	if err != nil {
		return err
	}

	if cfg.APIKey == "" {
		printNoProviderInstructions(cmd)
		return nil
	}

	if cfg.ThinkingBudget != nil {
		cmd.Printf("Using LLM model \"%s\" (thinking budget: %d)\n", cfg.ModelID, *cfg.ThinkingBudget)
	} else {
		cmd.Printf("Using LLM model \"%s\"\n", cfg.ModelID)
	}

	targetDocFile, err := selectDocumentationFile(cmd, packageRoot, flags.nonInteractive)
	if err != nil {
		return fmt.Errorf("failed to select documentation file: %w", err)
	}
	if !flags.nonInteractive && targetDocFile != "README.md" {
		cmd.Printf("Selected documentation file: %s\n", targetDocFile)
	}

	repositoryRoot, err := files.FindRepositoryRootFrom(packageRoot)
	if err != nil {
		return fmt.Errorf("failed to find repository root: %w", err)
	}
	defer repositoryRoot.Close()

	tracingConfig := llmconfig.TracingConfig(p)

	docAgent, err := docagent.NewDocumentationAgent(cmd.Context(), docagent.AgentConfig{
		Provider:       cfg.Provider,
		APIKey:         cfg.APIKey,
		ModelID:        cfg.ModelID,
		PackageRoot:    packageRoot,
		RepositoryRoot: repositoryRoot,
		DocFile:        targetDocFile,
		Profile:        p,
		ThinkingBudget: cfg.ThinkingBudget,
		TracingConfig:  tracingConfig,
	})
	if err != nil {
		return fmt.Errorf("failed to create documentation agent: %w", err)
	}

	useModifyMode, err := selectUpdateMode(cmd, flags.nonInteractive, flags.modifyPrompt)
	if err != nil {
		return err
	}

	return runDocumentationUpdate(cmd, docAgent, useModifyMode, flags.nonInteractive, flags.modifyPrompt)
}

