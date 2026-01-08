// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

// Package testing provides integration testing and evaluation tools for
// documentation generation with staged validation.
package testing

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"google.golang.org/adk/model/gemini"
	"google.golang.org/genai"

	"github.com/elastic/elastic-package/internal/llmagent/docagent/specialists"
	"github.com/elastic/elastic-package/internal/llmagent/docagent/specialists/validators"
	"github.com/elastic/elastic-package/internal/llmagent/docagent/workflow"
	"github.com/elastic/elastic-package/internal/llmagent/tracing"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/packages"
)

// TestConfig configures a documentation generation test run
type TestConfig struct {
	// EnableStagedValidation enables the new staged validators
	EnableStagedValidation bool

	// MaxIterationsPerStage limits retries per validation stage
	MaxIterationsPerStage uint

	// EnableSnapshots saves intermediate documents
	EnableSnapshots bool

	// EnableTracing exports traces to Phoenix
	EnableTracing bool

	// TracingEndpoint is the Phoenix/OTLP endpoint
	TracingEndpoint string

	// DocFile specifies which documentation file to generate
	DocFile string

	// EnableLLM enables actual LLM generation (requires API key)
	EnableLLM bool

	// APIKey is the Gemini API key (or from GEMINI_API_KEY env var)
	APIKey string

	// ModelID is the LLM model to use (defaults to gemini-3-flash-preview)
	ModelID string
}

// DefaultTestConfig returns a test configuration with sensible defaults
func DefaultTestConfig() TestConfig {
	apiKey := os.Getenv("GEMINI_API_KEY")
	return TestConfig{
		EnableStagedValidation: true,
		MaxIterationsPerStage:  2,
		EnableSnapshots:        true,
		EnableTracing:          false,
		TracingEndpoint:        "http://localhost:6006/v1/traces",
		DocFile:                "README.md",
		EnableLLM:              apiKey != "",
		APIKey:                 apiKey,
		ModelID:                "gemini-3-flash-preview",
	}
}

// TestResult holds the results of a documentation generation test
type TestResult struct {
	// PackageName is the name of the tested package
	PackageName string `json:"package_name"`

	// PackagePath is the full path to the package
	PackagePath string `json:"package_path"`

	// RunID uniquely identifies this test run
	RunID string `json:"run_id"`

	// Timestamp when the test started
	Timestamp time.Time `json:"timestamp"`

	// Duration of the test run
	Duration time.Duration `json:"duration"`

	// Config used for this test
	Config TestConfig `json:"config"`

	// GeneratedContent is the final generated documentation
	GeneratedContent string `json:"generated_content"`

	// OriginalContent is the original README (if it existed)
	OriginalContent string `json:"original_content,omitempty"`

	// Approved indicates if all validation stages passed
	Approved bool `json:"approved"`

	// TotalIterations across all stages
	TotalIterations int `json:"total_iterations"`

	// IssueHistory tracks critical/major issue counts per iteration (for convergence analysis)
	IssueHistory []int `json:"issue_history,omitempty"`

	// ConvergenceBonus indicates if an extra iteration was granted due to convergence
	ConvergenceBonus bool `json:"convergence_bonus,omitempty"`

	// ValidationSummary provides a quick overview of validation results
	ValidationSummary *ValidationSummary `json:"validation_summary,omitempty"`

	// StageResults holds per-stage validation results
	StageResults map[string]*StageResult `json:"stage_results,omitempty"`

	// Metrics holds computed quality metrics
	Metrics *QualityMetrics `json:"metrics,omitempty"`

	// Error contains any error message
	Error string `json:"error,omitempty"`

	// SnapshotDir is where intermediate snapshots were saved
	SnapshotDir string `json:"snapshot_dir,omitempty"`
}

// ValidationSummary provides a quick overview of validation results
type ValidationSummary struct {
	// TotalIssues is the total count of all issues across all stages
	TotalIssues int `json:"total_issues"`
	
	// CriticalIssues is the count of critical severity issues
	CriticalIssues int `json:"critical_issues"`
	
	// MajorIssues is the count of major severity issues
	MajorIssues int `json:"major_issues"`
	
	// MinorIssues is the count of minor severity issues
	MinorIssues int `json:"minor_issues"`
	
	// FailedStages lists the names of stages that failed validation
	FailedStages []string `json:"failed_stages,omitempty"`
	
	// PassedStages lists the names of stages that passed validation
	PassedStages []string `json:"passed_stages,omitempty"`
	
	// TopIssues lists the most critical issues (up to 5) for quick reference
	TopIssues []string `json:"top_issues,omitempty"`
	
	// FailureReason provides a human-readable summary of why validation failed
	FailureReason string `json:"failure_reason,omitempty"`
}

// StageResult holds results for a single validation stage
type StageResult struct {
	Stage         string                   `json:"stage"`
	Valid         bool                     `json:"valid"`
	Score         int                      `json:"score"`
	Iterations    int                      `json:"iterations"`
	Issues        []string                 `json:"issues,omitempty"`         // Simple issue messages (for backward compatibility)
	DetailedIssues []ValidationIssueDetail `json:"detailed_issues,omitempty"` // Full issue details
	Suggestions   []string                 `json:"suggestions,omitempty"`    // Actionable suggestions for fixing
	Warnings      []string                 `json:"warnings,omitempty"`       // Non-blocking warnings
}

// ValidationIssueDetail provides full details about a validation issue
type ValidationIssueDetail struct {
	Severity    string `json:"severity"`     // critical, major, minor
	Category    string `json:"category"`     // structure, accuracy, completeness, quality, placeholders
	Location    string `json:"location"`     // Where in the document the issue was found
	Message     string `json:"message"`      // Description of the issue
	Suggestion  string `json:"suggestion"`   // How to fix the issue
	SourceCheck string `json:"source_check"` // "static" or "llm"
}

// BatchResult holds results for multiple package tests
type BatchResult struct {
	RunID     string        `json:"run_id"`
	Timestamp time.Time     `json:"timestamp"`
	Duration  time.Duration `json:"duration"`
	Results   []*TestResult `json:"results"`
	Summary   *BatchSummary `json:"summary"`
}

// BatchSummary provides aggregate statistics
type BatchSummary struct {
	TotalPackages   int     `json:"total_packages"`
	PassedPackages  int     `json:"passed_packages"`
	FailedPackages  int     `json:"failed_packages"`
	AverageScore    float64 `json:"average_score"`
	TotalIterations int     `json:"total_iterations"`
}

// TestHarness runs documentation generation tests against real packages
type TestHarness struct {
	// IntegrationsPath is the path to the integrations repository
	IntegrationsPath string

	// OutputDir is where test results are saved
	OutputDir string

	// EnableTracing enables Phoenix tracing
	EnableTracing bool

	// TracingEndpoint is the Phoenix/OTLP endpoint
	TracingEndpoint string
}

// NewTestHarness creates a new test harness
func NewTestHarness(integrationsPath, outputDir string) *TestHarness {
	return &TestHarness{
		IntegrationsPath: integrationsPath,
		OutputDir:        outputDir,
		EnableTracing:    false,
		TracingEndpoint:  "http://localhost:6006/v1/traces",
	}
}

// DiscoverPackages finds all packages in the integrations repository
func (h *TestHarness) DiscoverPackages() ([]string, error) {
	packagesDir := filepath.Join(h.IntegrationsPath, "packages")

	entries, err := os.ReadDir(packagesDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read packages directory: %w", err)
	}

	var packageNames []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// Check if it has a manifest.yml
		manifestPath := filepath.Join(packagesDir, entry.Name(), "manifest.yml")
		if _, err := os.Stat(manifestPath); err == nil {
			packageNames = append(packageNames, entry.Name())
		}
	}

	return packageNames, nil
}

// GetPackagePath returns the full path to a package
func (h *TestHarness) GetPackagePath(packageName string) string {
	return filepath.Join(h.IntegrationsPath, "packages", packageName)
}

// RunTest executes a documentation generation test for a single package
func (h *TestHarness) RunTest(ctx context.Context, packageName string, cfg TestConfig) (*TestResult, error) {
	startTime := time.Now()
	runID := fmt.Sprintf("%s_%s", packageName, startTime.Format("20060102_150405"))

	result := &TestResult{
		PackageName:  packageName,
		PackagePath:  h.GetPackagePath(packageName),
		RunID:        runID,
		Timestamp:    startTime,
		Config:       cfg,
		StageResults: make(map[string]*StageResult),
	}

	logger.Debugf("Starting test run %s for package %s", runID, packageName)

	// Verify package exists
	if _, err := os.Stat(result.PackagePath); os.IsNotExist(err) {
		result.Error = fmt.Sprintf("package not found: %s", result.PackagePath)
		result.Duration = time.Since(startTime)
		return result, fmt.Errorf("package not found: %s", packageName)
	}

	// Load package context for validation
	pkgCtx, err := validators.LoadPackageContext(result.PackagePath)
	if err != nil {
		result.Error = fmt.Sprintf("failed to load package context: %v", err)
		result.Duration = time.Since(startTime)
		return result, err
	}

	// Read original README if it exists
	originalReadmePath := filepath.Join(result.PackagePath, "_dev", "build", "docs", cfg.DocFile)
	if content, err := os.ReadFile(originalReadmePath); err == nil {
		result.OriginalContent = string(content)
	}

	// Create snapshot manager if enabled
	var snapshotMgr *workflow.SnapshotManager
	if cfg.EnableSnapshots {
		snapshotDir := filepath.Join(h.OutputDir, "snapshots", packageName)
		snapshotMgr = workflow.NewSnapshotManager(snapshotDir, packageName)
		result.SnapshotDir = snapshotMgr.GetSessionDir()
	}

	// Run documentation generation
	if cfg.EnableStagedValidation {
		// Run with staged validation
		generatedContent, workflowResult, err := h.runStagedGeneration(ctx, pkgCtx, snapshotMgr, cfg)
		if err != nil {
			result.Error = fmt.Sprintf("staged generation failed: %v", err)
			result.Duration = time.Since(startTime)
			return result, err
		}

		result.GeneratedContent = generatedContent
		result.Approved = workflowResult.Approved
		result.TotalIterations = workflowResult.TotalIterations
		result.IssueHistory = workflowResult.IssueHistory
		result.ConvergenceBonus = workflowResult.ConvergenceBonus

		// Convert stage results with full details
		for stage, stageRes := range workflowResult.StageResults {
			// Simple issue messages (backward compatible)
			issues := make([]string, 0, len(stageRes.Issues))
			// Detailed issue information
			detailedIssues := make([]ValidationIssueDetail, 0, len(stageRes.Issues))
			
			for _, issue := range stageRes.Issues {
				issues = append(issues, issue.Message)
				detailedIssues = append(detailedIssues, ValidationIssueDetail{
					Severity:    string(issue.Severity),
					Category:    string(issue.Category),
					Location:    issue.Location,
					Message:     issue.Message,
					Suggestion:  issue.Suggestion,
					SourceCheck: issue.SourceCheck,
				})
			}
			result.StageResults[stage.String()] = &StageResult{
				Stage:          stage.String(),
				Valid:          stageRes.Valid,
				Score:          stageRes.Score,
				Issues:         issues,
				DetailedIssues: detailedIssues,
				Suggestions:    stageRes.Suggestions,
				Warnings:       stageRes.Warnings,
			}
		}
		
		// Build validation summary
		result.ValidationSummary = buildValidationSummary(result.StageResults, result.Approved)
	} else {
		// Run without staged validation (baseline)
		generatedContent, err := h.runBaselineGeneration(ctx, pkgCtx, cfg)
		if err != nil {
			result.Error = fmt.Sprintf("baseline generation failed: %v", err)
			result.Duration = time.Since(startTime)
			return result, err
		}
		result.GeneratedContent = generatedContent
		result.Approved = true // Baseline doesn't have validation
		result.TotalIterations = 1
	}

	// Compute quality metrics
	result.Metrics = ComputeMetrics(result.GeneratedContent, pkgCtx)

	// Save final snapshot
	if snapshotMgr != nil && cfg.EnableSnapshots {
		snapshotMgr.SaveSnapshot(result.GeneratedContent, "final", result.TotalIterations, nil)
	}

	result.Duration = time.Since(startTime)

	// Save result to file
	if err := h.saveResult(result); err != nil {
		logger.Debugf("Failed to save result: %v", err)
	}

	logger.Debugf("Test run %s completed: approved=%v, iterations=%d, duration=%v",
		runID, result.Approved, result.TotalIterations, result.Duration)

	return result, nil
}

// runStagedGeneration runs documentation generation with staged validation and feedback loop
func (h *TestHarness) runStagedGeneration(
	ctx context.Context,
	pkgCtx *validators.PackageContext,
	snapshotMgr *workflow.SnapshotManager,
	cfg TestConfig,
) (string, *workflow.StagedWorkflowResult, error) {

	var content string
	result := &workflow.StagedWorkflowResult{
		Approved:        true,
		TotalIterations: 0,
		StageResults:    make(map[validators.ValidatorStage]*validators.StagedValidationResult),
	}

	// Use the canonical list of all staged validators
	vals := specialists.AllStagedValidators()

	maxIterations := cfg.MaxIterationsPerStage
	if maxIterations == 0 {
		maxIterations = 3 // Default to 3 iterations
	}

	var feedback []string // Accumulated feedback for regeneration

	// Use actual LLM if enabled and API key is available
	if cfg.EnableLLM && cfg.APIKey != "" {
		// Initialize tracing for this generation
		tracingCfg := tracing.Config{
			Enabled:  cfg.EnableTracing,
			Endpoint: cfg.TracingEndpoint,
		}
		if err := tracing.InitWithConfig(ctx, tracingCfg); err != nil {
			logger.Debugf("Failed to initialize tracing: %v", err)
		}

		// Track issue counts across iterations for convergence detection
		issueHistory := make([]int, 0, int(maxIterations)+1)
		extraIterationAllowed := true // Allow one extra iteration if converging
		effectiveMaxIterations := maxIterations

		// Feedback loop: generate -> validate -> regenerate with feedback
		for iteration := uint(1); iteration <= effectiveMaxIterations; iteration++ {
			result.TotalIterations = int(iteration)

			iterationLabel := fmt.Sprintf("%d/%d", iteration, maxIterations)
			if iteration > maxIterations {
				iterationLabel = fmt.Sprintf("%d (bonus - converging)", iteration)
			}
			fmt.Printf("🤖 Generating documentation with LLM (model: %s, iteration %s)...\n", cfg.ModelID, iterationLabel)

			// Generate documentation using workflow (with feedback if available)
			generatedContent, err := h.runLLMGenerationWithFeedback(ctx, pkgCtx, cfg, feedback)
			if err != nil {
				fmt.Printf("❌ LLM generation error: %v\n", err)
				fmt.Println("💡 Tip: Try a different model. Known working models: gemini-3-flash-preview, gemini-3-pro-preview, gemini-2.5-pro")
				return "", nil, fmt.Errorf("LLM generation failed: %w", err)
			}
			content = generatedContent
			fmt.Printf("✅ Generated %d characters of documentation\n", len(content))

			// Save snapshot if enabled
			if snapshotMgr != nil {
				snapshotMgr.SaveSnapshot(content, fmt.Sprintf("iteration_%d", iteration), int(iteration), nil)
			}

			// Run staged validation
			fmt.Println("🔍 Running staged validation...")
			allValid := true
			feedback = nil // Reset feedback for this iteration

			for _, validator := range vals {
				if validator.SupportsStaticValidation() {
					staticResult, err := validator.StaticValidate(ctx, content, pkgCtx)
					if err != nil {
						logger.Debugf("Static validation error for %s: %v", validator.Name(), err)
						continue
					}

					// Track iterations per validator (using name to avoid overwriting when multiple validators share a stage)
					validatorKey := validator.Stage() // Use stage for backward compatibility with result structure
					validatorName := validator.Name()
					
					// Track iterations by validator name
					iterKey := validatorName
					if existing, ok := result.ValidatorIterations[iterKey]; ok {
						staticResult.Iterations = existing + 1
					} else {
						staticResult.Iterations = 1
					}
					if result.ValidatorIterations == nil {
						result.ValidatorIterations = make(map[string]int)
					}
					result.ValidatorIterations[iterKey] = staticResult.Iterations
					
					// Aggregate results for each stage (don't overwrite - merge issues with deduplication)
					if existing, ok := result.StageResults[validatorKey]; ok {
						// Merge issues from this validator into existing stage result (deduplicate)
						existing.Issues = mergeAndDeduplicateIssues(existing.Issues, staticResult.Issues)
						existing.Warnings = mergeAndDeduplicateStrings(existing.Warnings, staticResult.Warnings)
						existing.Suggestions = mergeAndDeduplicateStrings(existing.Suggestions, staticResult.Suggestions)
						if !staticResult.Valid {
							existing.Valid = false
						}
						// Keep the lower score
						if staticResult.Score < existing.Score {
							existing.Score = staticResult.Score
						}
					} else {
						result.StageResults[validatorKey] = staticResult
					}

					status := "✅"
					if !staticResult.Valid {
						status = "❌"
						allValid = false
						// Collect feedback from issues - include suggestions for better LLM guidance
						for _, issue := range staticResult.Issues {
							feedbackItem := fmt.Sprintf("[%s] %s: %s", validator.Stage().String(), issue.Location, issue.Message)
							if issue.Suggestion != "" {
								feedbackItem += fmt.Sprintf(" → FIX: %s", issue.Suggestion)
							}
							feedback = append(feedback, feedbackItem)
						}
					}
					fmt.Printf("  %s %s [%s] (iter %d): %d issues\n", status, validator.Stage().String(), validatorName, staticResult.Iterations, len(staticResult.Issues))
				}
			}

			// Count critical and major issues for this iteration
			iterationIssueCount := 0
			for _, stageResult := range result.StageResults {
				for _, issue := range stageResult.Issues {
					if issue.Severity == validators.SeverityCritical || issue.Severity == validators.SeverityMajor {
						iterationIssueCount++
					}
				}
			}
			issueHistory = append(issueHistory, iterationIssueCount)

			if allValid {
				fmt.Printf("✅ All validations passed after %d iteration(s)!\n", iteration)
				result.Approved = true
				break
			}

			// Check for convergence: are issues decreasing?
			isConverging := false
			if len(issueHistory) >= 2 {
				prevIssues := issueHistory[len(issueHistory)-2]
				currIssues := issueHistory[len(issueHistory)-1]
				isConverging = currIssues < prevIssues
				if isConverging {
					fmt.Printf("📉 Issue count decreasing: %d → %d (converging)\n", prevIssues, currIssues)
				}
			}

			if iteration < effectiveMaxIterations {
				fmt.Printf("🔄 Regenerating with %d feedback items...\n", len(feedback))
			} else if iteration == maxIterations && isConverging && extraIterationAllowed && iterationIssueCount > 0 {
				// Allow one extra iteration if we're converging but haven't hit zero
				effectiveMaxIterations = maxIterations + 1
				extraIterationAllowed = false // Only allow one extra
				result.ConvergenceBonus = true
				fmt.Printf("📈 Converging but not yet at zero issues (%d remaining). Allowing bonus iteration...\n", iterationIssueCount)
			} else {
				fmt.Printf("⚠️ Max iterations (%d) reached. %d critical/major issues remaining.\n", iteration, iterationIssueCount)
				result.Approved = false
			}
		}

		// Save issue history for convergence analysis
		result.IssueHistory = issueHistory
		if len(issueHistory) > 1 {
			fmt.Printf("📊 Issue convergence history: %v\n", issueHistory)
		}
	} else {
		// Use existing README content for static-only testing
		logger.Debugf("Using existing README content (no LLM API key configured)")
		fmt.Println("📄 Running static validation on existing README...")

		content = pkgCtx.ExistingReadme
		if content == "" {
			// Generate a basic structure if no README exists
			content = h.generateBasicReadme(pkgCtx)
		}

		result.TotalIterations = 1

		// Run staged validation (no loop for static-only)
		fmt.Println("🔍 Running staged validation...")
		for _, validator := range vals {
			if validator.SupportsStaticValidation() {
				staticResult, err := validator.StaticValidate(ctx, content, pkgCtx)
				if err != nil {
					logger.Debugf("Static validation error for %s: %v", validator.Name(), err)
					continue
				}
				result.StageResults[validator.Stage()] = staticResult
				if !staticResult.Valid {
					result.Approved = false
				}

				status := "✅"
				if !staticResult.Valid {
					status = "❌"
				}
				fmt.Printf("  %s %s: %d issues\n", status, validator.Stage().String(), len(staticResult.Issues))
			}
		}
	}

	result.Content = content
	return content, result, nil
}

// runLLMGeneration uses the workflow builder to generate documentation with LLM
func (h *TestHarness) runLLMGeneration(ctx context.Context, pkgCtx *validators.PackageContext, cfg TestConfig) (string, error) {
	// Create Gemini model
	modelID := cfg.ModelID
	if modelID == "" {
		modelID = "gemini-3-flash-preview"
	}

	llmModel, err := gemini.NewModel(ctx, modelID, &genai.ClientConfig{
		APIKey: cfg.APIKey,
	})
	if err != nil {
		return "", fmt.Errorf("failed to create Gemini model: %w", err)
	}

	// Build workflow configuration
	workflowCfg := workflow.DefaultConfig().
		WithModel(llmModel).
		WithModelID(modelID).
		WithMaxIterations(cfg.MaxIterationsPerStage)

	// Create section context for generation
	sectionCtx := validators.SectionContext{
		PackageName:  pkgCtx.Manifest.Name,
		PackageTitle: pkgCtx.Manifest.Title,
		SectionTitle: "Overview",
		SectionLevel: 2,
	}

	// Add existing content as reference
	if pkgCtx.ExistingReadme != "" {
		sectionCtx.ExistingContent = pkgCtx.ExistingReadme
	}

	// Build and run workflow
	builder := workflow.NewBuilder(workflowCfg)
	result, err := builder.ExecuteWorkflow(ctx, sectionCtx)
	if err != nil {
		return "", fmt.Errorf("workflow failed: %w", err)
	}

	return result.Content, nil
}

// runLLMGenerationWithFeedback generates documentation with feedback from previous validation
func (h *TestHarness) runLLMGenerationWithFeedback(ctx context.Context, pkgCtx *validators.PackageContext, cfg TestConfig, feedback []string) (string, error) {
	// Create Gemini model
	modelID := cfg.ModelID
	if modelID == "" {
		modelID = "gemini-3-flash-preview"
	}

	llmModel, err := gemini.NewModel(ctx, modelID, &genai.ClientConfig{
		APIKey: cfg.APIKey,
	})
	if err != nil {
		return "", fmt.Errorf("failed to create Gemini model: %w", err)
	}

	// Build workflow configuration
	workflowCfg := workflow.DefaultConfig().
		WithModel(llmModel).
		WithModelID(modelID).
		WithMaxIterations(cfg.MaxIterationsPerStage)

	// Create section context for generation
	sectionCtx := validators.SectionContext{
		PackageName:  pkgCtx.Manifest.Name,
		PackageTitle: pkgCtx.Manifest.Title,
		SectionTitle: "Full README",
		SectionLevel: 1,
	}

	// Add existing content as reference
	if pkgCtx.ExistingReadme != "" {
		sectionCtx.ExistingContent = pkgCtx.ExistingReadme
	}

	// Build rich context for the generator (HEAD START)
	headStartContext := buildHeadStartContext(pkgCtx, feedback)
	sectionCtx.AdditionalContext = headStartContext

	// Build and run workflow
	builder := workflow.NewBuilder(workflowCfg)
	result, err := builder.ExecuteWorkflow(ctx, sectionCtx)
	if err != nil {
		return "", fmt.Errorf("workflow failed: %w", err)
	}

	return result.Content, nil
}

// buildHeadStartContext delegates to the shared workflow.BuildHeadStartContext
// This ensures both `update documentation` and `test documentation` use the same context builder
func buildHeadStartContext(pkgCtx *validators.PackageContext, feedback []string) string {
	return workflow.BuildHeadStartContext(pkgCtx, feedback)
}

// generateBasicReadme creates a basic README structure from package metadata
func (h *TestHarness) generateBasicReadme(pkgCtx *validators.PackageContext) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# %s\n\n", pkgCtx.Manifest.Title))
	sb.WriteString("## Overview\n\n")
	if pkgCtx.Manifest.Description != "" {
		sb.WriteString(pkgCtx.Manifest.Description + "\n\n")
	}

	sb.WriteString("## Compatibility\n\n")
	sb.WriteString("<< INFORMATION NOT AVAILABLE - PLEASE UPDATE >>\n\n")

	sb.WriteString("## Setup\n\n")
	sb.WriteString("<< INFORMATION NOT AVAILABLE - PLEASE UPDATE >>\n\n")

	if len(pkgCtx.DataStreams) > 0 {
		sb.WriteString("## Data streams\n\n")
		for _, ds := range pkgCtx.DataStreams {
			sb.WriteString(fmt.Sprintf("### %s\n\n", ds.Title))
			if ds.Description != "" {
				sb.WriteString(ds.Description + "\n\n")
			}
		}
	}

	sb.WriteString("## Reference\n\n")
	sb.WriteString("<< INFORMATION NOT AVAILABLE - PLEASE UPDATE >>\n\n")

	return sb.String()
}

// runBaselineGeneration runs documentation generation without staged validation
func (h *TestHarness) runBaselineGeneration(
	ctx context.Context,
	pkgCtx *validators.PackageContext,
	cfg TestConfig,
) (string, error) {
	// Placeholder for baseline generation
	// In production, this would run the original single-pass generation

	mockContent := fmt.Sprintf("# %s\n\n## Overview\n\nBaseline documentation for %s.\n",
		pkgCtx.Manifest.Title, pkgCtx.Manifest.Name)

	_ = cfg // Used in production integration

	return mockContent, nil
}

// mergeAndDeduplicateIssues combines two slices of ValidationIssue and removes duplicates
func mergeAndDeduplicateIssues(existing, new []validators.ValidationIssue) []validators.ValidationIssue {
	seen := make(map[string]bool)
	result := make([]validators.ValidationIssue, 0)

	// Add existing issues
	for _, issue := range existing {
		key := fmt.Sprintf("%s|%s|%s", issue.Category, issue.Location, issue.Message)
		if !seen[key] {
			seen[key] = true
			result = append(result, issue)
		}
	}

	// Add new issues (if not already present)
	for _, issue := range new {
		key := fmt.Sprintf("%s|%s|%s", issue.Category, issue.Location, issue.Message)
		if !seen[key] {
			seen[key] = true
			result = append(result, issue)
		}
	}

	return result
}

// mergeAndDeduplicateStrings combines two slices of strings and removes duplicates
func mergeAndDeduplicateStrings(existing, new []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0)

	for _, s := range existing {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}

	for _, s := range new {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}

	return result
}

// RunBatchTests executes tests for multiple packages
func (h *TestHarness) RunBatchTests(ctx context.Context, packageNames []string, cfg TestConfig) (*BatchResult, error) {
	startTime := time.Now()
	runID := fmt.Sprintf("batch_%s", startTime.Format("20060102_150405"))

	batchResult := &BatchResult{
		RunID:     runID,
		Timestamp: startTime,
		Results:   make([]*TestResult, 0, len(packageNames)),
	}

	logger.Debugf("Starting batch test %s for %d packages", runID, len(packageNames))

	for _, pkgName := range packageNames {
		result, err := h.RunTest(ctx, pkgName, cfg)
		if err != nil {
			logger.Debugf("Test failed for %s: %v", pkgName, err)
		}
		batchResult.Results = append(batchResult.Results, result)
	}

	batchResult.Duration = time.Since(startTime)
	batchResult.Summary = h.computeBatchSummary(batchResult.Results)

	// Save batch result
	if err := h.saveBatchResult(batchResult); err != nil {
		logger.Debugf("Failed to save batch result: %v", err)
	}

	return batchResult, nil
}

// computeBatchSummary calculates aggregate statistics
func (h *TestHarness) computeBatchSummary(results []*TestResult) *BatchSummary {
	summary := &BatchSummary{
		TotalPackages: len(results),
	}

	var totalScore float64
	for _, result := range results {
		if result.Approved {
			summary.PassedPackages++
		} else {
			summary.FailedPackages++
		}
		summary.TotalIterations += result.TotalIterations

		if result.Metrics != nil {
			totalScore += result.Metrics.CompositeScore
		}
	}

	if summary.TotalPackages > 0 {
		summary.AverageScore = totalScore / float64(summary.TotalPackages)
	}

	return summary
}

// CompareResults compares baseline and staged test results
func (h *TestHarness) CompareResults(baseline, staged *TestResult) *Comparison {
	comparison := &Comparison{
		PackageName:   baseline.PackageName,
		BaselineRunID: baseline.RunID,
		StagedRunID:   staged.RunID,
		Timestamp:     time.Now(),
		StageDeltas:   make(map[string]*StageDelta),
	}

	// Compare metrics
	if baseline.Metrics != nil && staged.Metrics != nil {
		comparison.BaselineScore = baseline.Metrics.CompositeScore
		comparison.StagedScore = staged.Metrics.CompositeScore
		comparison.ScoreDelta = staged.Metrics.CompositeScore - baseline.Metrics.CompositeScore
		comparison.PercentImprovement = 0
		if baseline.Metrics.CompositeScore > 0 {
			comparison.PercentImprovement = (comparison.ScoreDelta / baseline.Metrics.CompositeScore) * 100
		}

		// Compare individual metrics
		comparison.StructureDelta = staged.Metrics.StructureScore - baseline.Metrics.StructureScore
		comparison.AccuracyDelta = staged.Metrics.AccuracyScore - baseline.Metrics.AccuracyScore
		comparison.CompletenessDelta = staged.Metrics.CompletenessScore - baseline.Metrics.CompletenessScore
		comparison.QualityDelta = staged.Metrics.QualityScore - baseline.Metrics.QualityScore
		comparison.PlaceholderDelta = baseline.Metrics.PlaceholderCount - staged.Metrics.PlaceholderCount
	}

	// Compare iterations
	comparison.IterationDelta = staged.TotalIterations - baseline.TotalIterations

	// Compare per-stage results
	for stage, stagedRes := range staged.StageResults {
		delta := &StageDelta{
			Stage:      stage,
			StagedPass: stagedRes.Valid,
		}

		if baselineRes, ok := baseline.StageResults[stage]; ok {
			delta.BaselinePass = baselineRes.Valid
			delta.ScoreDelta = stagedRes.Score - baselineRes.Score
			delta.IssuesDelta = len(baselineRes.Issues) - len(stagedRes.Issues)
		}

		comparison.StageDeltas[stage] = delta
	}

	return comparison
}

// Comparison holds the comparison between baseline and staged results
type Comparison struct {
	PackageName        string                 `json:"package_name"`
	BaselineRunID      string                 `json:"baseline_run_id"`
	StagedRunID        string                 `json:"staged_run_id"`
	Timestamp          time.Time              `json:"timestamp"`
	BaselineScore      float64                `json:"baseline_score"`
	StagedScore        float64                `json:"staged_score"`
	ScoreDelta         float64                `json:"score_delta"`
	PercentImprovement float64                `json:"percent_improvement"`
	StructureDelta     float64                `json:"structure_delta"`
	AccuracyDelta      float64                `json:"accuracy_delta"`
	CompletenessDelta  float64                `json:"completeness_delta"`
	QualityDelta       float64                `json:"quality_delta"`
	PlaceholderDelta   int                    `json:"placeholder_delta"`
	IterationDelta     int                    `json:"iteration_delta"`
	StageDeltas        map[string]*StageDelta `json:"stage_deltas"`
}

// StageDelta holds the comparison for a single validation stage
type StageDelta struct {
	Stage        string `json:"stage"`
	BaselinePass bool   `json:"baseline_pass"`
	StagedPass   bool   `json:"staged_pass"`
	ScoreDelta   int    `json:"score_delta"`
	IssuesDelta  int    `json:"issues_delta"`
}

// saveResult saves a test result to a JSON file
func (h *TestHarness) saveResult(result *TestResult) error {
	resultDir := filepath.Join(h.OutputDir, "results", result.PackageName)
	if err := os.MkdirAll(resultDir, 0755); err != nil {
		return err
	}

	resultPath := filepath.Join(resultDir, result.RunID+".json")
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(resultPath, data, 0644)
}

// saveBatchResult saves a batch result to a JSON file
func (h *TestHarness) saveBatchResult(result *BatchResult) error {
	resultDir := filepath.Join(h.OutputDir, "batch_results")
	if err := os.MkdirAll(resultDir, 0755); err != nil {
		return err
	}

	resultPath := filepath.Join(resultDir, result.RunID+".json")
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(resultPath, data, 0644)
}

// LoadResult loads a test result from a JSON file
func LoadResult(filePath string) (*TestResult, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var result TestResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// LoadBatchResult loads a batch result from a JSON file
func LoadBatchResult(filePath string) (*BatchResult, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var result BatchResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// ValidatePackageExists checks if a package exists in the integrations repo
func (h *TestHarness) ValidatePackageExists(packageName string) error {
	pkgPath := h.GetPackagePath(packageName)
	manifestPath := filepath.Join(pkgPath, "manifest.yml")

	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		return fmt.Errorf("package '%s' not found at %s", packageName, pkgPath)
	}

	// Verify it's a valid package
	_, err := packages.ReadPackageManifestFromPackageRoot(pkgPath)
	if err != nil {
		return fmt.Errorf("invalid package manifest: %w", err)
	}

	return nil
}

// buildValidationSummary creates a summary of validation results
func buildValidationSummary(stageResults map[string]*StageResult, approved bool) *ValidationSummary {
	summary := &ValidationSummary{}
	
	var allIssues []ValidationIssueDetail
	
	for stageName, stageRes := range stageResults {
		if stageRes.Valid {
			summary.PassedStages = append(summary.PassedStages, stageName)
		} else {
			summary.FailedStages = append(summary.FailedStages, stageName)
		}
		
		// Count issues by severity
		for _, issue := range stageRes.DetailedIssues {
			summary.TotalIssues++
			allIssues = append(allIssues, issue)
			
			switch issue.Severity {
			case "critical":
				summary.CriticalIssues++
			case "major":
				summary.MajorIssues++
			case "minor":
				summary.MinorIssues++
			}
		}
	}
	
	// Extract top issues (prioritize critical, then major)
	// Sort by severity: critical first, then major, then minor
	criticalIssues := filterIssuesBySeverity(allIssues, "critical")
	majorIssues := filterIssuesBySeverity(allIssues, "major")
	
	topIssues := make([]string, 0, 5)
	for _, issue := range criticalIssues {
		if len(topIssues) >= 5 {
			break
		}
		issueStr := fmt.Sprintf("[CRITICAL] %s: %s", issue.Location, issue.Message)
		if issue.Suggestion != "" {
			issueStr += fmt.Sprintf(" → %s", issue.Suggestion)
		}
		topIssues = append(topIssues, issueStr)
	}
	for _, issue := range majorIssues {
		if len(topIssues) >= 5 {
			break
		}
		issueStr := fmt.Sprintf("[MAJOR] %s: %s", issue.Location, issue.Message)
		if issue.Suggestion != "" {
			issueStr += fmt.Sprintf(" → %s", issue.Suggestion)
		}
		topIssues = append(topIssues, issueStr)
	}
	summary.TopIssues = topIssues
	
	// Build failure reason
	if !approved {
		if len(summary.FailedStages) > 0 {
			summary.FailureReason = fmt.Sprintf("Validation failed in %d stage(s): %s. Found %d critical, %d major, and %d minor issue(s).",
				len(summary.FailedStages),
				strings.Join(summary.FailedStages, ", "),
				summary.CriticalIssues,
				summary.MajorIssues,
				summary.MinorIssues)
		}
	}
	
	return summary
}

// filterIssuesBySeverity returns issues matching the given severity
func filterIssuesBySeverity(issues []ValidationIssueDetail, severity string) []ValidationIssueDetail {
	var filtered []ValidationIssueDetail
	for _, issue := range issues {
		if issue.Severity == severity {
			filtered = append(filtered, issue)
		}
	}
	return filtered
}
