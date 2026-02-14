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
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"
	"google.golang.org/genai"

	"github.com/elastic/elastic-package/internal/llmagent/docagent/parsing"
	"github.com/elastic/elastic-package/internal/llmagent/docagent/specialists"
	"github.com/elastic/elastic-package/internal/llmagent/docagent/specialists/validators"
	"github.com/elastic/elastic-package/internal/llmagent/docagent/stylerules"
	"github.com/elastic/elastic-package/internal/llmagent/tools"
	"github.com/elastic/elastic-package/internal/llmagent/tracing"
	"github.com/elastic/elastic-package/internal/logger"
)

// truncateLen is the maximum length for truncated strings in logging/tracing
const truncateLen = 500

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
		generatorPrompt := buildGeneratorPrompt(sectionCtx, stateStore, b.config.PackageContext)
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

		// Step 6: Run LLM validators (if enabled) - use LLM to validate with validator instructions
		if b.config.EnableLLMValidation && b.config.PackageContext != nil {
			llmIssues := b.runLLMValidation(ctx, content)
			if len(llmIssues) > 0 {
				feedback := "LLM validation issues found:\n"
				for _, issue := range llmIssues {
					feedback += fmt.Sprintf("- [%s] %s: %s", issue.Category, issue.Location, issue.Message)
					if issue.Suggestion != "" {
						feedback += fmt.Sprintf(" → FIX: %s", issue.Suggestion)
					}
					feedback += "\n"
				}
				stateStore.Set(validators.StateKeyFeedback, feedback)
				result.Feedback = feedback
				logger.Debugf("LLM validation rejected with %d issues", len(llmIssues))
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
	adkAgent, err := b.buildAgent(ctx, agentName)
	if err != nil {
		return "", 0, 0, fmt.Errorf("failed to build agent: %w", err)
	}
	return b.runAgentWithADK(ctx, agentName, adkAgent, prompt)
}

// runAgentWithADK executes an ADK agent with an isolated session and returns its output
func (b *Builder) runAgentWithADK(ctx context.Context, agentName string, adkAgent agent.Agent, prompt string) (output string, promptTokens, completionTokens int, err error) {
	// Start agent span (container for the agent's work)
	agentCtx, agentSpan := tracing.StartAgentSpan(ctx, "agent:"+agentName, b.config.ModelID)
	defer func() {
		tracing.SetSpanOk(agentSpan)
		agentSpan.End()
	}()

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

	// Track input for LLM span
	inputMessages := []tracing.Message{{Role: "user", Content: truncate(prompt, truncateLen)}}

	for event, err := range r.Run(agentCtx, "docagent", sess.Session.ID(), userContent, agent.RunConfig{}) {
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

	// Create a proper LLM span with token counts for Phoenix cost calculation
	if promptTokens > 0 || completionTokens > 0 {
		_, llmSpan := tracing.StartLLMSpan(agentCtx, "llm:"+agentName, b.config.ModelID, inputMessages)
		outputMessages := []tracing.Message{{Role: "assistant", Content: truncate(output, truncateLen)}}
		tracing.EndLLMSpan(agentCtx, llmSpan, outputMessages, promptTokens, completionTokens)
	}

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
func buildGeneratorPrompt(sectionCtx validators.SectionContext, stateStore *specialists.StateStore, pkgCtx *validators.PackageContext) string {
	var prompt strings.Builder

	// Add critical formatting rules at the start for maximum visibility
	prompt.WriteString(stylerules.CriticalFormattingRules)
	prompt.WriteString("\n")
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
		prompt.WriteString("Use this as a style guide ONLY. Do not use it as a source of information:\n```\n")
		prompt.WriteString(sectionCtx.ExampleContent)
		prompt.WriteString("\n```\n")
	}

	if sectionCtx.ExistingContent != "" {
		prompt.WriteString("\n## Existing Content (to improve upon)\n")
		prompt.WriteString("```\n")
		prompt.WriteString(sectionCtx.ExistingContent)
		prompt.WriteString("\n```\n")
	}

	// Build package context directly from pkgCtx
	if pkgCtx != nil && pkgCtx.Manifest != nil {
		prompt.WriteString("\n## Package Context\n")
		prompt.WriteString(buildPackageContext(pkgCtx, sectionCtx.SectionTitle))
	}

	// Add validation feedback if passed via AdditionalContext (used in iteration loops)
	if sectionCtx.AdditionalContext != "" {
		prompt.WriteString("\n## Validation Feedback\n")
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
	return fmt.Sprintf("%s\nReview this documentation for style, voice, tone, and accessibility:\n\n%s", stylerules.CriticRejectionCriteria, content)
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

// runLLMValidation runs LLM-based validation using validator instructions
func (b *Builder) runLLMValidation(ctx context.Context, content string) []validators.ValidationIssue {
	var allIssues []validators.ValidationIssue
	pkgCtx := b.config.PackageContext

	// Use validators that have LLM instructions defined
	// These validators benefit most from LLM's semantic understanding
	llmValidators := []string{
		"style_validator",   // Elastic style guide compliance
		"quality_validator", // Writing quality assessment
	}

	vals := specialists.AllStagedValidators()

	logger.Debugf("Running LLM-based validation...")

	for _, validator := range vals {
		// Only run LLM validation for specific validators that benefit from it
		shouldRunLLM := false
		for _, name := range llmValidators {
			if validator.Name() == name {
				shouldRunLLM = true
				break
			}
		}
		if !shouldRunLLM {
			continue
		}

		// Skip if no instruction defined
		if validator.Instruction() == "" {
			continue
		}

		logger.Debugf("LLM validating with %s...", validator.Name())

		// Build context-aware prompt
		prompt := b.buildValidatorPrompt(validator, content, pkgCtx)

		// Run the LLM validator agent directly (not through registry)
		output, promptTokens, compTokens, err := b.runLLMValidatorAgent(ctx, validator, prompt)
		if err != nil {
			logger.Debugf("LLM validation error for %s: %v", validator.Name(), err)
			continue
		}
		logger.Debugf("LLM validator %s completed (tokens: %d/%d)", validator.Name(), promptTokens, compTokens)

		// Parse the LLM output
		result, err := validators.ParseLLMValidationResult(output, validator.Stage())
		if err != nil {
			logger.Debugf("Failed to parse LLM validation result for %s: %v", validator.Name(), err)
			continue
		}

		// Collect only major/critical issues
		llmIssueCount := 0
		for _, issue := range result.Issues {
			if issue.Severity == validators.SeverityCritical || issue.Severity == validators.SeverityMajor {
				issue.SourceCheck = "llm"
				allIssues = append(allIssues, issue)
				llmIssueCount++
			}
		}
		if llmIssueCount > 0 {
			logger.Debugf("LLM found %d critical/major issues", llmIssueCount)
		}
	}

	return allIssues
}

// runLLMValidatorAgent runs an LLM validator agent directly (not through registry)
func (b *Builder) runLLMValidatorAgent(ctx context.Context, validator validators.StagedValidator, prompt string) (output string, promptTokens, completionTokens int, err error) {
	agentName := "llm_validator:" + validator.Name()

	// Build the agent directly using llmagent.New (not from registry)
	adkAgent, err := llmagent.New(llmagent.Config{
		Name:                     agentName,
		Description:              "LLM-based validator for " + validator.Name(),
		Model:                    b.config.Model,
		Instruction:              validator.Instruction(),
		DisallowTransferToParent: true,
		DisallowTransferToPeers:  true,
		GenerateContentConfig: &genai.GenerateContentConfig{
			ResponseMIMEType: "application/json",
		},
	})
	if err != nil {
		return "", 0, 0, fmt.Errorf("failed to create agent: %w", err)
	}

	return b.runAgentWithADK(ctx, agentName, adkAgent, prompt)
}

// buildValidatorPrompt creates a prompt for LLM validation
func (b *Builder) buildValidatorPrompt(validator validators.StagedValidator, content string, pkgCtx *validators.PackageContext) string {
	var prompt strings.Builder

	// Add the validator's instruction
	prompt.WriteString(validator.Instruction())
	prompt.WriteString("\n\n")

	// Add context about the package
	if pkgCtx != nil {
		prompt.WriteString("=== PACKAGE CONTEXT ===\n")
		prompt.WriteString(fmt.Sprintf("Package: %s (%s)\n", pkgCtx.Manifest.Name, pkgCtx.Manifest.Title))

		// Add vendor links if available
		if pkgCtx.HasServiceInfoLinks() {
			prompt.WriteString("\n=== VENDOR DOCUMENTATION LINKS ===\n")
			for _, link := range pkgCtx.GetServiceInfoLinks() {
				prompt.WriteString(fmt.Sprintf("- [%s](%s)\n", link.Text, link.URL))
			}
		}

		prompt.WriteString("\n")
	}

	// Add the content to validate
	prompt.WriteString("=== DOCUMENTATION TO VALIDATE ===\n")
	prompt.WriteString(content)

	return prompt.String()
}

// buildPackageContext builds complete context for the generator from package metadata
// sectionTitle is used to filter service info content to only relevant sections
func buildPackageContext(pkgCtx *validators.PackageContext, sectionTitle string) string {
	if pkgCtx == nil || pkgCtx.Manifest == nil {
		return ""
	}

	var sb strings.Builder

	// Package information
	sb.WriteString("=== PACKAGE INFORMATION ===\n")
	sb.WriteString(fmt.Sprintf("Package Name: %s\n", pkgCtx.Manifest.Name))
	sb.WriteString(fmt.Sprintf("Package Title: %s\n", pkgCtx.Manifest.Title))
	if pkgCtx.Manifest.Description != "" {
		sb.WriteString(fmt.Sprintf("Description: %s\n", pkgCtx.Manifest.Description))
	}

	// Data streams
	if len(pkgCtx.DataStreams) > 0 {
		sb.WriteString("Data streams in this package:\n")
		for _, ds := range pkgCtx.DataStreams {
			sb.WriteString(fmt.Sprintf("- %s", ds.Name))
			if ds.Title != "" && ds.Title != ds.Name {
				sb.WriteString(fmt.Sprintf(" (%s)", ds.Title))
			}
			if ds.HasExampleEvent {
				sb.WriteString(" [has example]")
			}
			if ds.Description != "" {
				sb.WriteString(fmt.Sprintf(": %s", ds.Description))
			}
			sb.WriteString("\n")
			sb.WriteString(fmt.Sprintf("  → Use: {{fields \"%s\"}}", ds.Name))
			if ds.HasExampleEvent {
				sb.WriteString(fmt.Sprintf(" and {{event \"%s\"}}", ds.Name))
			}
			sb.WriteString("\n")
		}
	}

	// Service info content - filtered to relevant sections for this doc section
	if pkgCtx.ServiceInfo != "" {
		serviceInfoContent := getRelevantServiceInfo(pkgCtx.ServiceInfo, sectionTitle)
		if serviceInfoContent != "" {
			sb.WriteString("\n=== SERVICE INFO CONTENT (use this for context) ===\n")
			sb.WriteString(serviceInfoContent)
			sb.WriteString("\n")
		}
	}

	// Instructions
	sb.WriteString(buildInstructions())

	return sb.String()
}

// getRelevantServiceInfo extracts only the service info sections relevant to the given doc section
func getRelevantServiceInfo(fullServiceInfo string, sectionTitle string) string {
	// Get the mapping of which service_info sections are relevant for this doc section
	relevantSectionTitles := tools.GetServiceInfoMappingForSection(sectionTitle)

	// If no specific mapping, return empty (don't include all service info)
	if len(relevantSectionTitles) == 0 {
		return ""
	}

	// Parse the service info content
	sections := parsing.ParseSections(fullServiceInfo)
	if len(sections) == 0 {
		return ""
	}

	// Find and collect matching sections
	var matchedContent []string
	for _, requestedTitle := range relevantSectionTitles {
		section := parsing.FindSectionByTitleHierarchical(sections, requestedTitle)
		if section != nil {
			matchedContent = append(matchedContent, section.GetAllContent())
		}
	}

	if len(matchedContent) == 0 {
		return ""
	}

	return strings.Join(matchedContent, "\n\n")
}

// buildInstructions returns the final instructions for the generator
func buildInstructions() string {
	var sb strings.Builder
	sb.WriteString("\n=== INSTRUCTIONS ===\n")
	sb.WriteString("1. Use the EXACT section names shown above (## Overview, ## What data does this integration collect?, etc.)\n")
	sb.WriteString("2. Do NOT rename sections (e.g., don't use \"## Setup\" instead of \"## How do I deploy this integration?\")\n")
	sb.WriteString("3. Include ALL vendor documentation links - COPY URLS EXACTLY, do not modify them\n")
	sb.WriteString("4. Document ALL data streams listed above\n")
	sb.WriteString("5. Ensure heading hierarchy: # for title, ## for main sections, ### for subsections, #### for sub-subsections\n")
	sb.WriteString("6. In ## How do I deploy this integration?, add a '#### Vendor resources' subsection at the end of the vendor-specific setup subsection when vendor links are provided\n")
	sb.WriteString("7. In ## Reference section, use {{event \"<datastream_name>\"}} and {{fields \"<datastream_name>\"}} for EACH data stream (see DATA STREAMS section above for exact templates)\n")
	sb.WriteString("8. Address EVERY validation issue if any are listed above\n")
	sb.WriteString("9. For code blocks, always specify the language (e.g., ```bash, ```yaml)\n")
	sb.WriteString("10. Document ALL advanced settings with appropriate warnings (security, debug, SSL, etc.)\n")
	sb.WriteString("11. Use sentence case for headings (e.g., 'Vendor-side configuration' NOT 'Vendor-Side Configuration')\n")
	sb.WriteString("12. When showing example values like example.com, 10.0.0.1, or <placeholder>, add '(replace with your actual value)' or use format like `<your-hostname>`\n")
	sb.WriteString("13. Generate ONLY ONE H1 heading (the title) - all other headings should be H2 or lower\n")
	sb.WriteString("14. NEVER use # for code examples or configuration sections - use ### or #### instead\n")
	sb.WriteString("15. Heading levels must be sequential: H1 → H2 → H3 → H4 (never skip levels like H2 → H4)\n")
	sb.WriteString("16. In ## Troubleshooting, use Problem-Solution bullet format (NOT tables)\n")
	sb.WriteString("\n=== CONSISTENCY REQUIREMENTS ===\n")
	sb.WriteString("17. NEVER put bash comments (lines starting with #) outside code blocks - they will be parsed as H1 headings!\n")
	sb.WriteString("18. Use these EXACT subsection names in Troubleshooting:\n")
	sb.WriteString("    - '### Common configuration issues' (use Problem-Solution bullet format)\n")
	sb.WriteString("    - '### Vendor resources' (links to vendor documentation)\n")
	sb.WriteString("19. Use sentence case for ALL subsections (capitalize only first word): '### Vendor-specific issues' NOT '### Vendor-Specific Issues'\n")
	sb.WriteString("20. Under ## Reference, use:\n")
	sb.WriteString("    - '### Inputs used' (required)\n")
	sb.WriteString("    - '### API usage' (only for API-based integrations like httpjson)\n")
	sb.WriteString("    - '### Vendor documentation links' OR include links inline in relevant sections\n")
	sb.WriteString("21. All code blocks MUST have language specified: ```bash, ```yaml, ```json - NEVER use bare ``` blocks\n")
	return sb.String()
}
