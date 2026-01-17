// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"
	"google.golang.org/genai"

	"github.com/elastic/elastic-package/internal/llmagent/docagent/specialists"
	"github.com/elastic/elastic-package/internal/llmagent/docagent/specialists/validators"
	"github.com/elastic/elastic-package/internal/llmagent/tracing"
	"github.com/elastic/elastic-package/internal/logger"
)

// StagedWorkflowConfig configures the staged validation workflow
type StagedWorkflowConfig struct {
	// Stages defines which validation stages to run and in what order
	Stages []validators.ValidatorStage

	// MaxIterationsPerStage is the max retries for each stage
	MaxIterationsPerStage uint

	// PackageContext provides package metadata for static validation
	PackageContext *validators.PackageContext

	// SnapshotManager handles saving intermediate documents
	SnapshotManager *SnapshotManager

	// EnableStaticValidation enables static (non-LLM) validation
	EnableStaticValidation bool

	// EnableLLMValidation enables LLM-based validation
	EnableLLMValidation bool
}

// StagedWorkflowResult holds the final result of the staged workflow
type StagedWorkflowResult struct {
	// Content is the final generated content
	Content string

	// Approved indicates if all validation stages passed
	Approved bool

	// TotalIterations is the total number of generation iterations
	TotalIterations int

	// StageResults holds results for each validation stage
	StageResults map[validators.ValidatorStage]*validators.StagedValidationResult

	// ValidatorIterations tracks iteration count per validator name
	ValidatorIterations map[string]int

	// FinalFeedback contains any remaining feedback
	FinalFeedback string

	// IssueHistory tracks critical/major issue counts per iteration (for convergence analysis)
	IssueHistory []int

	// ConvergenceBonus indicates if an extra iteration was granted due to convergence
	ConvergenceBonus bool
}

// StagedWorkflowBuilder builds and executes staged validation workflows
type StagedWorkflowBuilder struct {
	config      Config
	stagedCfg   StagedWorkflowConfig
	vals        map[validators.ValidatorStage]validators.StagedValidator
	stateStore  *specialists.StateStore
}

// NewStagedWorkflowBuilder creates a new staged workflow builder
func NewStagedWorkflowBuilder(cfg Config, stagedCfg StagedWorkflowConfig) *StagedWorkflowBuilder {
	if stagedCfg.MaxIterationsPerStage == 0 {
		stagedCfg.MaxIterationsPerStage = 2
	}
	if len(stagedCfg.Stages) == 0 {
		// Default stage order based on verification_prompt.txt sections
		stagedCfg.Stages = []validators.ValidatorStage{
			validators.StageStructure,    // Section A
			validators.StageAccuracy,     // Section B
			validators.StageCompleteness, // Section C
			validators.StageURLs,         // Section D
			validators.StageQuality,      // Section E
			validators.StagePlaceholders, // Section F
		}
	}
	if !stagedCfg.EnableStaticValidation && !stagedCfg.EnableLLMValidation {
		stagedCfg.EnableStaticValidation = true
		stagedCfg.EnableLLMValidation = true
	}

	return &StagedWorkflowBuilder{
		config:    cfg,
		stagedCfg: stagedCfg,
		vals:      make(map[validators.ValidatorStage]validators.StagedValidator),
	}
}

// RegisterValidator registers a validator for a specific stage
func (b *StagedWorkflowBuilder) RegisterValidator(validator validators.StagedValidator) {
	b.vals[validator.Stage()] = validator
}

// RegisterDefaultValidators registers all default staged validators
func (b *StagedWorkflowBuilder) RegisterDefaultValidators() {
	// Use the canonical list of all staged validators
	for _, validator := range specialists.AllStagedValidators() {
		b.RegisterValidator(validator)
	}
	// URL validator is already in the existing registry
}

// ExecuteStagedWorkflow runs the complete staged validation pipeline
func (b *StagedWorkflowBuilder) ExecuteStagedWorkflow(ctx context.Context, sectionCtx validators.SectionContext) (*StagedWorkflowResult, error) {
	// Start workflow span for tracing
	ctx, span := tracing.StartWorkflowSpanWithConfig(ctx, "workflow:staged", b.stagedCfg.MaxIterationsPerStage*uint(len(b.stagedCfg.Stages)))
	defer span.End()

	result := &StagedWorkflowResult{
		StageResults: make(map[validators.ValidatorStage]*validators.StagedValidationResult),
	}

	// Create state store for sharing data between agents
	b.stateStore = specialists.NewStateStore(nil)
	specialists.SetActiveStateStore(b.stateStore)
	defer specialists.ClearActiveStateStore()

	// Step 1: Initial content generation
	logger.Debugf("Starting initial content generation")
	content, err := b.runGenerator(ctx, sectionCtx, nil)
	if err != nil {
		return nil, fmt.Errorf("initial generation failed: %w", err)
	}
	result.Content = content
	result.TotalIterations = 1

	// Save initial snapshot
	if b.stagedCfg.SnapshotManager != nil {
		b.stagedCfg.SnapshotManager.SaveSnapshot(content, "initial", 0, nil)
	}

	// Step 2: Run each validation stage
	for _, stage := range b.stagedCfg.Stages {
		validator, ok := b.vals[stage]
		if !ok {
			logger.Debugf("Skipping stage %s - no validator registered", stage.String())
			continue
		}

		logger.Debugf("Starting validation stage: %s", stage.String())

		// Run stage with feedback loop
		stageResult, newContent, iterations, err := b.runValidationStage(ctx, validator, content, sectionCtx)
		if err != nil {
			logger.Debugf("Stage %s failed with error: %v", stage.String(), err)
			// Continue to next stage even if this one fails
			result.StageResults[stage] = &validators.StagedValidationResult{
				Stage: stage,
				Valid: false,
				Issues: []validators.ValidationIssue{{
					Severity: validators.SeverityCritical,
					Category: validators.ValidationCategory(stage.String()),
					Message:  fmt.Sprintf("Stage failed: %v", err),
				}},
			}
			continue
		}

		result.StageResults[stage] = stageResult
		result.TotalIterations += iterations

		if newContent != "" {
			content = newContent
			result.Content = content
		}

		// Save post-stage snapshot
		if b.stagedCfg.SnapshotManager != nil {
			b.stagedCfg.SnapshotManager.SaveSnapshot(content, stage.String(), iterations, stageResult)
		}

		logger.Debugf("Stage %s completed: valid=%v, iterations=%d", stage.String(), stageResult.Valid, iterations)
	}

	// Determine overall approval
	result.Approved = true
	for _, stageResult := range result.StageResults {
		if !stageResult.Valid {
			result.Approved = false
			if result.FinalFeedback == "" {
				result.FinalFeedback = stageResult.GetFeedbackForGenerator()
			} else {
				result.FinalFeedback += "\n\n" + stageResult.GetFeedbackForGenerator()
			}
		}
	}

	// Record final result
	tracing.RecordWorkflowResult(span, result.Approved, result.TotalIterations, result.Content)

	return result, nil
}

// runValidationStage runs a single validation stage with feedback loop
func (b *StagedWorkflowBuilder) runValidationStage(
	ctx context.Context,
	validator validators.StagedValidator,
	content string,
	sectionCtx validators.SectionContext,
) (*validators.StagedValidationResult, string, int, error) {

	var lastResult *validators.StagedValidationResult
	iterations := 0

	// Track issue counts for convergence detection
	issueHistory := make([]int, 0, int(b.stagedCfg.MaxIterationsPerStage)+1)
	extraIterationAllowed := true
	effectiveMaxIterations := b.stagedCfg.MaxIterationsPerStage

	for iteration := uint(0); iteration < effectiveMaxIterations; iteration++ {
		iterations++

		// Run static validation first (if enabled and supported)
		var staticResult *validators.StagedValidationResult
		if b.stagedCfg.EnableStaticValidation && validator.SupportsStaticValidation() {
			var err error
			staticResult, err = validator.StaticValidate(ctx, content, b.stagedCfg.PackageContext)
			if err != nil {
				logger.Debugf("Static validation error (non-fatal): %v", err)
			}

			// If static validation fails, regenerate immediately
			if staticResult != nil && !staticResult.Valid {
				logger.Debugf("Static validation failed with %d issues, regenerating", len(staticResult.Issues))

				feedback := staticResult.GetFeedbackForGenerator()
				newContent, err := b.runGenerator(ctx, sectionCtx, &feedback)
				if err != nil {
					return staticResult, content, iterations, fmt.Errorf("regeneration failed: %w", err)
				}
				content = newContent

				// Save intermediate snapshot
				if b.stagedCfg.SnapshotManager != nil {
					b.stagedCfg.SnapshotManager.SaveSnapshot(content, validator.Stage().String()+"_static", int(iteration), staticResult)
				}

				continue
			}
		}

		// Run LLM validation (if enabled)
		var llmResult *validators.StagedValidationResult
		if b.stagedCfg.EnableLLMValidation {
			llmOutput, err := b.runValidatorAgent(ctx, validator, content)
			if err != nil {
				logger.Debugf("LLM validation error (non-fatal): %v", err)
			} else {
				llmResult, _ = validators.ParseLLMValidationResult(llmOutput, validator.Stage())
			}
		}

		// Merge results
		lastResult = validators.MergeValidationResults(staticResult, llmResult)
		if lastResult == nil {
			lastResult = &validators.StagedValidationResult{
				Stage: validator.Stage(),
				Valid: true,
			}
		}

		// Track issue count for convergence detection
		issueCount := countCriticalMajorIssues(lastResult)
		issueHistory = append(issueHistory, issueCount)

		// If validation passed, we're done with this stage
		if lastResult.Valid {
			return lastResult, content, iterations, nil
		}

		// Check for convergence: are issues decreasing?
		isConverging := false
		if len(issueHistory) >= 2 {
			prevIssues := issueHistory[len(issueHistory)-2]
			currIssues := issueHistory[len(issueHistory)-1]
			isConverging = currIssues < prevIssues
			if isConverging {
				logger.Debugf("Issue count decreasing: %d → %d (converging)", prevIssues, currIssues)
			}
		}

		// If not the last iteration, regenerate with feedback
		if iteration < effectiveMaxIterations-1 {
			feedback := lastResult.GetFeedbackForGenerator()
			newContent, err := b.runGenerator(ctx, sectionCtx, &feedback)
			if err != nil {
				return lastResult, content, iterations, fmt.Errorf("regeneration failed: %w", err)
			}
			content = newContent

			// Save intermediate snapshot
			if b.stagedCfg.SnapshotManager != nil {
				b.stagedCfg.SnapshotManager.SaveSnapshot(content, validator.Stage().String()+"_regen", int(iteration), lastResult)
			}
		} else if iteration == b.stagedCfg.MaxIterationsPerStage-1 && isConverging && extraIterationAllowed && issueCount > 0 {
			// Allow one extra iteration if we're converging but haven't hit zero
			effectiveMaxIterations = b.stagedCfg.MaxIterationsPerStage + 1
			extraIterationAllowed = false
			logger.Debugf("Converging but not yet at zero issues (%d remaining). Allowing bonus iteration for %s", issueCount, validator.Name())

			// Regenerate with feedback for bonus iteration
			feedback := lastResult.GetFeedbackForGenerator()
			newContent, err := b.runGenerator(ctx, sectionCtx, &feedback)
			if err != nil {
				return lastResult, content, iterations, fmt.Errorf("regeneration failed: %w", err)
			}
			content = newContent
		}
	}

	return lastResult, content, iterations, nil
}

// countCriticalMajorIssues counts the number of critical and major issues in a result
func countCriticalMajorIssues(result *validators.StagedValidationResult) int {
	if result == nil {
		return 0
	}
	count := 0
	for _, issue := range result.Issues {
		if issue.Severity == validators.SeverityCritical || issue.Severity == validators.SeverityMajor {
			count++
		}
	}
	return count
}

// runGenerator executes the generator agent
func (b *StagedWorkflowBuilder) runGenerator(ctx context.Context, sectionCtx validators.SectionContext, feedback *string) (string, error) {
	prompt := buildGeneratorPrompt(sectionCtx, b.stateStore, b.stagedCfg.PackageContext)

	// Add feedback if provided
	if feedback != nil && *feedback != "" {
		prompt += "\n\n## Feedback to Address\n" + *feedback
	}

	output, _, _, err := b.runAgent(ctx, "generator", prompt)
	if err != nil {
		return "", err
	}

	// Update state store
	b.stateStore.Set(validators.StateKeyContent, output)

	return output, nil
}

// runValidatorAgent executes a validator agent
func (b *StagedWorkflowBuilder) runValidatorAgent(ctx context.Context, validator validators.StagedValidator, content string) (string, error) {
	// Build context-aware prompt
	var promptBuilder strings.Builder
	promptBuilder.WriteString("Validate the following documentation content:\n\n")
	promptBuilder.WriteString(content)

	// Add package context if available
	if b.stagedCfg.PackageContext != nil && b.stagedCfg.PackageContext.Manifest != nil {
		promptBuilder.WriteString("\n\n## Package Context\n")
		promptBuilder.WriteString(fmt.Sprintf("- Package Name: %s\n", b.stagedCfg.PackageContext.Manifest.Name))
		promptBuilder.WriteString(fmt.Sprintf("- Package Title: %s\n", b.stagedCfg.PackageContext.Manifest.Title))
		promptBuilder.WriteString(fmt.Sprintf("- Package Version: %s\n", b.stagedCfg.PackageContext.Manifest.Version))
		promptBuilder.WriteString(fmt.Sprintf("- Data Streams: %v\n", b.stagedCfg.PackageContext.GetDataStreamNames()))
	}

	output, _, _, err := b.runAgent(ctx, validator.Name(), promptBuilder.String())
	return output, err
}

// runAgent executes a single agent with an isolated session
func (b *StagedWorkflowBuilder) runAgent(ctx context.Context, agentName, prompt string) (output string, promptTokens, completionTokens int, err error) {
	// Start agent span
	_, agentSpan := tracing.StartAgentSpan(ctx, "agent:"+agentName, b.config.ModelID)
	defer func() {
		if promptTokens > 0 || completionTokens > 0 {
			tracing.EndLLMSpan(ctx, agentSpan, nil, promptTokens, completionTokens)
		} else {
			tracing.SetSpanOk(agentSpan)
			agentSpan.End()
		}
	}()

	// Build the agent
	adkAgent, err := b.buildAgent(ctx, agentName)
	if err != nil {
		return "", 0, 0, fmt.Errorf("failed to build agent: %w", err)
	}

	// Create isolated session service
	sessionService := session.InMemoryService()

	// Create runner
	r, err := runner.New(runner.Config{
		AppName:        "docagent-staged-" + agentName,
		Agent:          adkAgent,
		SessionService: sessionService,
	})
	if err != nil {
		return "", 0, 0, fmt.Errorf("failed to create runner: %w", err)
	}

	// Create session
	sess, err := sessionService.Create(ctx, &session.CreateRequest{
		AppName: "docagent-staged-" + agentName,
		UserID:  "docagent",
	})
	if err != nil {
		return "", 0, 0, fmt.Errorf("failed to create session: %w", err)
	}

	// Run the agent
	userContent := genai.NewContentFromText(prompt, genai.RoleUser)
	var outputs []string

	for event, err := range r.Run(ctx, "docagent", sess.Session.ID(), userContent, agent.RunConfig{}) {
		if err != nil {
			return "", promptTokens, completionTokens, fmt.Errorf("agent error: %w", err)
		}
		if event == nil {
			continue
		}

		// Accumulate token counts
		if event.UsageMetadata != nil {
			promptTokens += int(event.UsageMetadata.PromptTokenCount)
			completionTokens += int(event.UsageMetadata.CandidatesTokenCount)
		}

		// Capture text output
		if event.Content != nil {
			for _, part := range event.Content.Parts {
				if part.Text != "" {
					outputs = append(outputs, part.Text)
				}
			}
		}

		if event.IsFinalResponse() {
			break
		}
	}

	output = strings.TrimSpace(strings.Join(outputs, ""))
	logger.Debugf("Agent %s finished: %d chars output", agentName, len(output))
	return output, promptTokens, completionTokens, nil
}

// buildAgent creates an ADK agent by name
func (b *StagedWorkflowBuilder) buildAgent(ctx context.Context, name string) (agent.Agent, error) {
	agentCfg := validators.AgentConfig{
		Model:    b.config.Model,
		Tools:    b.config.Tools,
		Toolsets: b.config.Toolsets,
	}

	// Check staged validators first
	for _, validator := range b.vals {
		if validator.Name() == name {
			return validator.Build(ctx, agentCfg)
		}
	}

	// Fall back to registry
	if b.config.Registry != nil {
		for _, sa := range b.config.Registry.All() {
			if sa.Name() == name {
				return sa.Build(ctx, agentCfg)
			}
		}
	}

	return nil, fmt.Errorf("agent %s not found", name)
}

// GetStageOrder returns the current validation stage order
func (b *StagedWorkflowBuilder) GetStageOrder() []validators.ValidatorStage {
	return b.stagedCfg.Stages
}

// SetStageOrder sets a custom validation stage order
func (b *StagedWorkflowBuilder) SetStageOrder(stages []validators.ValidatorStage) {
	b.stagedCfg.Stages = stages
}

// GenerateAuditReport creates a summary report of the workflow execution
func (result *StagedWorkflowResult) GenerateAuditReport() string {
	var report strings.Builder

	report.WriteString("# Staged Validation Audit Report\n\n")
	report.WriteString(fmt.Sprintf("**Overall Status**: %s\n", map[bool]string{true: "✅ APPROVED", false: "❌ NEEDS REVISION"}[result.Approved]))
	report.WriteString(fmt.Sprintf("**Total Iterations**: %d\n\n", result.TotalIterations))

	report.WriteString("## Stage Results\n\n")
	for stage, stageResult := range result.StageResults {
		status := "✅"
		if !stageResult.Valid {
			status = "❌"
		}
		report.WriteString(fmt.Sprintf("### %s %s\n", status, stage.String()))
		report.WriteString(fmt.Sprintf("- Valid: %v\n", stageResult.Valid))
		report.WriteString(fmt.Sprintf("- Score: %d/100\n", stageResult.Score))
		report.WriteString(fmt.Sprintf("- Issues: %d\n", len(stageResult.Issues)))

		if len(stageResult.Issues) > 0 {
			report.WriteString("\n**Issues:**\n")
			for _, issue := range stageResult.Issues {
				report.WriteString(fmt.Sprintf("- [%s] %s: %s\n", issue.Severity, issue.Location, issue.Message))
			}
		}
		report.WriteString("\n")
	}

	if result.FinalFeedback != "" {
		report.WriteString("## Remaining Feedback\n\n")
		report.WriteString(result.FinalFeedback)
	}

	// Serialize full results as JSON for programmatic access
	report.WriteString("\n## Raw Results (JSON)\n\n```json\n")
	jsonBytes, _ := json.MarshalIndent(result, "", "  ")
	report.WriteString(string(jsonBytes))
	report.WriteString("\n```\n")

	return report.String()
}

