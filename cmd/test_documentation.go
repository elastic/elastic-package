// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/elastic/elastic-package/internal/cobraext"
	"github.com/elastic/elastic-package/internal/llmagent/docagent/specialists/validators"
	"github.com/elastic/elastic-package/internal/llmagent/docagent/testing"
	"github.com/elastic/elastic-package/internal/llmagent/tracing"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/packages"
)

const (
	docTestFlagBatch            = "batch"
	docTestFlagBatchDescription = "Comma-separated list of package names to test in batch"

	docTestFlagEnableStaged     = "enable-staged-validation"
	docTestFlagEnableStagedDesc = "Enable staged validation (default: true)"

	docTestFlagOutputDir     = "output-dir"
	docTestFlagOutputDirDesc = "Directory to save test results"

	docTestFlagGoldenDir     = "golden-dir"
	docTestFlagGoldenDirDesc = "Directory containing golden files"

	docTestFlagCreateGolden     = "create-golden"
	docTestFlagCreateGoldenDesc = "Create a golden file from the specified package's existing README"

	docTestFlagCompare         = "compare"
	docTestFlagCompareDesc     = "Compare two test runs"
	docTestFlagBaselineRun     = "baseline-run"
	docTestFlagBaselineRunDesc = "Path to baseline test result JSON for comparison"
	docTestFlagStagedRun       = "staged-run"
	docTestFlagStagedRunDesc   = "Path to staged test result JSON for comparison"

	docTestFlagIntegrationsPath     = "integrations-path"
	docTestFlagIntegrationsPathDesc = "Path to integrations repository (overrides INTEGRATIONS_PATH env var)"

	docTestFlagEnableTracing     = "enable-tracing"
	docTestFlagEnableTracingDesc = "Enable Phoenix tracing for test runs"

	docTestFlagMaxIterations     = "max-iterations"
	docTestFlagMaxIterationsDesc = "Maximum iterations per validation stage (default 3)"

	docTestFlagEnableLLM     = "enable-llm"
	docTestFlagEnableLLMDesc = "Enable LLM generation (requires GEMINI_API_KEY env var)"

	docTestFlagModelID     = "model"
	docTestFlagModelIDDesc = "LLM model ID to use (default: gemini-3-flash-preview)"

	docTestFlagClearResults     = "clear-results"
	docTestFlagClearResultsDesc = "Clear previous results from output directory before running tests (default: true)"

	docTestFlagParallel     = "parallel"
	docTestFlagParallelDesc = "Number of integrations to process in parallel in batch mode (default: 1)"
)

func getTestRunnerDocumentationCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "documentation",
		Short: "Run documentation generation tests for the package.",
		Long: `Run documentation generation tests for the package.

This command tests the documentation generation workflow with staged validation
against real packages to measure quality improvements.

Test modes:
1. Single package test: Tests documentation generation for one package
2. Batch test: Tests multiple packages and generates comparison reports
3. Comparison test: Compares baseline vs staged validation results

Examples:
  # Test a single package
  elastic-package test documentation -C /path/to/package

  # Test multiple packages in batch
  elastic-package test documentation --batch citrix_adc,nginx,apache

  # Compare baseline vs staged validation
  elastic-package test documentation --compare --baseline-run <id> --staged-run <id>

  # Generate golden file from existing README
  elastic-package test documentation --create-golden citrix_adc
`,
		Args: cobra.NoArgs,
		RunE: testRunnerDocumentationCommandAction,
	}

	// Add flags
	cmd.Flags().String(docTestFlagBatch, "", docTestFlagBatchDescription)
	cmd.Flags().Bool(docTestFlagEnableStaged, true, docTestFlagEnableStagedDesc)
	cmd.Flags().String(docTestFlagOutputDir, "", docTestFlagOutputDirDesc)
	cmd.Flags().String(docTestFlagGoldenDir, "", docTestFlagGoldenDirDesc)
	cmd.Flags().String(docTestFlagCreateGolden, "", docTestFlagCreateGoldenDesc)
	cmd.Flags().Bool(docTestFlagCompare, false, docTestFlagCompareDesc)
	cmd.Flags().String(docTestFlagBaselineRun, "", docTestFlagBaselineRunDesc)
	cmd.Flags().String(docTestFlagStagedRun, "", docTestFlagStagedRunDesc)
	cmd.Flags().String(docTestFlagIntegrationsPath, "", docTestFlagIntegrationsPathDesc)
	cmd.Flags().Bool(docTestFlagEnableTracing, false, docTestFlagEnableTracingDesc)
	cmd.Flags().Uint(docTestFlagMaxIterations, 3, docTestFlagMaxIterationsDesc)
	cmd.Flags().Bool(docTestFlagEnableLLM, false, docTestFlagEnableLLMDesc)
	cmd.Flags().String(docTestFlagModelID, "gemini-3-flash-preview", docTestFlagModelIDDesc)
	cmd.Flags().Bool(docTestFlagClearResults, true, docTestFlagClearResultsDesc)
	cmd.Flags().Int(docTestFlagParallel, 1, docTestFlagParallelDesc)

	return cmd
}

func testRunnerDocumentationCommandAction(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Parse flags
	batchPackages, err := cmd.Flags().GetString(docTestFlagBatch)
	if err != nil {
		return cobraext.FlagParsingError(err, docTestFlagBatch)
	}

	enableStaged, err := cmd.Flags().GetBool(docTestFlagEnableStaged)
	if err != nil {
		return cobraext.FlagParsingError(err, docTestFlagEnableStaged)
	}

	outputDir, err := cmd.Flags().GetString(docTestFlagOutputDir)
	if err != nil {
		return cobraext.FlagParsingError(err, docTestFlagOutputDir)
	}

	goldenDir, err := cmd.Flags().GetString(docTestFlagGoldenDir)
	if err != nil {
		return cobraext.FlagParsingError(err, docTestFlagGoldenDir)
	}

	createGolden, err := cmd.Flags().GetString(docTestFlagCreateGolden)
	if err != nil {
		return cobraext.FlagParsingError(err, docTestFlagCreateGolden)
	}

	integrationsPath, err := cmd.Flags().GetString(docTestFlagIntegrationsPath)
	if err != nil {
		return cobraext.FlagParsingError(err, docTestFlagIntegrationsPath)
	}

	enableTracing, err := cmd.Flags().GetBool(docTestFlagEnableTracing)
	if err != nil {
		return cobraext.FlagParsingError(err, docTestFlagEnableTracing)
	}

	maxIterations, err := cmd.Flags().GetUint(docTestFlagMaxIterations)
	if err != nil {
		return cobraext.FlagParsingError(err, docTestFlagMaxIterations)
	}

	enableLLM, err := cmd.Flags().GetBool(docTestFlagEnableLLM)
	if err != nil {
		return cobraext.FlagParsingError(err, docTestFlagEnableLLM)
	}

	modelID, err := cmd.Flags().GetString(docTestFlagModelID)
	if err != nil {
		return cobraext.FlagParsingError(err, docTestFlagModelID)
	}

	clearResults, err := cmd.Flags().GetBool(docTestFlagClearResults)
	if err != nil {
		return cobraext.FlagParsingError(err, docTestFlagClearResults)
	}

	parallelism, err := cmd.Flags().GetInt(docTestFlagParallel)
	if err != nil {
		return cobraext.FlagParsingError(err, docTestFlagParallel)
	}
	if parallelism < 1 {
		parallelism = 1
	}

	// Determine integrations path
	if integrationsPath == "" {
		integrationsPath = os.Getenv("INTEGRATIONS_PATH")
	}
	if integrationsPath == "" && batchPackages != "" {
		return fmt.Errorf("integrations path not specified. Use --integrations-path or set INTEGRATIONS_PATH environment variable")
	}

	// Set up output directory
	if outputDir == "" {
		outputDir = filepath.Join(".", "doc_test_results")
	}

	// Clear previous results if requested
	if clearResults {
		fmt.Printf("🧹 Clearing previous results from %s...\n", outputDir)
		if err := clearResultsDirectory(outputDir); err != nil {
			return fmt.Errorf("failed to clear results directory: %w", err)
		}
		fmt.Println("✅ Previous results cleared")
	}

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Set up golden directory
	if goldenDir == "" {
		goldenDir = filepath.Join(outputDir, "golden")
	}
	if err := os.MkdirAll(goldenDir, 0755); err != nil {
		return fmt.Errorf("failed to create golden directory: %w", err)
	}

	// Initialize tracing if enabled
	if enableTracing {
		tracingCfg := tracing.ConfigFromEnv()
		tracingCfg.Enabled = true
		if err := tracing.InitWithConfig(ctx, tracingCfg); err != nil {
			logger.Debugf("Failed to initialize tracing: %v", err)
		} else {
			defer tracing.Shutdown(ctx)
		}
	}

	// Get API Key from environment variable
	apiKey := os.Getenv("GEMINI_API_KEY")
	if enableLLM && apiKey == "" {
		return fmt.Errorf("GEMINI_API_KEY environment variable is not set. Required for LLM generation")
	}

	harness := testing.NewTestHarness(integrationsPath, outputDir)

	testCfg := testing.TestConfig{
		EnableStagedValidation: enableStaged,
		EnableSnapshots:        true,
		MaxIterationsPerStage:  maxIterations,
		EnableLLM:              enableLLM,
		APIKey:                 apiKey,
		ModelID:                modelID,
		EnableTracing:          enableTracing,
		Parallelism:            parallelism,
	}

	// Handle create-golden mode
	if createGolden != "" {
		return handleCreateGolden(ctx, harness, createGolden, integrationsPath)
	}

	// Handle batch mode
	if batchPackages != "" {
		packageList := strings.Split(batchPackages, ",")
		for i := range packageList {
			packageList[i] = strings.TrimSpace(packageList[i])
		}

		if parallelism > 1 {
			fmt.Printf("Running batch documentation test for %d packages (parallel: %d): %s\n", len(packageList), parallelism, batchPackages)
		} else {
			fmt.Printf("Running batch documentation test for %d packages: %s\n", len(packageList), batchPackages)
		}

		batchResult, err := harness.RunBatchTests(ctx, packageList, testCfg)
		if err != nil {
			return fmt.Errorf("batch test failed: %w", err)
		}

		// Print batch report
		fmt.Println("# Batch Test Report")
		fmt.Printf("\n**Run ID**: %s\n", batchResult.RunID)
		fmt.Printf("**Timestamp**: %s\n", batchResult.Timestamp.Format("2006-01-02T15:04:05-07:00"))
		fmt.Printf("**Duration**: %v\n", batchResult.Duration)
		fmt.Println("\n## Summary")
		fmt.Println("\n| Metric | Value |")
		fmt.Println("|--------|-------|")
		fmt.Printf("| Total Packages | %d |\n", batchResult.Summary.TotalPackages)
		fmt.Printf("| Passed | %d |\n", batchResult.Summary.PassedPackages)
		fmt.Printf("| Failed | %d |\n", batchResult.Summary.FailedPackages)
		fmt.Printf("| Average Score | %.1f |\n", batchResult.Summary.AverageScore)
		fmt.Printf("| Total Iterations | %d |\n", batchResult.Summary.TotalIterations)
		fmt.Println("\n## Package Results")
		fmt.Println("\n| Package | Status | Score | Iterations |")
		fmt.Println("|---------|--------|-------|------------|")
		for _, r := range batchResult.Results {
			status := "✅"
			if !r.Approved {
				status = "❌"
			}
			fmt.Printf("| %s | %s | %.1f | %d |\n", r.PackageName, status, r.Metrics.CompositeScore, r.TotalIterations)
		}

		// Save batch results
		batchResultsDir := filepath.Join(outputDir, "batch_results")
		if err := os.MkdirAll(batchResultsDir, 0755); err != nil {
			return fmt.Errorf("failed to create batch results directory: %w", err)
		}

		batchResultPath := filepath.Join(batchResultsDir, fmt.Sprintf("%s.json", batchResult.RunID))
		batchResultJSON, _ := json.MarshalIndent(batchResult, "", "  ")
		if err := os.WriteFile(batchResultPath, batchResultJSON, 0644); err != nil {
			logger.Debugf("Failed to write batch result: %v", err)
		}

		fmt.Printf("Batch test completed. Results saved to: %s\n", outputDir)

		// Return error if any packages failed
		failedCount := 0
		for _, result := range batchResult.Results {
			if !result.Approved {
				failedCount++
			}
		}
		if failedCount > 0 {
			return fmt.Errorf("%d of %d packages failed validation", failedCount, len(batchResult.Results))
		}

		return nil
	}

	// Single package test (run from package directory)
	packageRoot, err := packages.MustFindPackageRoot()
	if err != nil {
		return fmt.Errorf("cannot find package root: %w. Use --batch for batch testing or run from a package directory", err)
	}

	manifest, err := packages.ReadPackageManifestFromPackageRoot(packageRoot)
	if err != nil {
		return fmt.Errorf("failed to read package manifest: %w", err)
	}

	fmt.Printf("Running documentation test for package: %s\n", manifest.Name)

	result, err := harness.RunTest(ctx, manifest.Name, testCfg)
	if err != nil {
		return fmt.Errorf("test failed: %w", err)
	}

	// Print result summary
	fmt.Printf("\nTest Result for %s:\n", manifest.Name)
	fmt.Printf("  Status: %s\n", map[bool]string{true: "✅ PASSED", false: "❌ FAILED"}[result.Approved])
	fmt.Printf("  Iterations: %d\n", result.TotalIterations)
	fmt.Printf("  Duration: %v\n", result.Duration)
	fmt.Printf("  Composite Score: %.1f\n", result.Metrics.CompositeScore)

	if !result.Approved {
		return fmt.Errorf("documentation validation failed")
	}

	return nil
}

func handleCreateGolden(ctx context.Context, harness *testing.TestHarness, packageName, integrationsPath string) error {
	// Find package path
	packagesDir := filepath.Join(integrationsPath, "packages")
	packagePath := filepath.Join(packagesDir, packageName)

	if _, err := os.Stat(packagePath); os.IsNotExist(err) {
		return fmt.Errorf("package %s not found at %s", packageName, packagePath)
	}

	pkgCtx, err := validators.LoadPackageContext(packagePath)
	if err != nil {
		return fmt.Errorf("failed to load package context for %s: %w", packageName, err)
	}

	readmeContent := pkgCtx.ExistingReadme
	if readmeContent == "" {
		return fmt.Errorf("no existing README.md found for package %s to create golden file from", packageName)
	}

	metrics := testing.ComputeMetrics(readmeContent, pkgCtx)

	// Save golden file
	goldenDir := filepath.Join(harness.OutputDir, "golden")
	if err := os.MkdirAll(goldenDir, 0755); err != nil {
		return fmt.Errorf("failed to create golden directory: %w", err)
	}

	goldenPath := filepath.Join(goldenDir, packageName+".golden.json")
	goldenData := map[string]interface{}{
		"package_name":    packageName,
		"content":         readmeContent,
		"quality_metrics": metrics,
		"created_at":      time.Now(),
	}
	goldenJSON, _ := json.MarshalIndent(goldenData, "", "  ")
	if err := os.WriteFile(goldenPath, goldenJSON, 0644); err != nil {
		return fmt.Errorf("failed to write golden file: %w", err)
	}

	fmt.Printf("Golden file created successfully:\n")
	fmt.Printf("  Package: %s\n", packageName)
	fmt.Printf("  Path: %s\n", goldenPath)
	fmt.Printf("  Composite Score: %.1f\n", metrics.CompositeScore)

	return nil
}

// clearResultsDirectory removes all contents from the results directory
func clearResultsDirectory(dir string) error {
	// Check if directory exists
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil // Nothing to clear
	}

	// Read directory contents
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("failed to read directory: %w", err)
	}

	// Remove each entry
	for _, entry := range entries {
		path := filepath.Join(dir, entry.Name())
		if entry.IsDir() {
			if err := os.RemoveAll(path); err != nil {
				return fmt.Errorf("failed to remove directory %s: %w", path, err)
			}
		} else {
			if err := os.Remove(path); err != nil {
				return fmt.Errorf("failed to remove file %s: %w", path, err)
			}
		}
	}

	return nil
}

