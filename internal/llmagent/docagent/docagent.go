// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package docagent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/elastic/elastic-package/internal/llmagent/docagent/specialists"
	"github.com/elastic/elastic-package/internal/llmagent/docagent/specialists/validators"
	"github.com/elastic/elastic-package/internal/llmagent/docagent/workflow"
	"github.com/elastic/elastic-package/internal/llmagent/mcptools"
	"github.com/elastic/elastic-package/internal/llmagent/tools"
	"github.com/elastic/elastic-package/internal/llmagent/tracing"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/packages/archetype"
	"github.com/elastic/elastic-package/internal/profile"
	"github.com/elastic/elastic-package/internal/tui"
)

const (
	// How far back in the conversation ResponseAnalysis will consider
	analysisLookbackCount = 5
)

type responseStatus int

const (
	// responseSuccess indicates the LLM response is valid and successful
	responseSuccess responseStatus = iota
	// responseError indicates the LLM encountered an error
	responseError
	// responseEmpty indicates the response was empty (may or may not indicate an error)
	responseEmpty
)

type responseAnalyzer struct {
	successIndicators []string
	errorIndicators   []string
	errorMarkers      []string
}

// responseAnalysis contains the results of analyzing an LLM response
type responseAnalysis struct {
	Status  responseStatus
	Message string // Optional message explaining the status
}

// DocumentationAgent handles documentation updates for packages
type DocumentationAgent struct {
	executor              *Executor
	packageRoot           string
	repositoryRoot        *os.Root
	targetDocFile         string // Target documentation file (e.g., README.md, vpc.md)
	profile               *profile.Profile
	originalReadmeContent *string // Stores original content for restoration on cancel
	manifest              *packages.PackageManifest
	responseAnalyzer      *responseAnalyzer
	serviceInfoManager    *ServiceInfoManager
	parallelSections      bool // Whether to generate sections in parallel (default: true)
}

type PromptContext struct {
	Manifest        *packages.PackageManifest
	TargetDocFile   string
	Changes         string
	SectionTitle    string
	SectionLevel    int
	TemplateSection string
	ExampleSection  string
	PreserveContent string
}

// AgentConfig holds configuration for creating a DocumentationAgent
type AgentConfig struct {
	APIKey           string
	ModelID          string
	PackageRoot      string
	RepositoryRoot   *os.Root
	DocFile          string
	Profile          *profile.Profile
	ThinkingBudget   *int32         // Optional thinking budget for Gemini models
	TracingConfig    tracing.Config // Tracing configuration
	ParallelSections bool           // Whether to generate sections in parallel (default: true)
}

// NewDocumentationAgent creates a new documentation agent using ADK
func NewDocumentationAgent(ctx context.Context, cfg AgentConfig) (*DocumentationAgent, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("API key cannot be empty")
	}
	if cfg.PackageRoot == "" {
		return nil, fmt.Errorf("packageRoot cannot be empty")
	}
	if cfg.DocFile == "" {
		return nil, fmt.Errorf("targetDocFile cannot be empty")
	}

	// Initialize and load service info manager
	serviceInfoManager := NewServiceInfoManager(cfg.PackageRoot)
	// Attempt to load service_info (don't fail if it doesn't exist)
	_ = serviceInfoManager.Load()

	// Get package tools
	packageTools := tools.PackageTools(cfg.PackageRoot, serviceInfoManager)

	// Load MCP toolsets
	mcpToolsets := mcptools.LoadToolsets()

	// Create executor configuration with system instructions
	execCfg := ExecutorConfig{
		APIKey:         cfg.APIKey,
		ModelID:        cfg.ModelID,
		Instruction:    AgentInstructions,
		ThinkingBudget: cfg.ThinkingBudget,
		TracingConfig:  cfg.TracingConfig,
	}

	// Create executor with tools and toolsets
	executor, err := NewExecutorWithToolsets(ctx, execCfg, packageTools, mcpToolsets)
	if err != nil {
		return nil, fmt.Errorf("failed to create executor: %w", err)
	}

	manifest, err := packages.ReadPackageManifestFromPackageRoot(cfg.PackageRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to read package manifest: %w", err)
	}

	responseAnalyzer := NewResponseAnalyzer()
	return &DocumentationAgent{
		executor:           executor,
		packageRoot:        cfg.PackageRoot,
		repositoryRoot:     cfg.RepositoryRoot,
		targetDocFile:      cfg.DocFile,
		profile:            cfg.Profile,
		manifest:           manifest,
		responseAnalyzer:   responseAnalyzer,
		serviceInfoManager: serviceInfoManager,
		parallelSections:   cfg.ParallelSections,
	}, nil
}

// ConfirmInstructionsUnderstood asks the LLM to confirm it understood the system instructions.
// This should be called before any documentation workflow to ensure proper adherence.
func (d *DocumentationAgent) ConfirmInstructionsUnderstood(ctx context.Context) error {
	fmt.Println("🔍 Verifying LLM understands documentation guidelines...")

	confirmPrompt := `Please confirm that you understand and will follow all instructions provided in the system prompt for authoring Elastic documentation.

Briefly summarize the key principles you will adhere to:
1. The cumulative documentation model and applies_to mechanism
2. Voice and tone requirements  
3. Accessibility and inclusivity requirements

End your response with "CONFIRMED: I will follow all guidelines." if you understand.`

	result, err := d.executor.ExecuteTask(ctx, confirmPrompt)
	if err != nil {
		return fmt.Errorf("failed to confirm instructions: %w", err)
	}

	// Log the confirmation response
	logger.Debugf("LLM confirmation response: %s", result.FinalContent)

	// Check if the LLM confirmed understanding
	if !strings.Contains(strings.ToLower(result.FinalContent), "confirmed") {
		return fmt.Errorf("LLM did not confirm understanding of documentation guidelines")
	}

	fmt.Println("✅ LLM confirmed understanding of documentation guidelines")
	return nil
}

// UpdateDocumentation runs the documentation update process using the shared generation + validation loop
func (d *DocumentationAgent) UpdateDocumentation(ctx context.Context, nonInteractive bool) error {
	return d.UpdateDocumentationWithConfig(ctx, nonInteractive, DefaultGenerationConfig())
}

// UpdateDocumentationWithConfig runs documentation update with custom configuration
// Uses section-based generation where each section has its own generate-validate loop
func (d *DocumentationAgent) UpdateDocumentationWithConfig(ctx context.Context, nonInteractive bool, genCfg GenerationConfig) error {
	ctx, sessionSpan := tracing.StartSessionSpan(ctx, "doc:generate", d.executor.ModelID())
	var sessionOutput string
	defer func() {
		tracing.EndSessionSpan(ctx, sessionSpan, sessionOutput)
	}()

	// Output session ID for trace retrieval (only when tracing is enabled)
	if tracing.IsEnabled() {
		if sessionID, ok := tracing.SessionIDFromContext(ctx); ok {
			fmt.Printf("🔍 Tracing session ID: %s\n", sessionID)
		}
	} else {
		panic("tracing is not enabled")
	}

	// Record the input request
	tracing.RecordSessionInput(sessionSpan, fmt.Sprintf("Generate documentation for package: %s (file: %s)", d.manifest.Name, d.targetDocFile))

	// Confirm LLM understands the documentation guidelines before proceeding
	if err := d.ConfirmInstructionsUnderstood(ctx); err != nil {
		return fmt.Errorf("instruction confirmation failed: %w", err)
	}

	// Backup original README content before making any changes
	d.backupOriginalReadme()

	// Load package context for validation
	pkgCtx, err := validators.LoadPackageContext(d.packageRoot)
	if err != nil {
		return fmt.Errorf("failed to load package context: %w", err)
	}

	// Generate sections using section-based approach with per-section validation
	fmt.Printf("📊 Starting section-based generation (max %d iterations per section)...\n", genCfg.MaxIterations)
	result, err := d.GenerateAllSectionsWithValidation(ctx, pkgCtx, genCfg)
	if err != nil {
		return fmt.Errorf("failed to generate documentation: %w", err)
	}

	sessionOutput = fmt.Sprintf("Generated %d sections, %d characters for %s", len(result.SectionResults), len(result.Content), d.targetDocFile)

	// Write the generated document
	docPath := filepath.Join(d.packageRoot, "_dev", "build", "docs", d.targetDocFile)
	if err := d.writeDocumentation(docPath, result.Content); err != nil {
		return fmt.Errorf("failed to write documentation: %w", err)
	}

	// Count sections for display
	sections := ParseSections(result.Content)
	approvedStr := ""
	if result.Approved {
		approvedStr = " ✓ validated"
	} else {
		approvedStr = " ⚠ validation issues remain"
	}
	fmt.Printf("\n✅ Documentation generated successfully! (%d sections, %d characters%s)\n",
		len(sections), len(result.Content), approvedStr)
	fmt.Printf("📄 Written to: _dev/build/docs/%s\n", d.targetDocFile)

	// Show per-section summary
	fmt.Println("📊 Per-section summary:")
	for _, sr := range result.SectionResults {
		status := "✅"
		if !sr.Approved {
			status = "⚠️"
		}
		fmt.Printf("  %s %s: %d iterations, best=%d (%d chars)\n",
			status, sr.SectionTitle, sr.TotalIterations, sr.BestIteration, len(sr.Content))
	}

	// In interactive mode, allow review
	if !nonInteractive {
		return d.runInteractiveSectionReview(ctx, sections)
	}

	return nil
}

// ModifyDocumentation runs the documentation modification process for targeted changes using section-based approach
func (d *DocumentationAgent) ModifyDocumentation(ctx context.Context, nonInteractive bool, modifyPrompt string) error {
	ctx, sessionSpan := tracing.StartSessionSpan(ctx, "doc:modify", d.executor.ModelID())
	var sessionOutput string
	defer func() {
		tracing.EndSessionSpan(ctx, sessionSpan, sessionOutput)
	}()

	// Output session ID for trace retrieval (only when tracing is enabled)
	if tracing.IsEnabled() {
		if sessionID, ok := tracing.SessionIDFromContext(ctx); ok {
			fmt.Printf("🔍 Tracing session ID: %s\n", sessionID)
		}
	}

	// Check if documentation file exists
	docPath := filepath.Join(d.packageRoot, "_dev", "build", "docs", d.targetDocFile)
	if _, err := os.Stat(docPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("cannot modify documentation: %s does not exist at _dev/build/docs/%s", d.targetDocFile, d.targetDocFile)
		}
		return fmt.Errorf("failed to check %s: %w", d.targetDocFile, err)
	}

	// Record initial input for session
	inputDesc := fmt.Sprintf("Modify documentation for package: %s (file: %s)", d.manifest.Name, d.targetDocFile)
	if modifyPrompt != "" {
		inputDesc += fmt.Sprintf(" - Request: %s", modifyPrompt)
	}
	tracing.RecordSessionInput(sessionSpan, inputDesc)

	// Confirm LLM understands the documentation guidelines before proceeding
	if err := d.ConfirmInstructionsUnderstood(ctx); err != nil {
		return fmt.Errorf("instruction confirmation failed: %w", err)
	}

	// Backup original README content before making any changes
	d.backupOriginalReadme()

	// Get modification instructions if not provided
	var instructions string
	if modifyPrompt != "" {
		instructions = modifyPrompt
	} else if !nonInteractive {
		// Prompt user for modification instructions
		var err error
		instructions, err = tui.AskTextArea("What changes would you like to make to the documentation?")
		if err != nil {
			// Check if user cancelled
			if errors.Is(err, tui.ErrCancelled) {
				fmt.Println("⚠️  Modification cancelled.")
				return nil
			}
			return fmt.Errorf("prompt failed: %w", err)
		}

		// Check if no changes were provided
		if strings.TrimSpace(instructions) == "" {
			return fmt.Errorf("no modification instructions provided")
		}
	} else {
		return fmt.Errorf("--modify-prompt flag is required in non-interactive mode")
	}

	fmt.Println("📝 Analyzing modification request...")

	// Parse existing documentation into sections
	existingContent, err := d.readCurrentReadme()
	if err != nil {
		return fmt.Errorf("failed to read current documentation: %w", err)
	}
	existingSections := ParseSections(existingContent)

	if len(existingSections) == 0 {
		return fmt.Errorf("no sections found in existing documentation")
	}

	// Get template sections for reference (structure)
	templateContent := archetype.GetPackageDocsReadmeTemplate()
	templateSections := ParseSections(templateContent)

	// Analyze modification scope
	scope, err := d.analyzeModificationScope(ctx, instructions, templateSections)
	if err != nil {
		logger.Debugf("Scope analysis failed, defaulting to global: %v", err)
		scope = &ModificationScope{
			Type:       ScopeGlobal,
			Confidence: 0.5,
			Reasoning:  "Scope analysis failed, defaulting to global",
		}
	}

	// Report scope to user
	fmt.Printf("✓ Scope: %s", scope.Type)
	if scope.Type == ScopeSpecific {
		fmt.Printf(" (sections: %s)", strings.Join(scope.AffectedSections, ", "))
	}
	if scope.Confidence < 0.7 {
		fmt.Printf(" [confidence: %.0f%%]", scope.Confidence*100)
	}
	fmt.Println()
	if scope.Reasoning != "" {
		logger.Debugf("Scope reasoning: %s", scope.Reasoning)
	}

	// Apply modifications based on scope
	var finalSections []Section

	switch scope.Type {
	case ScopeGlobal, ScopeAmbiguous:
		if scope.Type == ScopeAmbiguous {
			fmt.Println("⚠️  Scope is ambiguous, modifying all sections to be safe")
		}
		fmt.Printf("📝 Modifying all %d sections...\n", len(existingSections))
		finalSections, err = d.modifyAllSections(ctx, existingSections, instructions)
		if err != nil {
			return fmt.Errorf("failed to modify sections: %w", err)
		}

	case ScopeSpecific:
		fmt.Printf("📝 Modifying %d of %d sections...\n", len(scope.AffectedSections), len(existingSections))
		finalSections, err = d.modifySpecificSections(ctx, existingSections, scope.AffectedSections, instructions)
		if err != nil {
			return fmt.Errorf("failed to modify sections: %w", err)
		}
	}

	// Combine and write
	finalContent := CombineSections(finalSections)
	sessionOutput = fmt.Sprintf("Modified %d sections, %d characters for %s", len(finalSections), len(finalContent), d.targetDocFile)

	if err := d.writeDocumentation(docPath, finalContent); err != nil {
		return fmt.Errorf("failed to write documentation: %w", err)
	}

	fmt.Printf("\n✅ Documentation modified successfully! (%d characters)\n", len(finalContent))
	fmt.Printf("📄 Written to: _dev/build/docs/%s\n", d.targetDocFile)

	// In interactive mode, allow review
	if !nonInteractive {
		return d.runInteractiveSectionReview(ctx, finalSections)
	}

	return nil
}

// logAgentResponse logs debug information about the agent response
func (d *DocumentationAgent) logAgentResponse(result *TaskResult) {
	logger.Debugf("DEBUG: Full agent task response follows (may contain sensitive content)")
	logger.Debugf("Agent task response - Success: %t", result.Success)
	logger.Debugf("Agent task response - FinalContent: %s", result.FinalContent)
	logger.Debugf("Agent task response - Conversation entries: %d", len(result.Conversation))
	for i, entry := range result.Conversation {
		logger.Debugf("Agent task response - Conversation[%d]: type=%s, content_length=%d",
			i, entry.Type, len(entry.Content))
		logger.Tracef("Agent task response - Conversation[%d]: content=%s", i, entry.Content)
	}
}

// executeTaskWithLogging executes a task and logs the result
func (d *DocumentationAgent) executeTaskWithLogging(ctx context.Context, prompt string) (*TaskResult, error) {
	fmt.Println("🤖 LLM Agent is working...")

	result, err := d.executor.ExecuteTask(ctx, prompt)
	if err != nil {
		fmt.Println("❌ Agent task failed")
		fmt.Printf("❌ result is %v\n", result)
		return nil, fmt.Errorf("agent task failed: %w", err)
	}

	fmt.Println("✅ Task completed")
	d.logAgentResponse(result)
	return result, nil
}

// NewResponseAnalyzer creates a new ResponseAnalyzer with default patterns
//
// These responses should be chosen to represent LLM responses to states, but are unlikely to appear in generated
// documentation, which could trigger false positives.
func NewResponseAnalyzer() *responseAnalyzer {
	return &responseAnalyzer{
		successIndicators: []string{
			"✅ success",
			"successfully wrote",
			"completed successfully",
		},
		errorIndicators: []string{
			"I encountered an error",
			"I'm experiencing an error",
			"I cannot complete",
			"I'm unable to complete",
			"Something went wrong",
			"There was an error",
			"I'm having trouble",
			"I failed to",
			"Error occurred",
			"Task did not complete within maximum iterations",
		},
		errorMarkers: []string{
			"❌ error",
			"failed:",
		},
	}
}

// AnalyzeResponse will detect the LLM state based on it's response to us.
func (ra *responseAnalyzer) AnalyzeResponse(content string, conversation []ConversationEntry) responseAnalysis {
	// Check for empty content
	if strings.TrimSpace(content) == "" {
		// Empty content might be okay if recent tools succeeded
		if conversation != nil && ra.hasRecentSuccessfulTools(conversation) {
			return responseAnalysis{
				Status:  responseSuccess,
				Message: "Empty response after successful tool execution",
			}
		}
		return responseAnalysis{
			Status:  responseEmpty,
			Message: "Empty response without tool success context",
		}
	}

	// Check for error indicators
	if ra.containsAnyIndicator(content, ra.errorIndicators) {
		// However, if recent tools succeeded, this might be a false error report
		if conversation != nil && ra.hasRecentSuccessfulTools(conversation) {
			return responseAnalysis{
				Status:  responseSuccess,
				Message: "Error message detected but recent tools succeeded (likely false error)",
			}
		}
		return responseAnalysis{
			Status:  responseError,
			Message: "LLM reported an error",
		}
	}

	// Default: success
	return responseAnalysis{
		Status:  responseSuccess,
		Message: "Normal response",
	}
}

// containsAnyIndicator checks if content contains any of the given indicators (case-insensitive)
func (ra *responseAnalyzer) containsAnyIndicator(content string, indicators []string) bool {
	contentLower := strings.ToLower(content)
	for _, indicator := range indicators {
		if strings.Contains(contentLower, strings.ToLower(indicator)) {
			return true
		}
	}
	return false
}

// hasRecentSuccessfulTools checks if recent tool executions were successful
func (ra *responseAnalyzer) hasRecentSuccessfulTools(conversation []ConversationEntry) bool {
	// Look at the last 5 conversation entries for tool results
	lookbackCount := analysisLookbackCount
	startIdx := len(conversation) - lookbackCount
	if startIdx < 0 {
		startIdx = 0
	}

	for i := len(conversation) - 1; i >= startIdx; i-- {
		entry := conversation[i]
		if entry.Type == "tool_result" {
			// Check for success indicators first
			if ra.containsAnyIndicator(entry.Content, ra.successIndicators) {
				return true
			}

			// If we hit an actual error marker, stop looking
			if ra.containsAnyIndicator(entry.Content, ra.errorMarkers) {
				return false
			}
		}
	}
	return false
}

// buildSectionPrompt builds a prompt for generating a single section
func (d *DocumentationAgent) buildSectionPrompt(sectionCtx SectionGenerationContext) string {
	// Create a prompt context with section-specific information
	promptCtx := PromptContext{
		Manifest:      sectionCtx.PackageInfo.Manifest,
		TargetDocFile: sectionCtx.PackageInfo.TargetDocFile,
		SectionTitle:  sectionCtx.Section.Title,
		SectionLevel:  sectionCtx.Section.Level,
	}

	// Add template section content - use FullContent to include subsections
	if sectionCtx.TemplateSection != nil {
		promptCtx.TemplateSection = sectionCtx.TemplateSection.GetAllContent()
	} else {
		promptCtx.TemplateSection = "No template section available for this section."
	}

	// Add example section content - use FullContent to include subsections
	if sectionCtx.ExampleSection != nil {
		promptCtx.ExampleSection = sectionCtx.ExampleSection.GetAllContent()
	} else {
		promptCtx.ExampleSection = "No example section available for this section."
	}

	// Add preserve content if any
	if sectionCtx.Section.HasPreserve {
		promptCtx.PreserveContent = sectionCtx.Section.PreserveContent
	}

	return d.buildPrompt(PromptTypeSectionGeneration, promptCtx)
}

// writeDocumentation writes the documentation content to a file
func (d *DocumentationAgent) writeDocumentation(path, content string) error {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Write the file
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// runInteractiveSectionReview allows user to review and request changes in interactive mode
func (d *DocumentationAgent) runInteractiveSectionReview(ctx context.Context, sections []Section) error {
	// Display the generated documentation
	if err := d.displayReadme(); err != nil {
		logger.Debugf("could not display readme: %v", err)
	}

	// Get user action
	action, err := d.getUserAction()
	if err != nil {
		return err
	}

	readmeUpdated := true // We just wrote it
	actionResult := d.handleUserAction(action, readmeUpdated)
	if actionResult.Err != nil {
		return actionResult.Err
	}

	// If user requests changes, fall back to the modify workflow
	if actionResult.ShouldContinue {
		fmt.Println("For changes to section-based documentation, please use the modify mode.")
		fmt.Println("Run: elastic-package update documentation --modify-prompt \"your changes\"")
		return nil
	}

	return nil
}

// modifyAllSections regenerates all sections with modification context
func (d *DocumentationAgent) modifyAllSections(ctx context.Context, existingSections []Section, modificationPrompt string) ([]Section, error) {
	var modifiedSections []Section

	for i, section := range existingSections {
		fmt.Printf("📝 Modifying section %d/%d: %s\n", i+1, len(existingSections), section.Title)

		// Build modification prompt for this section
		promptCtx := PromptContext{
			Manifest:        d.manifest,
			TargetDocFile:   d.targetDocFile,
			Changes:         modificationPrompt,
			SectionTitle:    section.Title,
			SectionLevel:    section.Level,
			TemplateSection: section.Content,
		}

		if section.HasPreserve {
			promptCtx.PreserveContent = section.PreserveContent
		}

		prompt := d.buildPrompt(PromptTypeModification, promptCtx)

		// Generate modified section
		modifiedSection, err := d.generateModifiedSection(ctx, section, prompt)
		if err != nil {
			logger.Debugf("Failed to modify section %s: %v", section.Title, err)
			// On error, keep the original section
			modifiedSections = append(modifiedSections, section)
			continue
		}

		modifiedSections = append(modifiedSections, modifiedSection)
	}

	return modifiedSections, nil
}

// modifySpecificSections regenerates only affected sections
// For hierarchical sections, if a subsection is affected, the entire parent section is regenerated
func (d *DocumentationAgent) modifySpecificSections(ctx context.Context, existingSections []Section, affectedSectionTitles []string, modificationPrompt string) ([]Section, error) {
	var finalSections []Section
	modifiedCount := 0

	for _, section := range existingSections {
		// Check if this section or any of its subsections are affected
		isAffected := isSectionAffected(section.Title, affectedSectionTitles)

		// Check subsections - if any subsection is affected, modify the parent
		if !isAffected {
			for _, subsection := range section.Subsections {
				if isSectionAffected(subsection.Title, affectedSectionTitles) {
					isAffected = true
					logger.Debugf("Subsection %s is affected, will regenerate parent section %s", subsection.Title, section.Title)
					break
				}
			}
		}

		if isAffected {
			modifiedCount++
			fmt.Printf("📝 Modifying section %d/%d: %s", modifiedCount, len(affectedSectionTitles), section.Title)
			if section.HasSubsections() {
				fmt.Printf(" (with %d subsections)", len(section.Subsections))
			}
			fmt.Println()

			// Build modification prompt for this section (use FullContent for hierarchical context)
			promptCtx := PromptContext{
				Manifest:        d.manifest,
				TargetDocFile:   d.targetDocFile,
				Changes:         modificationPrompt,
				SectionTitle:    section.Title,
				SectionLevel:    section.Level,
				TemplateSection: section.GetAllContent(), // Include subsections in context
			}

			if section.HasPreserve {
				promptCtx.PreserveContent = section.PreserveContent
			}

			prompt := d.buildPrompt(PromptTypeModification, promptCtx)

			// Generate modified section (includes subsections)
			modifiedSection, err := d.generateModifiedSection(ctx, section, prompt)
			if err != nil {
				logger.Debugf("Failed to modify section %s: %v", section.Title, err)
				// On error, keep the original section
				finalSections = append(finalSections, section)
				continue
			}

			// Parse the generated content to extract hierarchical structure
			parsedModified := ParseSections(modifiedSection.Content)
			if len(parsedModified) > 0 {
				modifiedSection = parsedModified[0] // Take the full hierarchical section
			}

			finalSections = append(finalSections, modifiedSection)
		} else {
			// Preserve entire section unchanged (including subsections)
			finalSections = append(finalSections, section)
		}
	}

	preservedCount := len(existingSections) - modifiedCount
	fmt.Printf("✓ Modified: %d sections, Preserved: %d sections\n", modifiedCount, preservedCount)

	return finalSections, nil
}

// generateModifiedSection generates a modified version of a section using the LLM
func (d *DocumentationAgent) generateModifiedSection(ctx context.Context, originalSection Section, prompt string) (Section, error) {
	// Execute the task
	result, err := d.executor.ExecuteTask(ctx, prompt)
	if err != nil {
		return Section{}, fmt.Errorf("agent task failed: %w", err)
	}

	// Log the result
	d.logAgentResponse(result)

	// Analyze the response
	analysis := d.responseAnalyzer.AnalyzeResponse(result.FinalContent, result.Conversation)
	if analysis.Status == responseError {
		return Section{}, fmt.Errorf("LLM reported an error: %s", analysis.Message)
	}

	// Extract the generated content
	generatedContent := d.extractGeneratedSectionContent(result, originalSection.Title)

	// Create the modified section
	modifiedSection := Section{
		Title:           originalSection.Title,
		Level:           originalSection.Level,
		Content:         generatedContent,
		HasPreserve:     originalSection.HasPreserve,
		PreserveContent: originalSection.PreserveContent,
		StartLine:       originalSection.StartLine,
		EndLine:         originalSection.EndLine,
	}

	return modifiedSection, nil
}

// sectionResult holds the result of generating a single section
type sectionResult struct {
	index   int
	section Section
	err     error
}

// GenerateAllSectionsWithWorkflow generates all sections using the multi-agent workflow.
// This method uses a configurable pipeline of agents (generator, critic, validator, etc.)
// to iteratively refine each section. By default sections are generated in parallel,
// but this can be changed via the ParallelSections config option.
func (d *DocumentationAgent) GenerateAllSectionsWithWorkflow(ctx context.Context, workflowCfg workflow.Config) ([]Section, error) {
	ctx, chainSpan := tracing.StartChainSpan(ctx, "doc:generate:workflow")
	defer func() {
		// Flush all child spans before ending the chain span to ensure proper trace hierarchy
		if err := tracing.ForceFlush(ctx); err != nil {
			logger.Debugf("Failed to flush traces before ending chain span: %v", err)
		}
		chainSpan.End()
	}()

	// Get the template content
	templateContent := archetype.GetPackageDocsReadmeTemplate()

	// Get the example content
	exampleContent := tools.GetDefaultExampleContent()

	// Parse sections from template
	templateSections := ParseSections(templateContent)
	if len(templateSections) == 0 {
		return nil, fmt.Errorf("no sections found in template")
	}

	// Parse sections from example
	exampleSections := ParseSections(exampleContent)

	// Read existing documentation if it exists
	existingContent, _ := d.readCurrentReadme()
	var existingSections []Section
	if existingContent != "" {
		existingSections = ParseSections(existingContent)
	}

	// Collect top-level sections to generate
	var topLevelSections []Section
	for _, s := range templateSections {
		if s.IsTopLevel() {
			topLevelSections = append(topLevelSections, s)
		}
	}

	if len(topLevelSections) == 0 {
		return nil, fmt.Errorf("no top-level sections found in template")
	}

	// Choose parallel or sequential execution based on config
	if d.parallelSections {
		return d.generateSectionsParallel(ctx, workflowCfg, topLevelSections, exampleSections, existingSections)
	}
	return d.generateSectionsSequential(ctx, workflowCfg, topLevelSections, exampleSections, existingSections)
}

// generateSectionsParallel generates all sections in parallel using goroutines
func (d *DocumentationAgent) generateSectionsParallel(ctx context.Context, workflowCfg workflow.Config, topLevelSections, exampleSections, existingSections []Section) ([]Section, error) {
	fmt.Printf("📝 Generating %d sections in parallel...\n", len(topLevelSections))

	// Create channel to collect results
	resultsChan := make(chan sectionResult, len(topLevelSections))

	// Use WaitGroup to track goroutines
	var wg sync.WaitGroup

	// Generate sections in parallel
	for idx, templateSection := range topLevelSections {
		wg.Add(1)
		go func(index int, tmplSection Section) {
			defer wg.Done()
			result := d.generateSingleSection(ctx, workflowCfg, index, tmplSection, exampleSections, existingSections)
			resultsChan <- result
		}(idx, templateSection)
	}

	// Wait for all goroutines to complete, then close channel
	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	// Collect results and maintain order
	results := make([]sectionResult, len(topLevelSections))
	successCount := 0
	failCount := 0

	for result := range resultsChan {
		results[result.index] = result
		if result.err != nil {
			failCount++
			fmt.Printf("  ❌ Section %d: %s (failed)\n", result.index+1, result.section.Title)
		} else {
			successCount++
			fmt.Printf("  ✅ Section %d: %s (done)\n", result.index+1, result.section.Title)
		}
	}

	fmt.Printf("📊 Generated %d/%d sections successfully\n", successCount, len(topLevelSections))

	// Extract sections in order
	generatedSections := make([]Section, len(topLevelSections))
	for i, r := range results {
		generatedSections[i] = r.section
	}

	return generatedSections, nil
}

// GenerateFullDocumentWithWorkflow generates the entire document in a single pass
// This approach produces coherent output without duplicate sections that can occur
// when generating sections in parallel
func (d *DocumentationAgent) GenerateFullDocumentWithWorkflow(ctx context.Context, workflowCfg workflow.Config) (string, error) {
	// Load package context for rich context
	pkgCtx, err := validators.LoadPackageContext(d.packageRoot)
	if err != nil {
		return "", fmt.Errorf("failed to load package context: %w", err)
	}

	// Build section context for full document generation
	sectionCtx := validators.SectionContext{
		PackageName:  d.manifest.Name,
		PackageTitle: d.manifest.Title,
		SectionTitle: "Full README",
		SectionLevel: 1,
	}

	// Add existing content as reference (if available)
	if pkgCtx.ExistingReadme != "" {
		sectionCtx.ExistingContent = pkgCtx.ExistingReadme
	}

	// Build rich context with all package information (using shared context builder)
	sectionCtx.AdditionalContext = workflow.BuildHeadStartContext(pkgCtx, nil)

	// Execute workflow
	builder := workflow.NewBuilder(workflowCfg)
	result, err := builder.ExecuteWorkflow(ctx, sectionCtx)
	if err != nil {
		return "", fmt.Errorf("workflow execution failed: %w", err)
	}

	return result.Content, nil
}

// NOTE: buildHeadStartContext has been consolidated into workflow.BuildHeadStartContext
// This ensures consistent context building across regular and --evaluate modes

// GenerateAllSectionsWithValidation generates all sections using per-section validation loops
// Each section gets its own generate-validate iteration cycle with best-iteration tracking
func (d *DocumentationAgent) GenerateAllSectionsWithValidation(ctx context.Context, pkgCtx *validators.PackageContext, genCfg GenerationConfig) (*GenerationResult, error) {
	ctx, chainSpan := tracing.StartChainSpan(ctx, "doc:generate:sections")
	defer func() {
		// Flush all child spans before ending the chain span to ensure proper trace hierarchy
		if err := tracing.ForceFlush(ctx); err != nil {
			logger.Debugf("Failed to flush traces before ending chain span: %v", err)
		}
		chainSpan.End()
	}()

	// Get the template content
	templateContent := archetype.GetPackageDocsReadmeTemplate()

	// Get the example content
	exampleContent := tools.GetDefaultExampleContent()

	// Parse sections from template
	templateSections := ParseSections(templateContent)
	if len(templateSections) == 0 {
		return nil, fmt.Errorf("no sections found in template")
	}

	// Parse sections from example
	exampleSections := ParseSections(exampleContent)

	// Read existing documentation if it exists
	existingContent, _ := d.readCurrentReadme()
	var existingSections []Section
	if existingContent != "" {
		existingSections = ParseSections(existingContent)
	}

	// Collect top-level sections to generate
	var topLevelSections []Section
	for _, s := range templateSections {
		if s.IsTopLevel() {
			topLevelSections = append(topLevelSections, s)
		}
	}

	if len(topLevelSections) == 0 {
		return nil, fmt.Errorf("no top-level sections found in template")
	}

	// Build rich context for all sections
	headStartContext := workflow.BuildHeadStartContext(pkgCtx, nil)

	fmt.Printf("📝 Generating %d sections with per-section validation...\n", len(topLevelSections))

	// Channel to collect results
	type sectionGenResult struct {
		index  int
		result *SectionGenerationResult
		err    error
	}
	resultsChan := make(chan sectionGenResult, len(topLevelSections))

	// Use WaitGroup to track goroutines
	var wg sync.WaitGroup

	// Generate sections in parallel with per-section validation loops
	for idx, templateSection := range topLevelSections {
		wg.Add(1)
		go func(index int, tmplSection Section) {
			defer wg.Done()

			// Find corresponding example section
			exampleSection := FindSectionByTitle(exampleSections, tmplSection.Title)

			// Find existing section
			var existingSection *Section
			if len(existingSections) > 0 {
				existingSection = FindSectionByTitle(existingSections, tmplSection.Title)
			}

			// Build section context for workflow
			sectionCtx := validators.SectionContext{
				SectionTitle:      tmplSection.Title,
				SectionLevel:      tmplSection.Level,
				PackageName:       d.manifest.Name,
				PackageTitle:      d.manifest.Title,
				AdditionalContext: headStartContext,
			}

			if tmplSection.Content != "" {
				sectionCtx.TemplateContent = tmplSection.GetAllContent()
			}
			if exampleSection != nil {
				sectionCtx.ExampleContent = exampleSection.GetAllContent()
			}
			if existingSection != nil {
				sectionCtx.ExistingContent = existingSection.GetAllContent()
			}

			// Generate section with validation loop
			sectionResult, err := d.GenerateSectionWithValidationLoop(ctx, sectionCtx, pkgCtx, genCfg)
			if err != nil {
				logger.Debugf("Section generation failed for %s: %v", tmplSection.Title, err)
				resultsChan <- sectionGenResult{
					index: index,
					err:   err,
				}
				return
			}

			resultsChan <- sectionGenResult{
				index:  index,
				result: sectionResult,
			}
		}(idx, templateSection)
	}

	// Wait for all goroutines to complete, then close channel
	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	// Collect results and maintain order
	results := make([]*SectionGenerationResult, len(topLevelSections))
	successCount := 0
	failCount := 0
	allApproved := true

	for res := range resultsChan {
		if res.err != nil {
			failCount++
			fmt.Printf("  ❌ Section %d: %s (failed: %v)\n", res.index+1, topLevelSections[res.index].Title, res.err)
			// Create placeholder result for failed sections
			results[res.index] = &SectionGenerationResult{
				SectionTitle: topLevelSections[res.index].Title,
				SectionLevel: topLevelSections[res.index].Level,
				Content:      fmt.Sprintf("## %s\n\n%s", topLevelSections[res.index].Title, emptySectionPlaceholder),
				Approved:     false,
			}
			allApproved = false
		} else {
			successCount++
			results[res.index] = res.result
			status := "✅"
			if !res.result.Approved {
				status = "⚠️"
				allApproved = false
			}
			fmt.Printf("  %s Section %d: %s (iterations=%d, best=%d)\n",
				status, res.index+1, res.result.SectionTitle, res.result.TotalIterations, res.result.BestIteration)
		}
	}

	fmt.Printf("📊 Generated %d/%d sections successfully\n", successCount, len(topLevelSections))

	// Flush spans from parallel goroutines before returning to ensure they're exported
	if err := tracing.ForceFlush(ctx); err != nil {
		logger.Debugf("Failed to flush traces after parallel section generation: %v", err)
	}

	// Convert section results to Section structs for combining
	var generatedSections []Section
	var sectionResults []SectionGenerationResult
	for _, sr := range results {
		if sr != nil {
			// Parse the content to get proper Section structure
			parsedSections := ParseSections(sr.Content)
			if len(parsedSections) > 0 {
				generatedSections = append(generatedSections, parsedSections[0])
			} else {
				// Fallback: create section from content directly
				generatedSections = append(generatedSections, Section{
					Title:   sr.SectionTitle,
					Level:   sr.SectionLevel,
					Content: sr.Content,
				})
			}
			sectionResults = append(sectionResults, *sr)
		}
	}

	// Combine sections into final document
	finalContent := CombineSections(generatedSections)

	// Calculate total iterations (sum across all sections)
	totalIterations := 0
	for _, sr := range sectionResults {
		totalIterations += sr.TotalIterations
	}

	return &GenerationResult{
		Content:         finalContent,
		Approved:        allApproved,
		TotalIterations: totalIterations,
		SectionResults:  sectionResults,
	}, nil
}

// GenerationConfig holds configuration for the generation + validation loop
type GenerationConfig struct {
	// MaxIterations is the maximum number of generation-validation iterations (default: 3)
	MaxIterations uint
	// EnableStagedValidation enables validation after each generation
	EnableStagedValidation bool
	// EnableLLMValidation enables LLM-based semantic validation in addition to static checks
	EnableLLMValidation bool
	// SnapshotManager for saving iteration snapshots (optional)
	SnapshotManager *workflow.SnapshotManager
}

// DefaultGenerationConfig returns default configuration for generation
func DefaultGenerationConfig() GenerationConfig {
	return GenerationConfig{
		MaxIterations:          3,
		EnableStagedValidation: true,
	}
}

// SectionGenerationResult holds the result of generating a single section with validation
type SectionGenerationResult struct {
	// SectionTitle is the title of the section
	SectionTitle string
	// SectionLevel is the heading level (2 = ##, 3 = ###, etc.)
	SectionLevel int
	// Content is the best generated content for this section
	Content string
	// Approved indicates if all validation stages passed for this section
	Approved bool
	// TotalIterations is the number of iterations performed for this section
	TotalIterations int
	// BestIteration is the iteration that produced the best content
	BestIteration int
	// IssueHistory tracks issue counts per iteration for this section
	IssueHistory []int
}

// GenerationResult holds the result of the generation + validation loop
type GenerationResult struct {
	// Content is the final generated documentation
	Content string
	// Approved indicates if all validation stages passed
	Approved bool
	// TotalIterations is the total number of iterations across all sections
	TotalIterations int
	// BestIteration is the iteration number that produced the best content (may differ from TotalIterations if later iterations regressed)
	BestIteration int
	// SectionResults holds per-section generation results
	SectionResults []SectionGenerationResult
	// StageResults holds per-stage validation results (uses StageResult from evaluation.go)
	StageResults map[string]*StageResult
	// ValidationFeedback contains the last validation feedback (if any)
	ValidationFeedback string
	// IssueHistory tracks issue counts per iteration (for convergence analysis)
	IssueHistory []int
	// ConvergenceBonus indicates if an extra iteration was granted due to convergence
	ConvergenceBonus bool
}

// DocumentSection represents a section of the documentation
type DocumentSection struct {
	Title   string // Section title (e.g., "## Troubleshooting")
	Level   int    // Heading level (1 = #, 2 = ##, etc.)
	Content string // Full section content including title and body
	Length  int    // Length of content in characters
}

// parseDocumentIntoSections parses a markdown document into sections
func parseDocumentIntoSections(content string) map[string]*DocumentSection {
	sections := make(map[string]*DocumentSection)
	lines := strings.Split(content, "\n")

	var currentSection *DocumentSection
	var currentContent strings.Builder
	var currentTitle string

	for i, line := range lines {
		// Check if this is a heading
		if strings.HasPrefix(line, "#") {
			// Save previous section
			if currentSection != nil {
				currentSection.Content = currentContent.String()
				currentSection.Length = len(currentSection.Content)
				sections[currentTitle] = currentSection
			}

			// Determine heading level
			level := 0
			for _, ch := range line {
				if ch == '#' {
					level++
				} else {
					break
				}
			}

			// Extract title (normalized for comparison)
			title := strings.TrimSpace(strings.TrimLeft(line, "# "))
			normalizedTitle := strings.ToLower(title)

			currentSection = &DocumentSection{
				Title: title,
				Level: level,
			}
			currentTitle = normalizedTitle
			currentContent.Reset()
			currentContent.WriteString(line)
			currentContent.WriteString("\n")
		} else if currentSection != nil {
			currentContent.WriteString(line)
			if i < len(lines)-1 {
				currentContent.WriteString("\n")
			}
		}
	}

	// Save last section
	if currentSection != nil {
		currentSection.Content = currentContent.String()
		currentSection.Length = len(currentSection.Content)
		sections[currentTitle] = currentSection
	}

	return sections
}

// isSectionBetter determines if newSection is better than oldSection
// Better means: more detailed (longer) content with substantive information
func isSectionBetter(newSection, oldSection *DocumentSection) bool {
	if oldSection == nil {
		return true
	}
	if newSection == nil {
		return false
	}

	// Significantly longer content is better (20% or more)
	if newSection.Length > oldSection.Length*12/10 {
		return true
	}

	// Slightly shorter but within 10% is acceptable if it has more structure
	// (more subsections, bullet points, tables)
	if newSection.Length >= oldSection.Length*9/10 {
		newStructure := countStructuralElements(newSection.Content)
		oldStructure := countStructuralElements(oldSection.Content)
		if newStructure > oldStructure {
			return true
		}
	}

	return false
}

// countStructuralElements counts structural elements in content
func countStructuralElements(content string) int {
	count := 0
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Count bullet points
		if strings.HasPrefix(trimmed, "* ") || strings.HasPrefix(trimmed, "- ") {
			count++
		}
		// Count numbered items
		if len(trimmed) > 2 && trimmed[0] >= '1' && trimmed[0] <= '9' && trimmed[1] == '.' {
			count++
		}
		// Count table rows
		if strings.HasPrefix(trimmed, "|") {
			count++
		}
		// Count code blocks
		if strings.HasPrefix(trimmed, "```") {
			count++
		}
		// Count subheadings
		if strings.HasPrefix(trimmed, "##") {
			count++
		}
	}
	return count
}

// assembleBestDocument assembles a document from the best sections
func assembleBestDocument(bestSections map[string]*DocumentSection, sectionOrder []string) string {
	var result strings.Builder

	for _, title := range sectionOrder {
		normalizedTitle := strings.ToLower(title)
		if section, exists := bestSections[normalizedTitle]; exists {
			if result.Len() > 0 {
				result.WriteString("\n")
			}
			result.WriteString(section.Content)
		}
	}

	// Post-process to deduplicate links in the Vendor Documentation Links section
	assembled := result.String()
	return deduplicateVendorLinks(assembled)
}

// cleanupVendorLinks cleans up the Vendor Documentation Links section by:
// 1. Removing duplicate links that already appear inline in the document body
// 2. Fixing generic link titles to be more descriptive based on the URL
func deduplicateVendorLinks(content string) string {
	// Find the vendor documentation links section
	vendorLinksHeader := "### Vendor Documentation Links"
	vendorLinksIdx := strings.Index(content, vendorLinksHeader)
	if vendorLinksIdx == -1 {
		return content
	}

	// Split into body (before vendor links) and vendor links section
	body := content[:vendorLinksIdx]
	vendorSection := content[vendorLinksIdx:]

	// Extract all URLs from the body
	bodyURLs := extractURLsFromContent(body)
	bodyURLSet := make(map[string]bool)
	for _, url := range bodyURLs {
		bodyURLSet[url] = true
	}

	// Process vendor links section line by line
	lines := strings.Split(vendorSection, "\n")
	var cleanedLines []string
	linkPattern := regexp.MustCompile(`\[([^\]]+)\]\((https?://[^)]+)\)`)

	for _, line := range lines {
		// Check if this line contains a markdown link
		match := linkPattern.FindStringSubmatch(line)
		if len(match) == 3 {
			title := match[1]
			url := match[2]

			// Skip if this URL already appears in the body
			if bodyURLSet[url] {
				continue
			}

			// Fix generic/duplicate titles by generating better ones from URL
			newTitle := generateDescriptiveLinkTitle(title, url)
			if newTitle != title {
				line = strings.Replace(line, "["+title+"]", "["+newTitle+"]", 1)
			}
		}
		cleanedLines = append(cleanedLines, line)
	}

	return body + strings.Join(cleanedLines, "\n")
}

// generateDescriptiveLinkTitle creates a better link title based on the URL
// This fixes cases where generic titles like "Elastic Guide Beats Filebeat" are used
// for multiple different links
func generateDescriptiveLinkTitle(currentTitle, url string) string {
	// Check if the title is generic and needs improvement
	genericTitles := map[string]bool{
		"Elastic Guide Beats Filebeat": true,
		"Elastic Guide Beats":          true,
		"Elastic Guide":                true,
		"Citrix Docs":                  true,
		"NetScaler Docs":               true,
	}

	if !genericTitles[currentTitle] {
		// Title seems specific enough, keep it
		return currentTitle
	}

	// Extract meaningful parts from the URL to create a better title
	urlLower := strings.ToLower(url)

	// Handle Elastic Beats input documentation
	if strings.Contains(urlLower, "filebeat-input-") {
		// Extract the input type from URL like filebeat-input-httpjson.html
		parts := strings.Split(url, "/")
		if len(parts) > 0 {
			filename := parts[len(parts)-1]
			filename = strings.TrimSuffix(filename, ".html")

			// Convert filebeat-input-httpjson to "Filebeat httpjson Input"
			if strings.HasPrefix(filename, "filebeat-input-") {
				inputType := strings.TrimPrefix(filename, "filebeat-input-")
				return fmt.Sprintf("Filebeat %s Input", inputType)
			}
		}
	}

	// Handle Elastic documentation pages
	if strings.Contains(urlLower, "elastic.co/guide") {
		parts := strings.Split(url, "/")
		if len(parts) >= 2 {
			// Get the last meaningful part
			lastPart := parts[len(parts)-1]
			lastPart = strings.TrimSuffix(lastPart, ".html")
			lastPart = strings.ReplaceAll(lastPart, "-", " ")
			if lastPart != "" && lastPart != "current" {
				return "Elastic: " + strings.Title(lastPart)
			}
		}
	}

	// Handle Citrix/NetScaler documentation
	if strings.Contains(urlLower, "citrix.com") || strings.Contains(urlLower, "netscaler") {
		parts := strings.Split(url, "/")
		if len(parts) >= 2 {
			// Get descriptive parts from the path
			for i := len(parts) - 1; i >= 0; i-- {
				part := parts[i]
				part = strings.TrimSuffix(part, ".html")
				// Skip generic parts
				if part != "" && part != "en-us" && part != "current-release" && part != "latest" && len(part) > 3 {
					part = strings.ReplaceAll(part, "-", " ")
					return "Citrix: " + strings.Title(part)
				}
			}
		}
	}

	// If we can't improve it, return original
	return currentTitle
}

// extractURLsFromContent extracts all URLs from content
func extractURLsFromContent(content string) []string {
	urlPattern := regexp.MustCompile(`https?://[^\s\)\]]+`)
	return urlPattern.FindAllString(content, -1)
}

// getSectionOrder extracts the section order from a document
func getSectionOrder(content string) []string {
	var order []string
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "#") {
			title := strings.TrimSpace(strings.TrimLeft(line, "# "))
			order = append(order, title)
		}
	}
	return order
}

// GenerateWithValidationLoop generates documentation with iterative validation feedback
// This is the core generation loop used by both update documentation and evaluate mode
func (d *DocumentationAgent) GenerateWithValidationLoop(ctx context.Context, cfg GenerationConfig) (*GenerationResult, error) {
	// Set defaults
	maxIterations := cfg.MaxIterations
	if maxIterations == 0 {
		maxIterations = 3
	}

	// Start a workflow span to group all iterations together for tracing
	ctx, workflowSpan := tracing.StartWorkflowSpanWithConfig(ctx, "doc:validation_loop", maxIterations)
	defer func() {
		tracing.SetSpanOk(workflowSpan)
		workflowSpan.End()
	}()

	// Load package context for validation
	pkgCtx, err := validators.LoadPackageContext(d.packageRoot)
	if err != nil {
		tracing.SetSpanError(workflowSpan, err)
		return nil, fmt.Errorf("failed to load package context: %w", err)
	}

	// Record package context on the workflow span
	fieldsCount := 0
	for _, fields := range pkgCtx.Fields {
		fieldsCount += len(fields)
	}
	tracing.RecordPackageContext(workflowSpan, d.manifest.Name, d.manifest.Title, d.manifest.Version,
		len(pkgCtx.DataStreams), fieldsCount)

	result := &GenerationResult{
		StageResults: make(map[string]*StageResult),
		IssueHistory: make([]int, 0),
	}

	var validationFeedback string
	extraIterationAllowed := true // Allow one extra iteration if converging

	// Track best sections across iterations to avoid regression
	// Sometimes later iterations produce worse output (truncated/summarized content) for specific sections
	bestSections := make(map[string]*DocumentSection)
	var sectionOrder []string // Preserve original section order

	// Section-level iteration loop
	for iteration := uint(1); iteration <= maxIterations; iteration++ {
		result.TotalIterations = int(iteration)

		// Start an iteration span for tracing
		iterCtx, iterSpan := tracing.StartGenerationIterationSpan(ctx, int(iteration), validationFeedback != "")
		tracing.RecordWorkflowIteration(workflowSpan, int(iteration))

		// Format iteration label (with bonus indicator if applicable)
		iterationLabel := fmt.Sprintf("%d/%d", iteration, maxIterations)
		if result.ConvergenceBonus && iteration == maxIterations {
			iterationLabel = fmt.Sprintf("%d (bonus - converging)", iteration)
		}

		// Generate documentation
		fmt.Printf("📝 Generating documentation (iteration %s)...\n", iterationLabel)
		workflowCfg := d.buildWorkflowConfig()

		var content string
		var genErr error

		if validationFeedback != "" {
			// Regenerate with validation feedback
			content, genErr = d.regenerateDocWithFeedback(iterCtx, result.Content, validationFeedback)
		} else {
			// Initial generation - use full document workflow
			content, genErr = d.GenerateFullDocumentWithWorkflow(iterCtx, workflowCfg)
		}

		if genErr != nil {
			tracing.SetSpanError(iterSpan, genErr)
			iterSpan.End()
			return nil, fmt.Errorf("failed to generate documentation: %w", genErr)
		}

		// Record content length on iteration span
		tracing.RecordGenerationContent(iterSpan, len(content))

		// Parse current iteration into sections
		currentSections := parseDocumentIntoSections(content)

		// On first iteration, establish section order
		if iteration == 1 {
			sectionOrder = getSectionOrder(content)
		}

		// Compare each section with best and update if better
		fmt.Printf("📊 Comparing sections with best versions...\n")
		sectionsUpdated := 0
		for title, currentSection := range currentSections {
			bestSection := bestSections[title]
			if isSectionBetter(currentSection, bestSection) {
				bestSections[title] = currentSection
				sectionsUpdated++
				if bestSection != nil {
					fmt.Printf("  📈 Section '%s': updated (length: %d → %d)\n",
						currentSection.Title, bestSection.Length, currentSection.Length)
				} else {
					fmt.Printf("  📌 Section '%s': new (length: %d)\n",
						currentSection.Title, currentSection.Length)
				}
			} else if bestSection != nil {
				fmt.Printf("  ⏸️  Section '%s': kept best (current: %d, best: %d)\n",
					currentSection.Title, currentSection.Length, bestSection.Length)
			}
		}
		fmt.Printf("  Updated %d/%d sections from iteration %d\n", sectionsUpdated, len(currentSections), iteration)

		// Assemble the best composite document for validation and next iteration
		compositeContent := assembleBestDocument(bestSections, sectionOrder)
		result.Content = compositeContent

		// Save snapshot if enabled (before validation)
		if cfg.SnapshotManager != nil {
			// Save both the raw iteration output and the composite
			if err := cfg.SnapshotManager.SaveSnapshot(content, fmt.Sprintf("iteration_%d_raw", iteration), int(iteration), nil); err != nil {
				logger.Debugf("Failed to save raw iteration snapshot: %v", err)
			}
			if err := cfg.SnapshotManager.SaveSnapshot(compositeContent, fmt.Sprintf("iteration_%d", iteration), int(iteration), nil); err != nil {
				logger.Debugf("Failed to save iteration snapshot: %v", err)
			}
		}

		// Run staged validation if enabled
		if cfg.EnableStagedValidation {
			result.Approved = true
			result.StageResults = make(map[string]*StageResult) // Reset for this iteration
			fmt.Println("🔍 Running staged validation...")

			// Group validators by stage to avoid double-counting issues
			vals := specialists.AllStagedValidators()
			validatorsByStage := make(map[string][]validators.StagedValidator)
			for _, validator := range vals {
				if validator.SupportsStaticValidation() {
					stageName := validator.Stage().String()
					validatorsByStage[stageName] = append(validatorsByStage[stageName], validator)
				}
			}

			// Process each stage once, aggregating results from all validators in that stage
			var allIssues []string
			stageOrder := []string{"structure", "accuracy", "completeness", "quality", "placeholders"}
			for _, stageName := range stageOrder {
				stageValidators, exists := validatorsByStage[stageName]
				if !exists || len(stageValidators) == 0 {
					continue
				}

				// Aggregate results from all validators in this stage
				var stageIssues []string
				var stageWarnings []string
				stageValid := true
				stageScore := 0
				validatorCount := 0

				for _, validator := range stageValidators {
					// Run static validation on the COMPOSITE (best assembled) document, not raw LLM output
					// This ensures we validate the actual best content, not a potentially bad iteration

					// Start static validation span for tracing
					staticCtx, staticSpan := tracing.StartStaticValidationSpan(ctx, validator.Name())
					staticResult, err := validator.StaticValidate(staticCtx, compositeContent, pkgCtx)
					if err != nil {
						tracing.EndValidationSpanWithError(staticSpan, err)
						logger.Debugf("Static validation error for %s: %v", validator.Name(), err)
						continue
					}
					// Record validation result in span
					tracing.EndValidationSpan(staticSpan, staticResult.Valid, staticResult.Score, len(staticResult.Issues), extractIssueMessages(staticResult.Issues))

					validatorCount++

					// Collect static issues (dedupe by message to avoid duplicates from similar validators)
					for _, issue := range staticResult.Issues {
						isDupe := false
						for _, existing := range stageIssues {
							if existing == issue.Message {
								isDupe = true
								break
							}
						}
						if !isDupe {
							stageIssues = append(stageIssues, issue.Message)
						}
					}

					// Collect warnings (also dedupe)
					for _, warning := range staticResult.Warnings {
						isDupe := false
						for _, existing := range stageWarnings {
							if existing == warning {
								isDupe = true
								break
							}
						}
						if !isDupe {
							stageWarnings = append(stageWarnings, warning)
						}
					}

					// Stage is invalid if any validator fails
					if !staticResult.Valid {
						stageValid = false
					}

					// Average the scores
					stageScore += staticResult.Score

					// Run LLM validation if enabled - only for specific validators that benefit from semantic understanding
					// This matches the test harness behavior which only runs LLM validation for these validators
					llmEnabledValidators := map[string]bool{
						"vendor_setup_validator": true, // Setup accuracy against vendor docs
						"style_validator":        true, // Elastic style guide compliance
						"quality_validator":      true, // Writing quality assessment
					}
					if cfg.EnableLLMValidation && validator.SupportsLLMValidation() && llmEnabledValidators[validator.Name()] {
						fmt.Printf("  🧠 LLM validating with %s...\n", validator.Name())
						llmGenerateFunc := d.createLLMValidateFunc()

						// Start LLM validation span for tracing
						llmCtx, llmSpan := tracing.StartLLMValidationSpan(ctx, validator.Name(), d.executor.ModelID())

						// Use composite content for LLM validation too
						llmResult, err := validator.LLMValidate(llmCtx, compositeContent, pkgCtx, llmGenerateFunc)
						if err != nil {
							tracing.EndValidationSpanWithError(llmSpan, err)
							fmt.Printf("  ❌ LLM validation error for %s: %v\n", validator.Name(), err)
							logger.Debugf("LLM validation error for %s: %v", validator.Name(), err)
						} else if llmResult != nil {
							// Record LLM validation result in span
							tracing.EndValidationSpan(llmSpan, llmResult.Valid, llmResult.Score, len(llmResult.Issues), extractIssueMessages(llmResult.Issues))

							fmt.Printf("  ✅ LLM validator %s completed\n", validator.Name())
							// Collect LLM issues (dedupe)
							for _, issue := range llmResult.Issues {
								isDupe := false
								for _, existing := range stageIssues {
									if existing == issue.Message {
										isDupe = true
										break
									}
								}
								if !isDupe {
									stageIssues = append(stageIssues, issue.Message)
								}
							}

							// LLM validation can also fail the stage
							if !llmResult.Valid {
								stageValid = false
							}

							// Use LLM score if available (typically more nuanced)
							if llmResult.Score > 0 {
								stageScore = (stageScore + llmResult.Score) / 2
							}
						} else {
							// No result but no error - just end the span
							llmSpan.End()
						}
					}
				}

				if validatorCount > 0 {
					stageScore = stageScore / validatorCount
				}

				// Store aggregated result for this stage
				result.StageResults[stageName] = &StageResult{
					Stage:      stageName,
					Valid:      stageValid,
					Score:      stageScore,
					Iterations: int(iteration),
					Issues:     stageIssues,
					Warnings:   stageWarnings,
				}

				// Add to allIssues for feedback (with stage prefix)
				for _, issue := range stageIssues {
					allIssues = append(allIssues, fmt.Sprintf("[%s] %s", stageName, issue))
				}

				if !stageValid {
					result.Approved = false
				}

				// Log once per stage (not per validator)
				status := "✅"
				if !stageValid {
					status = "❌"
				}
				fmt.Printf("  %s %s: %d issues (from %d validators)\n", status, stageName, len(stageIssues), validatorCount)
			}

			// Track issue count for convergence detection
			issueCount := len(allIssues)
			result.IssueHistory = append(result.IssueHistory, issueCount)

			// Note: Section-level best selection happens earlier in the loop
			// The composite document (result.Content) already contains the best sections

			// Check if we should continue iterating
			if result.Approved {
				fmt.Printf("✅ Document approved at iteration %d\n", iteration)
				tracing.SetSpanOk(iterSpan)
				iterSpan.End()
				break
			}

			// Check for convergence: are issues decreasing?
			isConverging := false
			if len(result.IssueHistory) >= 2 {
				prev := result.IssueHistory[len(result.IssueHistory)-2]
				curr := result.IssueHistory[len(result.IssueHistory)-1]
				isConverging = curr < prev
				if isConverging {
					fmt.Printf("📈 Issues decreasing: %d → %d (converging)\n", prev, curr)
				}
			}

			// If not approved and we have more iterations, prepare feedback for regeneration
			if iteration < maxIterations {
				validationFeedback = buildDocValidationFeedback(allIssues)
				result.ValidationFeedback = validationFeedback
				fmt.Printf("🔄 Validation failed with %d issues, regenerating...\n", issueCount)
			} else if iteration == maxIterations && isConverging && extraIterationAllowed && issueCount > 0 {
				// Allow one extra iteration if we're converging but haven't hit zero
				maxIterations++
				extraIterationAllowed = false // Only allow one extra
				result.ConvergenceBonus = true
				validationFeedback = buildDocValidationFeedback(allIssues)
				result.ValidationFeedback = validationFeedback
				fmt.Printf("📈 Converging but not yet at zero issues (%d remaining). Allowing bonus iteration...\n", issueCount)
			} else {
				fmt.Printf("⚠️ Max iterations (%d) reached with %d issues remaining\n", maxIterations, issueCount)
			}
		} else {
			result.Approved = true
			tracing.SetSpanOk(iterSpan)
			iterSpan.End()
			break
		}

		// End the iteration span
		tracing.SetSpanOk(iterSpan)
		iterSpan.End()
	}

	// Record workflow result
	tracing.RecordWorkflowResult(workflowSpan, result.Approved, result.TotalIterations, result.Content)

	// Log issue history for convergence analysis
	if len(result.IssueHistory) > 1 {
		fmt.Printf("📊 Issue convergence history: %v\n", result.IssueHistory)
	}

	// Final document is already the composite of best sections from all iterations
	// Log summary of section sources
	fmt.Printf("🏆 Final document assembled from best sections across all iterations:\n")
	for _, title := range sectionOrder {
		normalizedTitle := strings.ToLower(title)
		if section, exists := bestSections[normalizedTitle]; exists {
			fmt.Printf("  - %s: %d chars\n", section.Title, section.Length)
		}
	}
	result.BestIteration = result.TotalIterations // All iterations contributed

	return result, nil
}

// GenerateSectionWithValidationLoop generates a single section with iterative validation
// Each section gets its own generate-validate loop with best-iteration tracking
func (d *DocumentationAgent) GenerateSectionWithValidationLoop(
	ctx context.Context,
	sectionCtx validators.SectionContext,
	pkgCtx *validators.PackageContext,
	cfg GenerationConfig,
) (*SectionGenerationResult, error) {
	maxIterations := cfg.MaxIterations
	if maxIterations == 0 {
		maxIterations = 3
	}

	// Start a workflow span for this section
	ctx, sectionSpan := tracing.StartWorkflowSpanWithConfig(ctx, fmt.Sprintf("section:%s", sectionCtx.SectionTitle), maxIterations)
	defer func() {
		tracing.SetSpanOk(sectionSpan)
		sectionSpan.End()
	}()

	result := &SectionGenerationResult{
		SectionTitle: sectionCtx.SectionTitle,
		SectionLevel: sectionCtx.SectionLevel,
		IssueHistory: make([]int, 0),
	}

	// Track best content across iterations
	var bestContent string
	var bestLength int
	var bestStructure int
	bestIteration := 0

	var validationFeedback string

	// Section-level iteration loop
	for iteration := uint(1); iteration <= maxIterations; iteration++ {
		result.TotalIterations = int(iteration)

		// Build the generator prompt with any feedback from previous iteration
		currentSectionCtx := sectionCtx
		if validationFeedback != "" {
			currentSectionCtx.AdditionalContext = validationFeedback
		}

		// Execute workflow for this section
		workflowCfg := d.buildWorkflowConfig()
		builder := workflow.NewBuilder(workflowCfg)
		workflowResult, err := builder.ExecuteWorkflow(ctx, currentSectionCtx)
		if err != nil {
			logger.Debugf("Section workflow failed for %s (iteration %d): %v", sectionCtx.SectionTitle, iteration, err)
			// Continue to next iteration if we have content from previous iterations
			if bestContent != "" {
				continue
			}
			return nil, fmt.Errorf("failed to generate section %s: %w", sectionCtx.SectionTitle, err)
		}

		content := workflowResult.Content

		// Compare with best and update if better
		currentLength := len(content)
		currentStructure := countStructuralElements(content)

		isBetter := false
		if bestContent == "" {
			isBetter = true
		} else if currentLength > bestLength*12/10 {
			// Significantly longer (20%+) is better
			isBetter = true
		} else if currentLength >= bestLength*9/10 && currentStructure > bestStructure {
			// Similar length but more structure is better
			isBetter = true
		}

		if isBetter {
			bestContent = content
			bestLength = currentLength
			bestStructure = currentStructure
			bestIteration = int(iteration)
			logger.Debugf("Section %s: iteration %d is new best (%d chars, %d structure)",
				sectionCtx.SectionTitle, iteration, currentLength, currentStructure)
		}

		// Run section-level validation if enabled
		if cfg.EnableStagedValidation && pkgCtx != nil {
			issues := d.validateSectionContent(ctx, content, sectionCtx.SectionTitle, pkgCtx)
			issueCount := len(issues)
			result.IssueHistory = append(result.IssueHistory, issueCount)

			if issueCount > 0 {
				// Build feedback for next iteration
				validationFeedback = fmt.Sprintf("Section '%s' has %d issues:\n", sectionCtx.SectionTitle, issueCount)
				for _, issue := range issues {
					validationFeedback += fmt.Sprintf("- %s\n", issue)
				}

				if iteration < maxIterations {
					logger.Debugf("Section %s: %d issues, regenerating...", sectionCtx.SectionTitle, issueCount)
				}
			} else {
				result.Approved = true
				break
			}
		} else {
			// No validation, use first result
			result.Approved = true
			break
		}
	}

	// Use the best content across all iterations
	result.Content = bestContent
	result.BestIteration = bestIteration

	// If we never got content, return an error
	if result.Content == "" {
		return nil, fmt.Errorf("failed to generate content for section %s after %d iterations", sectionCtx.SectionTitle, maxIterations)
	}

	return result, nil
}

// validateSectionContent runs section-level validation and returns issues
func (d *DocumentationAgent) validateSectionContent(ctx context.Context, content, sectionTitle string, pkgCtx *validators.PackageContext) []string {
	var issues []string

	// Run a subset of validators that support section-level validation
	// Full-document validators (like structure) are deferred to assembly phase
	vals := specialists.AllStagedValidators()
	for _, validator := range vals {
		if !validator.SupportsStaticValidation() {
			continue
		}

		// Skip validators that require full document
		if validator.Scope() == validators.ScopeFullDocument {
			continue
		}

		staticResult, err := validator.StaticValidate(ctx, content, pkgCtx)
		if err != nil {
			logger.Debugf("Section validation error for %s with %s: %v", sectionTitle, validator.Name(), err)
			continue
		}

		for _, issue := range staticResult.Issues {
			issues = append(issues, fmt.Sprintf("[%s] %s", validator.Name(), issue.Message))
		}
	}

	return issues
}

// regenerateDocWithFeedback regenerates the document using validation feedback
func (d *DocumentationAgent) regenerateDocWithFeedback(ctx context.Context, previousContent string, feedback string) (string, error) {
	// Build a prompt that includes the previous content and feedback
	// Be VERY explicit that we need the full document output, not a summary
	regeneratePrompt := fmt.Sprintf(`You previously generated the following documentation, but it has validation issues that need to be fixed.

PREVIOUS DOCUMENT:
%s

VALIDATION ISSUES TO FIX:
%s

CRITICAL INSTRUCTIONS:
1. Output the COMPLETE, FULL documentation - every single section from start to finish
2. Do NOT output a summary or description of changes - output the ACTUAL DOCUMENT
3. Do NOT say "I have regenerated..." or similar - just output the markdown directly
4. Start your response with the document title (# heading)
5. Include ALL sections from the previous document, with fixes applied
6. Fix ALL validation issues listed above
7. Do NOT create duplicate sections (each section should appear only once)
8. Maintain proper heading hierarchy (start with # title, then ## sections)

Your response must be the complete markdown document, nothing else. Start with:

`, previousContent, feedback) + "```markdown\n# "

	// Use the executor to regenerate
	result, err := d.executor.ExecuteTask(ctx, regeneratePrompt)
	if err != nil {
		return "", fmt.Errorf("regeneration failed: %w", err)
	}

	// Extract markdown content from response
	content := extractDocMarkdownContent(result.FinalContent)
	if content == "" {
		// If no code block found, try to extract content starting from first heading
		content = extractContentFromHeading(result.FinalContent)
	}
	if content == "" {
		content = result.FinalContent
	}

	// Validate that we got substantial content, not just a summary
	if len(content) < 1000 {
		// Content is suspiciously short - the LLM likely returned a summary
		// Log a warning and return the previous content to avoid regression
		logger.Debugf("Regenerated content too short (%d chars), may be a summary instead of full doc", len(content))
		fmt.Printf("⚠️ Warning: Regenerated content seems incomplete (%d chars). Keeping previous version.\n", len(content))
		return previousContent, nil
	}

	return content, nil
}

// buildDocValidationFeedback constructs feedback string from validation issues
func buildDocValidationFeedback(issues []string) string {
	if len(issues) == 0 {
		return ""
	}

	feedback := "The following issues were found and must be fixed:\n"
	for i, issue := range issues {
		if i >= 10 {
			feedback += fmt.Sprintf("... and %d more issues\n", len(issues)-10)
			break
		}
		feedback += fmt.Sprintf("- %s\n", issue)
	}
	return feedback
}

// extractDocMarkdownContent extracts markdown from a response that may have code fences
func extractDocMarkdownContent(response string) string {
	// Look for markdown code block
	if idx := strings.Index(response, "```markdown"); idx != -1 {
		start := idx + len("```markdown")
		if end := strings.Index(response[start:], "```"); end != -1 {
			return strings.TrimSpace(response[start : start+end])
		}
	}
	// Look for generic code block
	if idx := strings.Index(response, "```"); idx != -1 {
		start := idx + 3
		// Skip language identifier if present
		if newline := strings.Index(response[start:], "\n"); newline != -1 {
			start += newline + 1
		}
		if end := strings.Index(response[start:], "```"); end != -1 {
			return strings.TrimSpace(response[start : start+end])
		}
	}
	return ""
}

// extractContentFromHeading extracts content starting from the first markdown heading
func extractContentFromHeading(response string) string {
	// Look for content starting with a markdown heading
	lines := strings.Split(response, "\n")
	startIdx := -1

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Found a markdown heading
		if strings.HasPrefix(trimmed, "# ") || strings.HasPrefix(trimmed, "---") {
			startIdx = i
			break
		}
	}

	if startIdx == -1 {
		return ""
	}

	// Return everything from the heading onwards
	return strings.TrimSpace(strings.Join(lines[startIdx:], "\n"))
}

// generateSectionsSequential generates all sections one at a time
func (d *DocumentationAgent) generateSectionsSequential(ctx context.Context, workflowCfg workflow.Config, topLevelSections, exampleSections, existingSections []Section) ([]Section, error) {
	fmt.Printf("📝 Generating %d sections sequentially...\n", len(topLevelSections))

	generatedSections := make([]Section, len(topLevelSections))
	successCount := 0
	failCount := 0

	for idx, templateSection := range topLevelSections {
		fmt.Printf("  ⏳ Section %d/%d: %s...\n", idx+1, len(topLevelSections), templateSection.Title)

		result := d.generateSingleSection(ctx, workflowCfg, idx, templateSection, exampleSections, existingSections)
		generatedSections[idx] = result.section

		if result.err != nil {
			failCount++
			fmt.Printf("  ❌ Section %d: %s (failed)\n", idx+1, result.section.Title)
		} else {
			successCount++
			fmt.Printf("  ✅ Section %d: %s (done)\n", idx+1, result.section.Title)
		}
	}

	fmt.Printf("📊 Generated %d/%d sections successfully\n", successCount, len(topLevelSections))
	return generatedSections, nil
}

// generateSingleSection generates a single section using the workflow
func (d *DocumentationAgent) generateSingleSection(ctx context.Context, workflowCfg workflow.Config, index int, tmplSection Section, exampleSections, existingSections []Section) sectionResult {
	builder := workflow.NewBuilder(workflowCfg)

	// Find corresponding example section
	exampleSection := FindSectionByTitle(exampleSections, tmplSection.Title)

	// Find existing section
	var existingSection *Section
	if len(existingSections) > 0 {
		existingSection = FindSectionByTitle(existingSections, tmplSection.Title)
	}

	// Build section context for workflow
	sectionCtx := validators.SectionContext{
		SectionTitle: tmplSection.Title,
		SectionLevel: tmplSection.Level,
		PackageName:  d.manifest.Name,
		PackageTitle: d.manifest.Title,
	}

	if tmplSection.Content != "" {
		sectionCtx.TemplateContent = tmplSection.GetAllContent()
	}
	if exampleSection != nil {
		sectionCtx.ExampleContent = exampleSection.GetAllContent()
	}
	if existingSection != nil {
		sectionCtx.ExistingContent = existingSection.GetAllContent()
	}

	// Execute workflow for this section
	result, err := builder.ExecuteWorkflow(ctx, sectionCtx)
	if err != nil {
		logger.Debugf("Workflow failed for section %s: %v", tmplSection.Title, err)
		// Fall back to placeholder on error
		return sectionResult{
			index: index,
			section: Section{
				Title:   tmplSection.Title,
				Level:   tmplSection.Level,
				Content: fmt.Sprintf("## %s\n\n%s", tmplSection.Title, emptySectionPlaceholder),
			},
			err: err,
		}
	}

	// Create section from result
	generatedSection := Section{
		Title:           tmplSection.Title,
		Level:           tmplSection.Level,
		Content:         result.Content,
		HasPreserve:     tmplSection.HasPreserve,
		PreserveContent: tmplSection.PreserveContent,
	}

	// Parse to extract hierarchical structure
	parsedGenerated := ParseSections(generatedSection.Content)
	if len(parsedGenerated) > 0 {
		generatedSection = parsedGenerated[0]
	}

	logger.Debugf("Section %s generated (iterations: %d, approved: %v)",
		tmplSection.Title, result.Iterations, result.Approved)

	return sectionResult{
		index:   index,
		section: generatedSection,
	}
}

// GetWorkflowConfig returns a workflow configuration suitable for this agent
func (d *DocumentationAgent) GetWorkflowConfig() workflow.Config {
	return d.buildWorkflowConfig()
}

// buildWorkflowConfig creates a workflow configuration with the agent's model and tools
func (d *DocumentationAgent) buildWorkflowConfig() workflow.Config {
	cfg := workflow.DefaultConfig().
		WithModel(d.executor.llmModel).
		WithModelID(d.executor.ModelID()).
		WithTools(d.executor.tools).
		WithToolsets(d.executor.toolsets)

	// Load package context for static validation
	pkgCtx, err := validators.LoadPackageContext(d.packageRoot)
	if err != nil {
		logger.Debugf("Could not load package context for static validation: %v", err)
	} else {
		cfg = cfg.WithStaticValidation(pkgCtx)
		logger.Debugf("Static validation enabled with package context from %s", d.packageRoot)
	}

	return cfg
}

// createLLMValidateFunc creates an LLMGenerateFunc using the agent's executor
// This allows validators to call the LLM without needing direct access to the executor
func (d *DocumentationAgent) createLLMValidateFunc() validators.LLMGenerateFunc {
	return func(ctx context.Context, prompt string) (string, error) {
		result, err := d.executor.ExecuteTask(ctx, prompt)
		if err != nil {
			return "", err
		}
		return result.FinalContent, nil
	}
}

// extractIssueMessages extracts message strings from ValidationIssue slice
func extractIssueMessages(issues []validators.ValidationIssue) []string {
	messages := make([]string, len(issues))
	for i, issue := range issues {
		messages[i] = issue.Message
	}
	return messages
}

// Printer interface for output (satisfied by cobra.Command)
type Printer interface {
	Println(a ...any)
	Printf(format string, a ...any)
}

// DebugRunCriticOnly runs the critic agent on existing documentation sections
// and outputs results to stdout without modifying files
func (d *DocumentationAgent) DebugRunCriticOnly(ctx context.Context, printer Printer) error {
	// Read existing documentation
	existingContent, err := d.readCurrentReadme()
	if err != nil {
		return fmt.Errorf("failed to read existing documentation: %w", err)
	}

	if existingContent == "" {
		return fmt.Errorf("no existing documentation found at _dev/build/docs/%s", d.targetDocFile)
	}

	// Parse into sections
	sections := ParseSections(existingContent)
	if len(sections) == 0 {
		return fmt.Errorf("no sections found in existing documentation")
	}

	printer.Printf("📄 Analyzing %d sections from %s\n\n", len(sections), d.targetDocFile)

	// Build workflow config for running critic
	workflowCfg := d.buildWorkflowConfig()
	builder := workflow.NewBuilder(workflowCfg)

	// Run critic on each section
	for i, section := range sections {
		printer.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
		printer.Printf("Section %d: %s\n", i+1, section.Title)
		printer.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")

		content := section.GetAllContent()
		output, err := builder.RunCriticOnContent(ctx, content)
		if err != nil {
			printer.Printf("❌ Error: %v\n\n", err)
			continue
		}

		// Parse the critic result
		var criticResult specialists.CriticResult
		if jsonErr := parseJSON(output, &criticResult); jsonErr != nil {
			printer.Printf("⚠️  Could not parse critic output as JSON: %v\n", jsonErr)
			printer.Printf("Raw output: %s\n\n", output)
			continue
		}

		// Display results
		if criticResult.Approved {
			printer.Printf("✅ Approved\n")
		} else {
			printer.Printf("❌ Not Approved\n")
		}
		printer.Printf("📊 Score: %d/10\n", criticResult.Score)
		if criticResult.Feedback != "" {
			printer.Printf("💬 Feedback: %s\n", criticResult.Feedback)
		}
		printer.Println()
	}

	return nil
}

// DebugRunValidatorOnly runs the validator agent on existing documentation sections
// and outputs results to stdout without modifying files
func (d *DocumentationAgent) DebugRunValidatorOnly(ctx context.Context, printer Printer) error {
	// Read existing documentation
	existingContent, err := d.readCurrentReadme()
	if err != nil {
		return fmt.Errorf("failed to read existing documentation: %w", err)
	}

	if existingContent == "" {
		return fmt.Errorf("no existing documentation found at _dev/build/docs/%s", d.targetDocFile)
	}

	// Parse into sections
	sections := ParseSections(existingContent)
	if len(sections) == 0 {
		return fmt.Errorf("no sections found in existing documentation")
	}

	printer.Printf("📄 Validating %d sections from %s\n\n", len(sections), d.targetDocFile)

	// Build workflow config for running validator
	workflowCfg := d.buildWorkflowConfig()
	builder := workflow.NewBuilder(workflowCfg)

	// Run validator on each section
	for i, section := range sections {
		printer.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
		printer.Printf("Section %d: %s\n", i+1, section.Title)
		printer.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")

		content := section.GetAllContent()
		output, err := builder.RunValidatorOnContent(ctx, content)
		if err != nil {
			printer.Printf("❌ Error: %v\n\n", err)
			continue
		}

		// Parse the validator result
		var validationResult validators.ValidationResult
		if jsonErr := parseJSON(output, &validationResult); jsonErr != nil {
			printer.Printf("⚠️  Could not parse validator output as JSON: %v\n", jsonErr)
			printer.Printf("Raw output: %s\n\n", output)
			continue
		}

		// Display results
		if validationResult.Valid {
			printer.Printf("✅ Valid\n")
		} else {
			printer.Printf("❌ Invalid\n")
		}
		if len(validationResult.Issues) > 0 {
			printer.Printf("🚨 Issues:\n")
			for _, issue := range validationResult.Issues {
				printer.Printf("   - %s\n", issue)
			}
		}
		if len(validationResult.Warnings) > 0 {
			printer.Printf("⚠️  Warnings:\n")
			for _, warning := range validationResult.Warnings {
				printer.Printf("   - %s\n", warning)
			}
		}
		printer.Println()
	}

	return nil
}

// UpdateDocumentationGeneratorOnly runs documentation generation with only the generator agent
// (no critic or validator), then proceeds to human review or writes to disk
func (d *DocumentationAgent) UpdateDocumentationGeneratorOnly(ctx context.Context, nonInteractive bool) error {
	ctx, sessionSpan := tracing.StartSessionSpan(ctx, "doc:generate:generator-only", d.executor.ModelID())
	var sessionOutput string
	defer func() {
		tracing.EndSessionSpan(ctx, sessionSpan, sessionOutput)
	}()

	// Record the input request
	tracing.RecordSessionInput(sessionSpan, fmt.Sprintf("Generate documentation (generator-only) for package: %s (file: %s)", d.manifest.Name, d.targetDocFile))

	// Backup original README content before making any changes
	d.backupOriginalReadme()

	// Generate all sections using generator-only workflow (no critic/validator)
	fmt.Println("📊 Using generator-only workflow (no critic/validator)")
	workflowCfg := d.buildWorkflowConfig().WithGeneratorOnly()
	sections, err := d.GenerateAllSectionsWithWorkflow(ctx, workflowCfg)
	if err != nil {
		return fmt.Errorf("failed to generate sections: %w", err)
	}

	// Combine sections into final document
	finalContent := CombineSections(sections)
	sessionOutput = fmt.Sprintf("Generated %d sections, %d characters for %s (generator-only)", len(sections), len(finalContent), d.targetDocFile)

	// Write the combined document
	docPath := filepath.Join(d.packageRoot, "_dev", "build", "docs", d.targetDocFile)
	if err := d.writeDocumentation(docPath, finalContent); err != nil {
		return fmt.Errorf("failed to write documentation: %w", err)
	}

	fmt.Printf("\n✅ Documentation generated successfully! (%d sections, %d characters)\n", len(sections), len(finalContent))
	fmt.Printf("📄 Written to: _dev/build/docs/%s\n", d.targetDocFile)

	// In interactive mode, allow review
	if !nonInteractive {
		return d.runInteractiveSectionReview(ctx, sections)
	}

	return nil
}

// parseJSON is a helper to parse JSON output
func parseJSON(data string, v any) error {
	return json.Unmarshal([]byte(data), v)
}
