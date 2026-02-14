// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package doceval

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/elastic/elastic-package/internal/llmagent/docagent"
	"github.com/elastic/elastic-package/internal/llmagent/tracing"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/profile"
)

// BatchEvaluationConfig holds configuration for batch documentation evaluation
type BatchEvaluationConfig struct {
	// IntegrationsPath is the path to the integrations repository
	IntegrationsPath string

	// OutputDir is the directory to save evaluation results
	OutputDir string

	// PackageNames is the list of package names to evaluate
	PackageNames []string

	// Parallelism is the number of packages to process in parallel
	Parallelism int

	// APIKey is the Gemini API key
	APIKey string

	// ModelID is the LLM model ID to use
	ModelID string

	// MaxIterations limits retries per validation stage
	MaxIterations uint

	// EnableTracing enables Phoenix tracing
	EnableTracing bool

	// Profile is the elastic-package profile for configuration
	Profile *profile.Profile

	// ThinkingBudget is the thinking budget for Gemini models
	ThinkingBudget *int32
}

// BatchEvaluationResult holds results for multiple package evaluations
type BatchEvaluationResult struct {
	RunID     string              `json:"run_id"`
	Timestamp time.Time           `json:"timestamp"`
	Duration  time.Duration       `json:"duration"`
	Results   []*EvaluationResult `json:"results"`
	Summary   *BatchSummary       `json:"summary"`
}

// BatchSummary provides aggregate statistics for batch evaluation
type BatchSummary struct {
	TotalPackages  int     `json:"total_packages"`
	PassedPackages int     `json:"passed_packages"`
	FailedPackages int     `json:"failed_packages"`
	AverageScore   float64 `json:"average_score"`
}

// batchJob represents a package to evaluate
type batchJob struct {
	index       int
	packageName string
	packagePath string
}

// batchJobResult holds the result of an evaluation job
type batchJobResult struct {
	index  int
	result *EvaluationResult
}

// RunBatchEvaluation executes documentation evaluation for multiple packages
func RunBatchEvaluation(ctx context.Context, cfg BatchEvaluationConfig) (*BatchEvaluationResult, error) {
	startTime := time.Now()
	runID := fmt.Sprintf("batch_%s", startTime.Format("20060102_150405"))

	batchResult := &BatchEvaluationResult{
		RunID:     runID,
		Timestamp: startTime,
		Results:   make([]*EvaluationResult, 0, len(cfg.PackageNames)),
	}

	// Ensure output directory exists
	if cfg.OutputDir != "" {
		if err := os.MkdirAll(cfg.OutputDir, 0o755); err != nil {
			return nil, fmt.Errorf("failed to create output directory: %w", err)
		}
	}

	// Validate packages exist and build job list
	packagesDir := filepath.Join(cfg.IntegrationsPath, "packages")
	var jobs []batchJob
	for i, pkgName := range cfg.PackageNames {
		pkgPath := filepath.Join(packagesDir, pkgName)
		manifestPath := filepath.Join(pkgPath, "manifest.yml")
		if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
			logger.Warnf("Package %s not found at %s, skipping", pkgName, pkgPath)
			continue
		}
		jobs = append(jobs, batchJob{
			index:       i,
			packageName: pkgName,
			packagePath: pkgPath,
		})
	}

	if len(jobs) == 0 {
		return nil, fmt.Errorf("no valid packages found to evaluate")
	}

	// Determine parallelism
	parallelism := cfg.Parallelism
	if parallelism < 1 {
		parallelism = 1
	}

	logger.Debugf("Starting batch evaluation %s for %d packages (parallelism: %d)", runID, len(jobs), parallelism)

	if parallelism == 1 {
		// Sequential execution
		for _, job := range jobs {
			result, err := evaluatePackage(ctx, job, cfg)
			if err != nil {
				logger.Debugf("Evaluation failed for %s: %v", job.packageName, err)
			}
			batchResult.Results = append(batchResult.Results, result)
		}
	} else {
		// Parallel execution with worker pool
		batchResult.Results = runParallelEvaluation(ctx, jobs, cfg, parallelism)
	}

	batchResult.Duration = time.Since(startTime)
	batchResult.Summary = computeBatchSummary(batchResult.Results)

	// Save batch result
	if cfg.OutputDir != "" {
		if err := saveBatchResult(batchResult, cfg.OutputDir); err != nil {
			logger.Debugf("Failed to save batch result: %v", err)
		}
	}

	return batchResult, nil
}

// evaluatePackage evaluates a single package
func evaluatePackage(ctx context.Context, job batchJob, cfg BatchEvaluationConfig) (*EvaluationResult, error) {
	logger.Debugf("Evaluating: %s", job.packageName)

	// Read package manifest
	manifest, err := packages.ReadPackageManifestFromPackageRoot(job.packagePath)
	if err != nil {
		return &EvaluationResult{
			PackageName: job.packageName,
			PackagePath: job.packagePath,
			Error:       fmt.Sprintf("failed to read manifest: %v", err),
			Timestamp:   time.Now(),
		}, err
	}

	// Get tracing configuration
	tracingConfig := tracing.Config{
		Enabled: cfg.EnableTracing,
	}

	// Create documentation agent for this package
	agentCfg := docagent.AgentConfig{
		APIKey:         cfg.APIKey,
		ModelID:        cfg.ModelID,
		PackageRoot:    job.packagePath,
		DocFile:        "README.md",
		Profile:        cfg.Profile,
		ThinkingBudget: cfg.ThinkingBudget,
		TracingConfig:  tracingConfig,
	}

	agent, err := docagent.NewDocumentationAgent(ctx, agentCfg)
	if err != nil {
		return &EvaluationResult{
			PackageName: job.packageName,
			PackagePath: job.packagePath,
			Error:       fmt.Sprintf("failed to create agent: %v", err),
			Timestamp:   time.Now(),
		}, err
	}

	// Run evaluation
	evalCfg := EvaluationConfig{
		OutputDir:     cfg.OutputDir,
		MaxIterations: cfg.MaxIterations,
		EnableTracing: cfg.EnableTracing,
		ModelID:       cfg.ModelID,
	}

	result, err := EvaluatePackage(ctx, agent, evalCfg)
	if err != nil {
		if result == nil {
			result = &EvaluationResult{
				PackageName: job.packageName,
				PackagePath: job.packagePath,
				Error:       fmt.Sprintf("evaluation failed: %v", err),
				Timestamp:   time.Now(),
			}
		}
	}

	// Report status
	score := 0.0
	if result.Metrics != nil {
		score = result.Metrics.CompositeScore
	}
	if result.Approved {
		logger.Debugf("Completed: %s (score: %.1f) - approved", manifest.Name, score)
	} else {
		logger.Debugf("Completed: %s (score: %.1f) - failed", manifest.Name, score)
	}

	return result, err
}

// runParallelEvaluation executes evaluations in parallel using a worker pool
func runParallelEvaluation(ctx context.Context, jobs []batchJob, cfg BatchEvaluationConfig, parallelism int) []*EvaluationResult {
	// Create channels for job distribution and result collection
	jobsChan := make(chan batchJob, len(jobs))
	results := make(chan batchJobResult, len(jobs))

	// Create wait group for workers
	var wg sync.WaitGroup

	// Start workers
	for w := 0; w < parallelism; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			evaluationWorker(ctx, workerID, jobsChan, results, cfg)
		}(w)
	}

	// Send jobs to workers with staggered starts to avoid API rate limiting
	go func() {
		for i, job := range jobs {
			jobsChan <- job
			// Stagger job starts by 500ms to avoid burst API requests
			if i < len(jobs)-1 {
				time.Sleep(500 * time.Millisecond)
			}
		}
		close(jobsChan)
	}()

	// Wait for all workers to complete, then close results channel
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results maintaining original order
	resultSlice := make([]*EvaluationResult, len(jobs))
	for jobResult := range results {
		resultSlice[jobResult.index] = jobResult.result
	}

	return resultSlice
}

// evaluationWorker processes evaluation jobs from the jobs channel
func evaluationWorker(ctx context.Context, workerID int, jobs <-chan batchJob, results chan<- batchJobResult, cfg BatchEvaluationConfig) {
	for job := range jobs {
		logger.Debugf("[Worker %d] Starting: %s", workerID, job.packageName)

		result, err := evaluatePackage(ctx, job, cfg)
		if err != nil {
			logger.Warnf("[Worker %d] Failed: %s - %v", workerID, job.packageName, err)
		}

		results <- batchJobResult{
			index:  job.index,
			result: result,
		}
	}
}

// computeBatchSummary calculates aggregate statistics for batch results
func computeBatchSummary(results []*EvaluationResult) *BatchSummary {
	summary := &BatchSummary{
		TotalPackages: len(results),
	}

	var totalScore float64
	for _, result := range results {
		if result == nil {
			summary.FailedPackages++
			continue
		}

		if result.Approved {
			summary.PassedPackages++
		} else {
			summary.FailedPackages++
		}
		if result.Metrics != nil {
			totalScore += result.Metrics.CompositeScore
		}
	}

	if summary.TotalPackages > 0 {
		summary.AverageScore = totalScore / float64(summary.TotalPackages)
	}

	return summary
}

// saveBatchResult saves the batch result to a JSON file
func saveBatchResult(result *BatchEvaluationResult, outputDir string) error {
	batchDir := filepath.Join(outputDir, "batch_results")
	if err := os.MkdirAll(batchDir, 0o755); err != nil {
		return err
	}

	resultPath := filepath.Join(batchDir, result.RunID+".json")
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(resultPath, data, 0o644)
}
