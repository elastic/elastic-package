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
	"time"

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

The AI agent supports three modes:
1. Rewrite mode (default): Full documentation regeneration using section-based generation
   - Analyzes your package structure, data streams, and configuration
   - Generates each section independently with its own validation loop
   - Each section is generated multiple times (configurable iterations) and the best version is selected
   - Sections are generated in parallel for faster processing
   - Creates or updates markdown files in /_dev/build/docs/
2. Modify mode: Targeted documentation changes
   - Makes specific changes to existing documentation
   - Requires existing documentation file at /_dev/build/docs/
   - Use --modify-prompt flag for non-interactive modifications
3. Evaluate mode: Documentation quality evaluation
   - Use --evaluate flag to run in evaluation mode
   - Outputs to a directory instead of modifying the package
   - Computes quality metrics (structure, accuracy, completeness, quality scores)
   - Supports batch processing of multiple packages with --batch flag

Section-based generation workflow:
The rewrite mode uses a sophisticated section-based approach where:
1. The README template is parsed into individual sections (Overview, Troubleshooting, etc.)
2. Each section is generated independently in parallel
3. Per-section validation loops run multiple iterations with feedback
4. The best iteration for each section is selected based on content quality
5. All sections are combined into the final document
6. Full-document validation is run on the assembled document

This approach produces higher quality documentation because:
- Each section gets focused attention and validation
- Issues in one section don't affect other sections
- Parallel generation is faster than sequential full-document generation
- Best-iteration tracking prevents regression in later iterations

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

Evaluation mode examples:
  # Evaluate a single package (run from package directory)
  elastic-package update documentation --evaluate --output-dir ./results

  # Batch evaluation of multiple packages
  elastic-package update documentation --evaluate \
    --batch citrix_adc,nginx,apache \
    --integrations-path ~/git/integrations \
    --output-dir ./batch_results \
    --parallel 4

  # With Phoenix tracing enabled
  elastic-package update documentation --evaluate --enable-tracing

If no LLM provider is configured, this command will print instructions for updating the documentation manually.

Configuration options for LLM providers (environment variables or profile config):
- GOOGLE_API_KEY / llm.gemini.api_key: API key for Gemini
- GEMINI_MODEL / llm.gemini.model: Model ID (defaults to gemini-3-pro-preview)
- GEMINI_THINKING_BUDGET / llm.gemini.thinking_budget: Thinking budget in tokens (defaults to 128 for "low" mode)`

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
	cmd.Println(tui.Info("  - Gemini: Set GOOGLE_API_KEY or add llm.gemini.api_key to profile config"))
	cmd.Println()
	cmd.Println(tui.Info("Profile configuration: ~/.elastic-package/profiles/<profile>/config.yml"))
}

// defaultThinkingBudget is the default "low" thinking budget for Gemini Pro models
const defaultThinkingBudget int32 = 128

// getGeminiConfig gets Gemini configuration from environment or profile
func getGeminiConfig(profile *profile.Profile) (apiKey string, modelID string, thinkingBudget *int32) {
	apiKey = getConfigValue(profile, "GOOGLE_API_KEY", "llm.gemini.api_key", "")
	modelID = getConfigValue(profile, "GEMINI_MODEL", "llm.gemini.model", tracing.DefaultModelID)

	// Get thinking budget - defaults to 128 ("low" mode) for Gemini Pro models
	budgetStr := getConfigValue(profile, "GEMINI_THINKING_BUDGET", "llm.gemini.thinking_budget", "")
	if budgetStr != "" {
		if budget, err := strconv.ParseInt(budgetStr, 10, 32); err == nil {
			b := int32(budget)
			thinkingBudget = &b
		}
	} else {
		// Default to low thinking budget
		b := defaultThinkingBudget
		thinkingBudget = &b
	}

	return apiKey, modelID, thinkingBudget
}

// getTracingConfig gets tracing configuration from environment or profile
func getTracingConfig(profile *profile.Profile) tracing.Config {
	cfg := tracing.Config{
		Enabled:     true, // Enabled by default
		Endpoint:    tracing.DefaultEndpoint,
		ProjectName: tracing.DefaultProjectName,
	}

	// Check enabled setting
	enabledStr := getConfigValue(profile, tracing.EnvTracingEnabled, "llm.tracing.enabled", "true")
	cfg.Enabled = enabledStr == "true" || enabledStr == "1"

	// Get endpoint
	if endpoint := getConfigValue(profile, tracing.EnvTracingEndpoint, "llm.tracing.endpoint", ""); endpoint != "" {
		cfg.Endpoint = endpoint
	}

	// Get API key
	cfg.APIKey = getConfigValue(profile, tracing.EnvTracingAPIKey, "llm.tracing.api_key", "")

	// Get project name
	if projectName := getConfigValue(profile, tracing.EnvTracingProjectName, "llm.tracing.project_name", ""); projectName != "" {
		cfg.ProjectName = projectName
	}

	return cfg
}

// getParallelSectionsConfig gets parallel sections configuration from environment or profile
// Returns true (parallel) by default
func getParallelSectionsConfig(profile *profile.Profile) bool {
	parallelStr := getConfigValue(profile, "ELASTIC_PACKAGE_LLM_PARALLEL_SECTIONS", "llm.parallel_sections", "true")
	return parallelStr == "true" || parallelStr == "1"
}

func updateDocumentationCommandAction(cmd *cobra.Command, args []string) error {
	// Check for evaluation mode flags early
	evaluateMode, err := cmd.Flags().GetBool("evaluate")
	if err != nil {
		return fmt.Errorf("failed to get evaluate flag: %w", err)
	}

	// Get profile for configuration access
	profile, err := cobraext.GetProfileFlag(cmd)
	if err != nil {
		return fmt.Errorf("failed to get profile: %w", err)
	}

	// Get Gemini configuration
	apiKey, modelID, thinkingBudget := getGeminiConfig(profile)

	// Check for model override from flag
	if modelFlag, _ := cmd.Flags().GetString("model"); modelFlag != "" {
		modelID = modelFlag
	}

	// Handle evaluation mode
	if evaluateMode {
		return handleEvaluationMode(cmd, profile, apiKey, modelID, thinkingBudget)
	}

	// Standard mode: require package root
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

	// Check for debug flags
	debugCriticOnly, err := cmd.Flags().GetBool("debug-critic-only")
	if err != nil {
		return fmt.Errorf("failed to get debug-critic-only flag: %w", err)
	}
	debugValidatorOnly, err := cmd.Flags().GetBool("debug-validator-only")
	if err != nil {
		return fmt.Errorf("failed to get debug-validator-only flag: %w", err)
	}
	debugGeneratorOnly, err := cmd.Flags().GetBool("debug-generator-only")
	if err != nil {
		return fmt.Errorf("failed to get debug-generator-only flag: %w", err)
	}

	// Validate mutually exclusive debug flags
	debugFlagCount := 0
	if debugCriticOnly {
		debugFlagCount++
	}
	if debugValidatorOnly {
		debugFlagCount++
	}
	if debugGeneratorOnly {
		debugFlagCount++
	}
	if debugFlagCount > 1 {
		return fmt.Errorf("only one debug flag can be used at a time: --debug-critic-only, --debug-validator-only, --debug-generator-only")
	}

	if apiKey == "" {
		printNoProviderInstructions(cmd)
		return nil
	}

	if thinkingBudget != nil {
		cmd.Printf("Using Gemini provider with model: %s (thinking budget: %d)\n", modelID, *thinkingBudget)
	} else {
		cmd.Printf("Using Gemini provider with model: %s\n", modelID)
	}

	// Select which documentation file to update
	// For debug modes, treat as non-interactive for file selection
	targetDocFile, err := selectDocumentationFile(cmd, packageRoot, nonInteractive || debugCriticOnly || debugValidatorOnly)
	if err != nil {
		return fmt.Errorf("failed to select documentation file: %w", err)
	}

	if !nonInteractive && targetDocFile != "README.md" {
		cmd.Printf("Selected documentation file: %s\n", targetDocFile)
	}

	// Find repository root for file operations
	repositoryRoot, err := files.FindRepositoryRootFrom(packageRoot)
	if err != nil {
		return fmt.Errorf("failed to find repository root: %w", err)
	}
	defer repositoryRoot.Close()

	// Get tracing configuration
	tracingConfig := getTracingConfig(profile)

	// Get parallel sections configuration
	parallelSections := getParallelSectionsConfig(profile)

	// Create the documentation agent using ADK
	docAgent, err := docagent.NewDocumentationAgent(cmd.Context(), docagent.AgentConfig{
		APIKey:           apiKey,
		ModelID:          modelID,
		PackageRoot:      packageRoot,
		RepositoryRoot:   repositoryRoot,
		DocFile:          targetDocFile,
		Profile:          profile,
		ThinkingBudget:   thinkingBudget,
		TracingConfig:    tracingConfig,
		ParallelSections: parallelSections,
	})
	if err != nil {
		return fmt.Errorf("failed to create documentation agent: %w", err)
	}

	// Ensure tracing is properly flushed before exit
	defer func() {
		if err := tracing.Shutdown(cmd.Context()); err != nil {
			cmd.PrintErrf("Warning: failed to shutdown tracing: %v\n", err)
		}
	}()

	// Handle debug modes
	if debugCriticOnly {
		cmd.Println("🔍 Running critic agent only (debug mode)...")
		return docAgent.DebugRunCriticOnly(cmd.Context(), cmd)
	}

	if debugValidatorOnly {
		cmd.Println("🔍 Running validator agent only (debug mode)...")
		return docAgent.DebugRunValidatorOnly(cmd.Context(), cmd)
	}

	// For generator-only mode, we still go through normal flow but with modified workflow config
	if debugGeneratorOnly {
		cmd.Println("🔍 Running generator agent only (debug mode - no critic/validator)...")
		return docAgent.UpdateDocumentationGeneratorOnly(cmd.Context(), nonInteractive)
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

// handleEvaluationMode handles the --evaluate flag for documentation quality evaluation
func handleEvaluationMode(cmd *cobra.Command, profile *profile.Profile, apiKey, modelID string, thinkingBudget *int32) error {
	if apiKey == "" {
		return fmt.Errorf("evaluation mode requires GOOGLE_API_KEY to be set")
	}

	// Get evaluation flags
	outputDir, err := cmd.Flags().GetString("output-dir")
	if err != nil {
		return fmt.Errorf("failed to get output-dir flag: %w", err)
	}

	batchFlag, err := cmd.Flags().GetString("batch")
	if err != nil {
		return fmt.Errorf("failed to get batch flag: %w", err)
	}

	integrationsPath, err := cmd.Flags().GetString("integrations-path")
	if err != nil {
		return fmt.Errorf("failed to get integrations-path flag: %w", err)
	}

	// Fallback to environment variable for integrations path
	if integrationsPath == "" {
		integrationsPath = os.Getenv("INTEGRATIONS_PATH")
	}

	parallelism, err := cmd.Flags().GetInt("parallel")
	if err != nil {
		return fmt.Errorf("failed to get parallel flag: %w", err)
	}

	maxIterations, err := cmd.Flags().GetUint("max-iterations")
	if err != nil {
		return fmt.Errorf("failed to get max-iterations flag: %w", err)
	}

	enableTracing, err := cmd.Flags().GetBool("enable-tracing")
	if err != nil {
		return fmt.Errorf("failed to get enable-tracing flag: %w", err)
	}

	// Get tracing configuration
	tracingConfig := getTracingConfig(profile)
	if enableTracing {
		tracingConfig.Enabled = true
	}

	if tracingConfig.Enabled {
		if err := tracing.InitWithConfig(cmd.Context(), tracingConfig); err != nil {
			cmd.Printf("Warning: failed to initialize tracing: %v\n", err)
		}
	}

	cmd.Printf("📊 Running documentation evaluation with model: %s\n", modelID)

	// Handle batch mode
	if batchFlag != "" {
		if integrationsPath == "" {
			return fmt.Errorf("--integrations-path is required for batch mode (or set INTEGRATIONS_PATH env var)")
		}

		packageNames := strings.Split(batchFlag, ",")
		for i := range packageNames {
			packageNames[i] = strings.TrimSpace(packageNames[i])
		}

		cmd.Printf("🔄 Starting batch evaluation of %d packages...\n", len(packageNames))

		batchCfg := docagent.BatchEvaluationConfig{
			IntegrationsPath: integrationsPath,
			OutputDir:        outputDir,
			PackageNames:     packageNames,
			Parallelism:      parallelism,
			APIKey:           apiKey,
			ModelID:          modelID,
			MaxIterations:    maxIterations,
			EnableTracing:    enableTracing,
			Profile:          profile,
			ThinkingBudget:   thinkingBudget,
		}

		result, err := docagent.RunBatchEvaluation(cmd.Context(), batchCfg)
		if err != nil {
			return fmt.Errorf("batch evaluation failed: %w", err)
		}

		// Print summary
		cmd.Printf("\n📊 Batch Evaluation Complete\n")
		cmd.Printf("   Total packages: %d\n", result.Summary.TotalPackages)
		cmd.Printf("   Passed: %d\n", result.Summary.PassedPackages)
		cmd.Printf("   Failed: %d\n", result.Summary.FailedPackages)
		cmd.Printf("   Average score: %.1f\n", result.Summary.AverageScore)
		cmd.Printf("   Duration: %s\n", result.Duration.Round(time.Second))
		cmd.Printf("   Results saved to: %s\n", outputDir)

		return nil
	}

	// Single package evaluation mode
	packageRoot, err := packages.FindPackageRoot()
	if err != nil {
		return fmt.Errorf("locating package root failed: %w", err)
	}

	// Create the documentation agent
	docAgent, err := docagent.NewDocumentationAgent(cmd.Context(), docagent.AgentConfig{
		APIKey:         apiKey,
		ModelID:        modelID,
		PackageRoot:    packageRoot,
		DocFile:        "README.md",
		Profile:        profile,
		ThinkingBudget: thinkingBudget,
		TracingConfig:  tracingConfig,
	})
	if err != nil {
		return fmt.Errorf("failed to create documentation agent: %w", err)
	}

	evalCfg := docagent.EvaluationConfig{
		OutputDir:     outputDir,
		MaxIterations: maxIterations,
		EnableTracing: tracingConfig.Enabled,
		ModelID:       modelID,
	}

	result, err := docAgent.EvaluateDocumentation(cmd.Context(), evalCfg)
	if err != nil {
		return fmt.Errorf("evaluation failed: %w", err)
	}

	// Print summary
	cmd.Printf("\n📊 Evaluation Complete: %s\n", result.PackageName)
	if result.Metrics != nil {
		cmd.Printf("   Composite Score: %.1f\n", result.Metrics.CompositeScore)
		cmd.Printf("   Structure Score: %.1f\n", result.Metrics.StructureScore)
		cmd.Printf("   Accuracy Score: %.1f\n", result.Metrics.AccuracyScore)
		cmd.Printf("   Completeness Score: %.1f\n", result.Metrics.CompletenessScore)
		cmd.Printf("   Quality Score: %.1f\n", result.Metrics.QualityScore)
		cmd.Printf("   Placeholder Count: %d\n", result.Metrics.PlaceholderCount)
	}
	cmd.Printf("   Approved: %v\n", result.Approved)
	cmd.Printf("   Duration: %s\n", result.Duration.Round(time.Second))
	if outputDir != "" {
		cmd.Printf("   Results saved to: %s\n", outputDir)
	}

	return nil
}
