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
   - Supports batch processing of multiple packages with --evaluate-batch flag

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
  elastic-package update documentation --evaluate --evaluate-output-dir ./results

  # Batch evaluation of multiple packages
  elastic-package update documentation --evaluate \
    --evaluate-batch citrix_adc,nginx,apache \
    --evaluate-integrations-path ~/git/integrations \
    --evaluate-output-dir ./batch_results \
    --evaluate-parallel 4

If no LLM provider is configured, this command will print instructions for updating the documentation manually.

Configuration options for LLM providers (environment variables or profile config):
- GOOGLE_API_KEY / llm.gemini.api_key: API key for Gemini
- GEMINI_MODEL / llm.gemini.model: Model ID (defaults to gemini-3-flash-preview)
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
	cmd.Println(tui.Info("For AI-powered documentation updates, configure Gemini:"))
	cmd.Println(tui.Info("  - Gemini: Set GOOGLE_API_KEY or add llm.gemini.api_key to elastic-packageprofile config"))
}

const (
	defaultModelID              = "gemini-3-flash-preview"
	defaultThinkingBudget int32 = 128
)

// getGeminiConfig gets Gemini configuration from environment or profile
func getGeminiConfig(profile *profile.Profile) (apiKey string, modelID string, thinkingBudget *int32) {
	apiKey = getConfigValue(profile, "GOOGLE_API_KEY", "llm.gemini.api_key", "")
	modelID = getConfigValue(profile, "GEMINI_MODEL", "llm.gemini.model", defaultModelID)

	b := defaultThinkingBudget
	if budgetStr := getConfigValue(profile, "GEMINI_THINKING_BUDGET", "llm.gemini.thinking_budget", ""); budgetStr != "" {
		if budget, err := strconv.ParseInt(budgetStr, 10, 32); err == nil {
			b = int32(budget)
		}
	}
	thinkingBudget = &b

	return apiKey, modelID, thinkingBudget
}

// getTracingConfig gets tracing configuration from environment or profile
func getTracingConfig(profile *profile.Profile) tracing.Config {
	cfg := tracing.Config{
		Enabled:     false,
		Endpoint:    tracing.DefaultEndpoint,
		ProjectName: tracing.DefaultProjectName,
	}

	// Check enabled setting
	enabledStr := getConfigValue(profile, tracing.EnvTracingEnabled, "llm.tracing.enabled", "false")
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

	// Ask user to choose mode
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
	cmd.Println("Done")
	return nil
}

func updateDocumentationCommandAction(cmd *cobra.Command, args []string) error {
	evaluateMode, err := cmd.Flags().GetBool("evaluate")
	if err != nil {
		return fmt.Errorf("failed to get evaluate flag: %w", err)
	}

	profile, err := cobraext.GetProfileFlag(cmd)
	if err != nil {
		return fmt.Errorf("failed to get profile: %w", err)
	}

	apiKey, modelID, thinkingBudget := getGeminiConfig(profile)

	if evaluateMode {
		return handleEvaluationMode(cmd, profile, apiKey, modelID, thinkingBudget)
	}

	return handleStandardMode(cmd, profile, apiKey, modelID, thinkingBudget)
}

// handleStandardMode handles the standard documentation update workflow
func handleStandardMode(cmd *cobra.Command, profile *profile.Profile, apiKey, modelID string, thinkingBudget *int32) error {
	packageRoot, err := packages.FindPackageRoot()
	if err != nil {
		return fmt.Errorf("locating package root failed: %w", err)
	}

	flags, err := getStandardModeFlags(cmd)
	if err != nil {
		return err
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

	tracingConfig := getTracingConfig(profile)

	docAgent, err := docagent.NewDocumentationAgent(cmd.Context(), docagent.AgentConfig{
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

	useModifyMode, err := selectUpdateMode(cmd, flags.nonInteractive, flags.modifyPrompt)
	if err != nil {
		return err
	}

	return runDocumentationUpdate(cmd, docAgent, useModifyMode, flags.nonInteractive, flags.modifyPrompt)
}

// evaluationModeFlags holds CLI flags for evaluation mode
type evaluationModeFlags struct {
	outputDir        string
	batchFlag        string
	integrationsPath string
	parallelism      int
	maxIterations    uint
}

// getEvaluationModeFlags extracts CLI flags needed for evaluation mode
func getEvaluationModeFlags(cmd *cobra.Command) (evaluationModeFlags, error) {
	var flags evaluationModeFlags
	var err error

	flags.outputDir, err = cmd.Flags().GetString("output-dir")
	if err != nil {
		return flags, fmt.Errorf("failed to get output-dir flag: %w", err)
	}

	flags.batchFlag, err = cmd.Flags().GetString("batch")
	if err != nil {
		return flags, fmt.Errorf("failed to get batch flag: %w", err)
	}

	flags.integrationsPath, err = cmd.Flags().GetString("integrations-path")
	if err != nil {
		return flags, fmt.Errorf("failed to get integrations-path flag: %w", err)
	}
	if flags.integrationsPath == "" {
		flags.integrationsPath = os.Getenv("INTEGRATIONS_PATH")
	}

	flags.parallelism, err = cmd.Flags().GetInt("parallel")
	if err != nil {
		return flags, fmt.Errorf("failed to get parallel flag: %w", err)
	}

	flags.maxIterations, err = cmd.Flags().GetUint("max-iterations")
	if err != nil {
		return flags, fmt.Errorf("failed to get max-iterations flag: %w", err)
	}

	return flags, nil
}

// printBatchEvaluationSummary prints the summary for batch evaluation results
func printBatchEvaluationSummary(cmd *cobra.Command, result *docagent.BatchEvaluationResult, outputDir string) {
	cmd.Printf("\n📊 Batch Evaluation Complete\n")
	cmd.Printf("   Total packages: %d\n", result.Summary.TotalPackages)
	cmd.Printf("   Passed: %d\n", result.Summary.PassedPackages)
	cmd.Printf("   Failed: %d\n", result.Summary.FailedPackages)
	cmd.Printf("   Average score: %.1f\n", result.Summary.AverageScore)
	cmd.Printf("   Duration: %s\n", result.Duration.Round(time.Second))
	cmd.Printf("   Results saved to: %s\n", outputDir)
}

// printSingleEvaluationSummary prints the summary for single package evaluation results
func printSingleEvaluationSummary(cmd *cobra.Command, result *docagent.EvaluationResult, outputDir string) {
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
}

// runBatchEvaluation executes batch evaluation for multiple packages
func runBatchEvaluation(cmd *cobra.Command, flags evaluationModeFlags, profile *profile.Profile, apiKey, modelID string, thinkingBudget *int32, tracingEnabled bool) error {
	if flags.integrationsPath == "" {
		return fmt.Errorf("--evaluate-integrations-path is required for batch mode (or set INTEGRATIONS_PATH env var)")
	}

	packageNames := strings.Split(flags.batchFlag, ",")
	for i := range packageNames {
		packageNames[i] = strings.TrimSpace(packageNames[i])
	}

	cmd.Printf("🔄 Starting batch evaluation of %d packages...\n", len(packageNames))

	batchCfg := docagent.BatchEvaluationConfig{
		IntegrationsPath: flags.integrationsPath,
		OutputDir:        flags.outputDir,
		PackageNames:     packageNames,
		Parallelism:      flags.parallelism,
		APIKey:           apiKey,
		ModelID:          modelID,
		MaxIterations:    flags.maxIterations,
		EnableTracing:    tracingEnabled,
		Profile:          profile,
		ThinkingBudget:   thinkingBudget,
	}

	result, err := docagent.RunBatchEvaluation(cmd.Context(), batchCfg)
	if err != nil {
		return fmt.Errorf("batch evaluation failed: %w", err)
	}

	printBatchEvaluationSummary(cmd, result, flags.outputDir)
	return nil
}

// runSinglePackageEvaluation executes evaluation for a single package
func runSinglePackageEvaluation(cmd *cobra.Command, flags evaluationModeFlags, profile *profile.Profile, apiKey, modelID string, thinkingBudget *int32, tracingConfig tracing.Config) error {
	packageRoot, err := packages.FindPackageRoot()
	if err != nil {
		return fmt.Errorf("locating package root failed: %w", err)
	}

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
		OutputDir:     flags.outputDir,
		MaxIterations: flags.maxIterations,
		EnableTracing: tracingConfig.Enabled,
		ModelID:       modelID,
	}

	result, err := docAgent.EvaluateDocumentation(cmd.Context(), evalCfg)
	if err != nil {
		return fmt.Errorf("evaluation failed: %w", err)
	}

	printSingleEvaluationSummary(cmd, result, flags.outputDir)
	return nil
}

// handleEvaluationMode handles the --evaluate flag for documentation quality evaluation
func handleEvaluationMode(cmd *cobra.Command, profile *profile.Profile, apiKey, modelID string, thinkingBudget *int32) error {
	if apiKey == "" {
		return fmt.Errorf("evaluation mode requires GOOGLE_API_KEY to be set")
	}

	flags, err := getEvaluationModeFlags(cmd)
	if err != nil {
		return err
	}

	tracingConfig := getTracingConfig(profile)
	if tracingConfig.Enabled {
		if err := tracing.InitWithConfig(cmd.Context(), tracingConfig); err != nil {
			cmd.Printf("Warning: failed to initialize tracing: %v\n", err)
		}
	}

	cmd.Printf("📊 Running documentation evaluation with model: %s\n", modelID)

	if flags.batchFlag != "" {
		return runBatchEvaluation(cmd, flags, profile, apiKey, modelID, thinkingBudget, tracingConfig.Enabled)
	}

	return runSinglePackageEvaluation(cmd, flags, profile, apiKey, modelID, thinkingBudget, tracingConfig)
}
