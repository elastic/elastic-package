// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package docagent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"go.opentelemetry.io/otel/trace"

	"github.com/elastic/elastic-package/internal/llmagent/docagent/specialists"
	"github.com/elastic/elastic-package/internal/llmagent/docagent/specialists/validators"
	"github.com/elastic/elastic-package/internal/llmagent/tracing"
	"github.com/elastic/elastic-package/internal/logger"
)

// EvaluationConfig holds configuration for documentation evaluation
type EvaluationConfig struct {
	// OutputDir is the directory to save evaluation results
	OutputDir string

	// MaxIterations limits retries per section generation
	MaxIterations uint

	// EnableTracing enables Phoenix tracing
	EnableTracing bool

	// ModelID is the LLM model to use
	ModelID string
}

// DefaultEvaluationConfig returns default evaluation configuration
func DefaultEvaluationConfig() EvaluationConfig {
	return EvaluationConfig{
		OutputDir:     "./doc_eval_results",
		MaxIterations: 3,
		EnableTracing: false,
		ModelID:       "gemini-3-flash-preview",
	}
}

// EvaluationResult holds the results of documentation evaluation
type EvaluationResult struct {
	// PackageName is the name of the evaluated package
	PackageName string `json:"package_name"`

	// PackagePath is the full path to the package
	PackagePath string `json:"package_path"`

	// RunID uniquely identifies this evaluation run
	RunID string `json:"run_id"`

	// Timestamp when the evaluation started
	Timestamp time.Time `json:"timestamp"`

	// Duration of the evaluation
	Duration time.Duration `json:"duration"`

	// Config holds the configuration used for this evaluation
	Config EvaluationConfig `json:"config"`

	// GeneratedContent is the final generated documentation
	GeneratedContent string `json:"generated_content"`

	// OriginalContent is the original README (if it existed)
	OriginalContent string `json:"original_content,omitempty"`

	// Approved indicates if all validation stages passed
	Approved bool `json:"approved"`

	// Metrics holds computed quality metrics
	Metrics *QualityMetrics `json:"metrics,omitempty"`

	// ValidationSummary provides a quick overview of validation results
	ValidationSummary *ValidationSummary `json:"validation_summary,omitempty"`

	// Error contains any error message
	Error string `json:"error,omitempty"`

	// TraceSessionID is the Phoenix session ID for this run
	TraceSessionID string `json:"trace_session_id,omitempty"`

	// TraceSummary holds aggregated trace data from Phoenix (if tracing enabled)
	TraceSummary *tracing.TraceSummary `json:"trace_summary,omitempty"`
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
	TopIssues []string `json:"top_issues"`

	// FailureReason provides a human-readable summary of why validation failed
	FailureReason string `json:"failure_reason,omitempty"`
}

// StageResult holds results for a single validation stage
type StageResult struct {
	Stage          string                  `json:"stage"`
	Valid          bool                    `json:"valid"`
	Score          int                     `json:"score"`
	Iterations     int                     `json:"iterations"`
	Issues         []string                `json:"issues,omitempty"`
	DetailedIssues []ValidationIssueDetail `json:"detailed_issues,omitempty"`
	Warnings       []string                `json:"warnings,omitempty"`
	Suggestions    []string                `json:"suggestions,omitempty"`
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

// EvaluateDocumentation runs documentation generation in evaluation mode
// Returns the evaluation result with metrics instead of writing to the package
// Uses the same generation + validation loop as update documentation
func (d *DocumentationAgent) EvaluateDocumentation(ctx context.Context, cfg EvaluationConfig) (*EvaluationResult, error) {
	startTime := time.Now()
	runID := fmt.Sprintf("%s_%s", d.manifest.Name, startTime.Format("20060102_150405"))

	result := &EvaluationResult{
		PackageName: d.manifest.Name,
		PackagePath: d.packageRoot,
		RunID:       runID,
		Timestamp:   startTime,
		Config:      cfg,
	}

	// Initialize tracing - track span so we can end it before flushing
	var sessionSpan trace.Span
	if cfg.EnableTracing {
		ctx, sessionSpan = tracing.StartSessionSpan(ctx, "doc:evaluate", d.executor.ModelID())
		// Note: We'll end the span explicitly before flushing, but keep defer as safety net
		defer func() {
			if sessionSpan != nil && sessionSpan.IsRecording() {
				tracing.EndSessionSpan(ctx, sessionSpan, result.GeneratedContent)
			}
		}()

		if tracing.IsEnabled() {
			if sessionID, ok := tracing.SessionIDFromContext(ctx); ok {
				result.TraceSessionID = sessionID
				fmt.Printf("🔍 Tracing session ID: %s\n", sessionID)
			}
		}
	}

	// Confirm LLM understands the documentation guidelines
	if err := d.ConfirmInstructionsUnderstood(ctx); err != nil {
		result.Error = fmt.Sprintf("instruction confirmation failed: %v", err)
		result.Duration = time.Since(startTime)
		return result, err
	}

	// Read original README if it exists
	originalReadmePath := filepath.Join(d.packageRoot, "_dev", "build", "docs", d.targetDocFile)
	if content, err := os.ReadFile(originalReadmePath); err == nil {
		result.OriginalContent = string(content)
	}

	// Load package context for metrics computation
	pkgCtx, err := validators.LoadPackageContextForDoc(d.packageRoot, d.targetDocFile)
	if err != nil {
		result.Error = fmt.Sprintf("failed to load package context: %v", err)
		result.Duration = time.Since(startTime)
		return result, err
	}

	// Build generation config from evaluation config
	genCfg := GenerationConfig{
		MaxIterations:          cfg.MaxIterations,
		EnableStagedValidation: true,
		EnableLLMValidation:    true,
	}
	if genCfg.MaxIterations == 0 {
		genCfg.MaxIterations = 3
	}

	// Use the same section-based generation method as --non-interactive mode
	fmt.Printf("📊 Starting section-based generation (max %d iterations per section)...\n", genCfg.MaxIterations)
	genResult, err := d.GenerateAllSectionsWithValidation(ctx, pkgCtx, genCfg)
	if err != nil {
		result.Error = fmt.Sprintf("failed to generate documentation: %v", err)
		result.Duration = time.Since(startTime)
		return result, err
	}

	// Copy results from generation
	result.GeneratedContent = genResult.Content

	// Validate the final assembled document
	stageResults, finalApproved := d.validateFinalDocument(ctx, genResult.Content, pkgCtx)
	result.Approved = finalApproved

	// Compute quality metrics
	result.Metrics = ComputeMetrics(genResult.Content, pkgCtx)

	// Build validation summary from final document validation
	result.ValidationSummary = buildValidationSummary(stageResults, result.Approved)

	// Log final status
	if result.Approved {
		fmt.Printf("✅ Final document approved after generation\n")
	} else {
		fmt.Printf("⚠️ Final document failed validation. Score: %.1f\n", result.Metrics.CompositeScore)
	}

	result.Duration = time.Since(startTime)

	// Fetch trace summary from Phoenix if tracing was enabled
	if cfg.EnableTracing && result.TraceSessionID != "" {
		// End the session span BEFORE flushing so it gets included in the export
		if sessionSpan != nil {
			tracing.EndSessionSpan(ctx, sessionSpan, result.GeneratedContent)
			sessionSpan = nil // Mark as ended so defer doesn't double-end
		}

		// Force flush pending traces to Phoenix before fetching
		if err := tracing.ForceFlush(ctx); err != nil {
			logger.Debugf("Failed to flush traces: %v", err)
		}

		// Give Phoenix time to ingest the traces
		fmt.Printf("🔍 Fetching trace summary from Phoenix...\n")
		time.Sleep(2 * time.Second)

		traceSummary, err := fetchTraceSummaryFromPhoenix(ctx, result.TraceSessionID)
		if err != nil {
			logger.Debugf("Failed to fetch trace summary: %v", err)
		} else if traceSummary != nil {
			result.TraceSummary = traceSummary
			fmt.Printf("📊 Trace summary: %d spans, %d LLM calls, %d total tokens\n",
				traceSummary.TotalSpans, traceSummary.LLMCalls, traceSummary.TotalTokens)
		}
	}

	// Save result to output directory
	if cfg.OutputDir != "" {
		if err := saveEvaluationResult(result, cfg.OutputDir); err != nil {
			logger.Debugf("Failed to save evaluation result: %v", err)
		}
	}

	return result, nil
}

// validateFinalDocument runs all validators against the final assembled document.
func (d *DocumentationAgent) validateFinalDocument(ctx context.Context, content string, pkgCtx *validators.PackageContext) (map[string]*StageResult, bool) {
	stageResults := make(map[string]*StageResult)
	generate := d.createLLMValidateFunc()

	for _, validator := range specialists.AllStagedValidators() {
		if validator.Scope() == validators.ScopeSectionLevel {
			continue
		}

		stageName := validator.Stage().String()
		stageResult, ok := stageResults[stageName]
		if !ok {
			stageResult = &StageResult{
				Stage: stageName,
				Valid: true,
				Score: 100,
			}
			stageResults[stageName] = stageResult
		}

		var staticResult *validators.StagedValidationResult
		if validator.SupportsStaticValidation() {
			res, err := validator.StaticValidate(ctx, content, pkgCtx)
			if err != nil {
				logger.Debugf("Static validation error for %s: %v", validator.Name(), err)
			} else {
				staticResult = res
			}
		}

		var llmResult *validators.StagedValidationResult
		if validator.SupportsLLMValidation() {
			res, err := validator.LLMValidate(ctx, content, pkgCtx, generate)
			if err != nil {
				logger.Debugf("LLM validation error for %s: %v", validator.Name(), err)
			} else {
				llmResult = res
			}
		}

		merged := validators.MergeValidationResults(staticResult, llmResult)
		if merged == nil {
			continue
		}

		stageResult.Valid = stageResult.Valid && merged.Valid
		if merged.Score < stageResult.Score {
			stageResult.Score = merged.Score
		}

		for _, issue := range merged.Issues {
			message := issue.Message
			if issue.Location != "" {
				message = fmt.Sprintf("%s: %s", issue.Location, issue.Message)
			}
			stageResult.Issues = append(stageResult.Issues, fmt.Sprintf("[%s] %s", validator.Name(), message))
			stageResult.DetailedIssues = append(stageResult.DetailedIssues, ValidationIssueDetail{
				Severity:    string(issue.Severity),
				Category:    string(issue.Category),
				Location:    issue.Location,
				Message:     issue.Message,
				Suggestion:  issue.Suggestion,
				SourceCheck: issue.SourceCheck,
			})
		}

		stageResult.Warnings = append(stageResult.Warnings, merged.Warnings...)
		stageResult.Suggestions = append(stageResult.Suggestions, merged.Suggestions...)
	}

	approved := true
	for _, stageResult := range stageResults {
		if !stageResult.Valid {
			approved = false
			break
		}
	}

	return stageResults, approved
}

// fetchTraceSummaryFromPhoenix fetches trace data from Phoenix
func fetchTraceSummaryFromPhoenix(ctx context.Context, sessionID string) (*tracing.TraceSummary, error) {
	client := tracing.NewPhoenixClient(tracing.DefaultEndpoint)

	// Check if Phoenix is available
	if !client.IsPhoenixAvailable(ctx) {
		logger.Debugf("Phoenix not available at %s", tracing.DefaultEndpoint)
		return nil, nil
	}

	// Fetch traces
	traces, err := client.FetchSessionTraces(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch traces: %w", err)
	}

	if traces == nil || traces.Summary == nil {
		return nil, nil
	}

	return traces.Summary, nil
}

// buildValidationSummary creates a summary of validation results
func buildValidationSummary(stageResults map[string]*StageResult, approved bool) *ValidationSummary {
	summary := &ValidationSummary{
		TopIssues: make([]string, 0),
	}

	for stageName, stageRes := range stageResults {
		if stageRes.Valid {
			summary.PassedStages = append(summary.PassedStages, stageName)
		} else {
			summary.FailedStages = append(summary.FailedStages, stageName)
		}

		issueCount := len(stageRes.Issues)
		if len(stageRes.DetailedIssues) > 0 {
			issueCount = len(stageRes.DetailedIssues)
		}
		summary.TotalIssues += issueCount

		// Count by severity from detailed issues if available
		for _, detail := range stageRes.DetailedIssues {
			switch detail.Severity {
			case "critical":
				summary.CriticalIssues++
			case "major":
				summary.MajorIssues++
			case "minor":
				summary.MinorIssues++
			}
		}

		// Add top issues (up to 5)
		issuesForSummary := stageRes.Issues
		if len(issuesForSummary) == 0 && len(stageRes.DetailedIssues) > 0 {
			for _, detail := range stageRes.DetailedIssues {
				message := detail.Message
				if detail.Location != "" {
					message = fmt.Sprintf("%s: %s", detail.Location, detail.Message)
				}
				issuesForSummary = append(issuesForSummary, message)
			}
		}
		for _, issue := range issuesForSummary {
			if len(summary.TopIssues) < 5 {
				summary.TopIssues = append(summary.TopIssues, fmt.Sprintf("[%s] %s", stageName, issue))
			}
		}
	}

	// Build failure reason
	if !approved && len(summary.FailedStages) > 0 {
		summary.FailureReason = fmt.Sprintf("Validation failed in %d stage(s): %v. Found %d critical, %d major, and %d minor issue(s).",
			len(summary.FailedStages),
			summary.FailedStages,
			summary.CriticalIssues,
			summary.MajorIssues,
			summary.MinorIssues)
	}

	return summary
}

// saveEvaluationResult saves the evaluation result to a JSON file and generated content to .md
func saveEvaluationResult(result *EvaluationResult, outputDir string) error {
	resultDir := filepath.Join(outputDir, "results", result.PackageName)
	if err := os.MkdirAll(resultDir, 0o755); err != nil {
		return err
	}

	// Save JSON result
	resultPath := filepath.Join(resultDir, result.RunID+".json")
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(resultPath, data, 0o644); err != nil {
		return err
	}

	// Save generated content as markdown file
	mdPath := filepath.Join(resultDir, result.RunID+".md")
	if err := os.WriteFile(mdPath, []byte(result.GeneratedContent), 0o644); err != nil {
		logger.Debugf("Failed to save generated markdown: %v", err)
	}

	// Save original content if available
	if result.OriginalContent != "" {
		originalPath := filepath.Join(resultDir, result.RunID+"_original.md")
		if err := os.WriteFile(originalPath, []byte(result.OriginalContent), 0o644); err != nil {
			logger.Debugf("Failed to save original markdown: %v", err)
		}
	}

	return nil
}
