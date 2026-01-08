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

// Builder constructs multi-agent workflows for documentation generation
type Builder struct {
	config Config
}

// NewBuilder creates a new workflow builder with the given configuration
func NewBuilder(cfg Config) *Builder {
	if cfg.Registry == nil {
		cfg.Registry = specialists.DefaultRegistry()
	}
	if cfg.MaxIterations == 0 {
		cfg.MaxIterations = DefaultMaxIterations
	}
	return &Builder{config: cfg}
}

// Result holds the output of a workflow execution
type Result struct {
	// Content is the final generated content
	Content string
	// Approved indicates if the content passed all checks
	Approved bool
	// Iterations is the number of refinement cycles
	Iterations int
	// Feedback contains the final feedback (if any)
	Feedback string
	// ValidationResult contains validation results
	ValidationResult *validators.ValidationResult
	// URLCheckResult contains URL check results
	URLCheckResult *validators.URLCheckResult
}

// buildAgent creates a single ADK agent by name
func (b *Builder) buildAgent(ctx context.Context, name string) (agent.Agent, error) {
	agentCfg := validators.AgentConfig{
		Model:    b.config.Model,
		Tools:    b.config.Tools,
		Toolsets: b.config.Toolsets,
	}

	for _, sa := range b.config.Registry.All() {
		if sa.Name() == name {
			return sa.Build(ctx, agentCfg)
		}
	}
	return nil, fmt.Errorf("agent %s not found in registry", name)
}

// ExecuteWorkflow runs the workflow with isolated agent contexts.
// Each agent runs in its own session to prevent conversation history accumulation.
func (b *Builder) ExecuteWorkflow(ctx context.Context, sectionCtx validators.SectionContext) (*Result, error) {
	// Start workflow span for tracing
	ctx, span := tracing.StartWorkflowSpanWithConfig(ctx, "workflow:section", b.config.MaxIterations)

	result := &Result{}
	iterations := 0

	defer func() {
		tracing.RecordWorkflowResult(span, result.Approved, iterations, result.Content)
		span.End()
	}()

	// Create state store for sharing data between agents
	stateStore := specialists.NewStateStore(nil)
	specialists.SetActiveStateStore(stateStore)
	defer specialists.ClearActiveStateStore()

	// Run the workflow loop
	for iteration := uint(0); iteration < b.config.MaxIterations; iteration++ {
		iterations = int(iteration) + 1

		// Step 1: Run generator
		generatorPrompt := buildGeneratorPrompt(sectionCtx, stateStore)
		content, promptTokens, compTokens, err := b.runAgent(ctx, "generator", generatorPrompt)
		if err != nil {
			return nil, fmt.Errorf("generator failed: %w", err)
		}
		result.Content = content
		stateStore.Set(validators.StateKeyContent, content)
		logger.Debugf("Generator produced %d chars (tokens: %d/%d)", len(content), promptTokens, compTokens)

		// Step 2: Run critic (if enabled)
		if b.config.EnableCritic {
			criticPrompt := buildCriticPrompt(content)
			criticOutput, promptTokens, compTokens, err := b.runAgent(ctx, "critic", criticPrompt)
			if err != nil {
				return nil, fmt.Errorf("critic failed: %w", err)
			}
			logger.Debugf("Critic output (tokens: %d/%d): %s", promptTokens, compTokens, truncate(criticOutput, 100))

			// Parse critic result
			var criticResult specialists.CriticResult
			if err := json.Unmarshal([]byte(criticOutput), &criticResult); err != nil {
				logger.Debugf("Failed to parse critic JSON, assuming approved: %v", err)
				criticResult.Approved = true
			}

			if !criticResult.Approved {
				stateStore.Set(validators.StateKeyFeedback, criticResult.Feedback)
				result.Feedback = criticResult.Feedback
				logger.Debugf("Critic rejected: %s", criticResult.Feedback)
				continue // Re-run generator with feedback
			}
		}

		// Step 3: Validation is handled by staged validators in Step 5

		// Step 4: Run URL validator (if enabled) - this is typically programmatic, not LLM
		if b.config.EnableURLValidator {
			// URL validator runs programmatically, handled by the agent itself
			urlPrompt := fmt.Sprintf("Validate URLs in this content:\n\n%s", content)
			urlOutput, _, _, err := b.runAgent(ctx, "url_validator", urlPrompt)
			if err != nil {
				logger.Debugf("URL validator error (non-fatal): %v", err)
			} else {
				var urlResult validators.URLCheckResult
				if err := json.Unmarshal([]byte(urlOutput), &urlResult); err == nil {
					result.URLCheckResult = &urlResult
				}
			}
		}

		// Step 5: Run static validators (if enabled) - check against package files
		if b.config.EnableStaticValidation && b.config.PackageContext != nil {
			staticIssues := b.runStaticValidation(ctx, content)
			if len(staticIssues) > 0 {
				feedback := "Static validation issues found:\n"
				for _, issue := range staticIssues {
					feedback += fmt.Sprintf("- [%s] %s: %s", issue.Category, issue.Location, issue.Message)
					if issue.Suggestion != "" {
						feedback += fmt.Sprintf(" → FIX: %s", issue.Suggestion)
					}
					feedback += "\n"
				}
				stateStore.Set(validators.StateKeyFeedback, feedback)
				result.Feedback = feedback
				logger.Debugf("Static validation rejected with %d issues", len(staticIssues))
				continue // Re-run generator with feedback
			}
		}

		// All checks passed
		result.Approved = true
		logger.Debugf("Workflow approved at iteration %d", iterations)
		break
	}

	result.Iterations = iterations
	return result, nil
}

// runAgent executes a single agent with an isolated session and returns its output
func (b *Builder) runAgent(ctx context.Context, agentName, prompt string) (output string, promptTokens, completionTokens int, err error) {
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
		AppName:        "docagent-" + agentName,
		Agent:          adkAgent,
		SessionService: sessionService,
	})
	if err != nil {
		return "", 0, 0, fmt.Errorf("failed to create runner: %w", err)
	}

	// Create session
	sess, err := sessionService.Create(ctx, &session.CreateRequest{
		AppName: "docagent-" + agentName,
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

// truncate shortens a string for logging
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// buildGeneratorPrompt creates a prompt with all context embedded directly.
func buildGeneratorPrompt(sectionCtx validators.SectionContext, stateStore *specialists.StateStore) string {
	var prompt strings.Builder

	prompt.WriteString("Generate documentation for the following section.\n\n")
	prompt.WriteString("## Section Context\n")
	prompt.WriteString(fmt.Sprintf("- **SectionTitle**: %s\n", sectionCtx.SectionTitle))
	prompt.WriteString(fmt.Sprintf("- **SectionLevel**: %d (use %s for heading)\n", sectionCtx.SectionLevel, strings.Repeat("#", sectionCtx.SectionLevel)))
	prompt.WriteString(fmt.Sprintf("- **PackageName**: %s\n", sectionCtx.PackageName))
	prompt.WriteString(fmt.Sprintf("- **PackageTitle**: %s\n", sectionCtx.PackageTitle))

	if sectionCtx.TemplateContent != "" {
		prompt.WriteString("\n## Template Structure\n")
		prompt.WriteString("Follow this structure:\n```\n")
		prompt.WriteString(sectionCtx.TemplateContent)
		prompt.WriteString("\n```\n")
	}

	if sectionCtx.ExampleContent != "" {
		prompt.WriteString("\n## Style Reference (Example)\n")
		prompt.WriteString("Use this as a style guide:\n```\n")
		prompt.WriteString(sectionCtx.ExampleContent)
		prompt.WriteString("\n```\n")
	}

	if sectionCtx.ExistingContent != "" {
		prompt.WriteString("\n## Existing Content (to improve upon)\n")
		prompt.WriteString("```\n")
		prompt.WriteString(sectionCtx.ExistingContent)
		prompt.WriteString("\n```\n")
	}

	// Add additional context (e.g., feedback from validation)
	if sectionCtx.AdditionalContext != "" {
		prompt.WriteString("\n## Additional Instructions\n")
		prompt.WriteString(sectionCtx.AdditionalContext)
		prompt.WriteString("\n")
	}

	// Check for feedback from previous iteration (state store)
	if stateStore != nil {
		if fb, ok := stateStore.Get(validators.StateKeyFeedback); ok {
			if fbStr, ok := fb.(string); ok && fbStr != "" {
				prompt.WriteString("\n## Feedback to Address\n")
				prompt.WriteString(fbStr)
				prompt.WriteString("\n")
			}
		}
	}

	prompt.WriteString("\nOutput the markdown content directly, starting with the section heading.")

	return prompt.String()
}

// buildCriticPrompt creates a prompt for the critic with content embedded.
func buildCriticPrompt(content string) string {
	return fmt.Sprintf("Review this documentation for style, voice, tone, and accessibility:\n\n%s", content)
}

// runStaticValidation runs all static validators against the content
func (b *Builder) runStaticValidation(ctx context.Context, content string) []validators.ValidationIssue {
	var allIssues []validators.ValidationIssue
	pkgCtx := b.config.PackageContext

	// Use the canonical list of all staged validators
	vals := specialists.AllStagedValidators()

	for _, validator := range vals {
		if validator.SupportsStaticValidation() {
			result, err := validator.StaticValidate(ctx, content, pkgCtx)
			if err != nil {
				logger.Debugf("Static validation error for %s: %v", validator.Name(), err)
				continue
			}
			// Collect only major/critical issues that should block approval
			for _, issue := range result.Issues {
				if issue.Severity == validators.SeverityCritical || issue.Severity == validators.SeverityMajor {
					allIssues = append(allIssues, issue)
				}
			}
		}
	}

	return allIssues
}
