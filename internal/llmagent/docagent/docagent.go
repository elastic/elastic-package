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
	"strings"
	"sync"

	"github.com/elastic/elastic-package/internal/llmagent/docagent/executor"
	"github.com/elastic/elastic-package/internal/llmagent/docagent/parsing"
	"github.com/elastic/elastic-package/internal/llmagent/docagent/prompts"
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

// Type aliases for subpackage types
type (
	Executor          = executor.Executor
	Section           = parsing.Section
	ConversationEntry = executor.ConversationEntry
)

// AgentInstructions is the system prompt for the agent
var AgentInstructions = prompts.AgentInstructions

// TaskResult is the result of an executor task
type TaskResult = executor.TaskResult

// PromptType constants
type PromptType = prompts.Type

const (
	PromptTypeRevision             = prompts.TypeRevision
	PromptTypeSectionGeneration    = prompts.TypeSectionGeneration
	PromptTypeModificationAnalysis = prompts.TypeModificationAnalysis
	PromptTypeModification         = prompts.TypeModification
)

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
	PackageContext  *validators.PackageContext // For section-specific instructions
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
	serviceInfoManager := NewServiceInfoManager(cfg.PackageRoot, cfg.DocFile)
	// Attempt to load service_info (don't fail if it doesn't exist)
	_ = serviceInfoManager.Load()

	// Get package tools
	packageTools := tools.PackageTools(cfg.PackageRoot, serviceInfoManager)

	// Load MCP toolsets
	mcpToolsets := mcptools.LoadToolsets()

	// Create executor configuration with system instructions
	execCfg := executor.Config{
		APIKey:         cfg.APIKey,
		ModelID:        cfg.ModelID,
		Instruction:    AgentInstructions,
		ThinkingBudget: cfg.ThinkingBudget,
		TracingConfig:  cfg.TracingConfig,
	}

	// Create executor with tools and toolsets
	exec, err := executor.NewWithToolsets(ctx, execCfg, packageTools, mcpToolsets)
	if err != nil {
		return nil, fmt.Errorf("failed to create executor: %w", err)
	}

	manifest, err := packages.ReadPackageManifestFromPackageRoot(cfg.PackageRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to read package manifest: %w", err)
	}

	responseAnalyzer := NewResponseAnalyzer()
	return &DocumentationAgent{
		executor:           exec,
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
	pkgCtx, err := validators.LoadPackageContextForDoc(d.packageRoot, d.targetDocFile)
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
	sections := parsing.ParseSections(result.Content)
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
		// Show validation issues for sections that failed validation
		if !sr.Approved && len(sr.ValidationIssues) > 0 {
			for _, issue := range sr.ValidationIssues {
				fmt.Printf("      - %s\n", issue)
			}
		}
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
	existingSections := parsing.ParseSections(existingContent)

	if len(existingSections) == 0 {
		return fmt.Errorf("no sections found in existing documentation")
	}

	// Get template sections for reference (structure)
	templateContent := archetype.GetPackageDocsReadmeTemplate()
	templateSections := parsing.ParseSections(templateContent)

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
	finalContent := parsing.CombineSections(finalSections)
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
			parsedModified := parsing.ParseSections(modifiedSection.Content)
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
	templateSections := parsing.ParseSections(templateContent)
	if len(templateSections) == 0 {
		return nil, fmt.Errorf("no sections found in template")
	}

	// Parse sections from example
	exampleSections := parsing.ParseSections(exampleContent)

	// Read existing documentation if it exists
	existingContent, _ := d.readCurrentReadme()
	var existingSections []Section
	if existingContent != "" {
		existingSections = parsing.ParseSections(existingContent)
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
	templateSections := parsing.ParseSections(templateContent)
	if len(templateSections) == 0 {
		return nil, fmt.Errorf("no sections found in template")
	}

	// Parse sections from example
	exampleSections := parsing.ParseSections(exampleContent)

	// Read existing documentation if it exists
	existingContent, _ := d.readCurrentReadme()
	var existingSections []Section
	if existingContent != "" {
		existingSections = parsing.ParseSections(existingContent)
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
			exampleSection := parsing.FindSectionByTitle(exampleSections, tmplSection.Title)

			// Find existing section
			var existingSection *Section
			if len(existingSections) > 0 {
				existingSection = parsing.FindSectionByTitle(existingSections, tmplSection.Title)
			}

			// Build section context for workflow
			sectionCtx := validators.SectionContext{
				SectionTitle: tmplSection.Title,
				SectionLevel: tmplSection.Level,
				PackageName:  d.manifest.Name,
				PackageTitle: d.manifest.Title,
			}

			if tmplSection.Content != "" {
				// Render template placeholders with package-specific values
				templateContent := tmplSection.GetAllContent()
				templateContent = strings.ReplaceAll(templateContent, "{[.Manifest.Title]}", d.manifest.Title)
				sectionCtx.TemplateContent = templateContent
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
			section := normalizeSectionContent(sr.SectionTitle, sr.SectionLevel, sr.Content)
			generatedSections = append(generatedSections, section)
			sectionResults = append(sectionResults, *sr)
		}
	}

	// Combine sections into final document with title
	packageTitle := d.manifest.Title
	if pkgCtx != nil && pkgCtx.Manifest != nil {
		packageTitle = pkgCtx.Manifest.Title
	}
	finalContent := parsing.CombineSectionsWithTitle(generatedSections, packageTitle)

	// Programmatic structure fixup (ensure title is correct)
	finalContent = d.FixDocumentStructure(finalContent, pkgCtx)

	// Ensure all data stream templates are present in Reference section
	finalContent = d.EnsureDataStreamTemplates(finalContent, pkgCtx)
	fmt.Printf("✅ Document assembled\n")

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
	// ValidationIssues contains unresolved validation issues (if not approved)
	ValidationIssues []string
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
	// ValidationFeedback contains the last validation feedback (if any)
	ValidationFeedback string
}

// normalizeSectionContent ensures a section has a proper header and non-empty content
// - Strips out any H1 titles or AI notes that the generator may have incorrectly included
// - Verifies the section header exists and is at the correct level
// - If the content is empty, adds a placeholder comment
func normalizeSectionContent(sectionTitle string, sectionLevel int, content string) Section {
	content = strings.TrimSpace(content)

	// Strip out H1 titles and AI notes that generators may incorrectly include
	// The document title and AI note are added separately by CombineSectionsWithTitle
	content = stripDocumentPreamble(content)

	// Build expected header prefix
	headerPrefix := strings.Repeat("#", sectionLevel) + " "
	expectedHeader := headerPrefix + sectionTitle

	// Check if content starts with the correct header
	if content == "" || !strings.HasPrefix(content, headerPrefix) {
		// Content is missing or doesn't start with header - add header
		if content == "" {
			content = expectedHeader + "\n\n<!-- SECTION NOT POPULATED! Add required information -->"
		} else {
			content = expectedHeader + "\n\n" + content
		}
	}

	// Parse the content to get proper Section structure with subsections
	parsedSections := parsing.ParseSections(content)
	if len(parsedSections) > 0 {
		return parsedSections[0]
	}

	// Fallback: create section from content directly
	return Section{
		Title:       sectionTitle,
		Level:       sectionLevel,
		Content:     content,
		FullContent: content,
	}
}

// stripDocumentPreamble removes H1 titles and AI notes from the beginning of content.
// Generators sometimes incorrectly include these, but they should be added only once
// by the document assembly process.
func stripDocumentPreamble(content string) string {
	lines := strings.Split(content, "\n")
	startIdx := 0

	for startIdx < len(lines) {
		line := strings.TrimSpace(lines[startIdx])

		// Skip H1 titles (should only be one at document level)
		if strings.HasPrefix(line, "# ") && !strings.HasPrefix(line, "## ") {
			startIdx++
			continue
		}

		// Skip AI-generated notice
		if strings.HasPrefix(line, "> **Note**: This documentation was generated") {
			startIdx++
			continue
		}

		// Skip empty lines at the beginning
		if line == "" {
			startIdx++
			continue
		}

		// Found actual content
		break
	}

	if startIdx == 0 {
		return content // Nothing to strip
	}

	return strings.TrimSpace(strings.Join(lines[startIdx:], "\n"))
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
	}

	// Track best content across iterations
	var bestContent string
	var bestLength int
	var bestStructure int
	bestIteration := 0

	var validationFeedback string
	var lastValidationIssues []string

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
		currentStructure := parsing.CountStructuralElements(content)

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
			if issueCount > 0 {
				// Store issues for reporting if this is the last iteration
				lastValidationIssues = issues

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
				lastValidationIssues = nil // Clear issues on success
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
	result.ValidationIssues = lastValidationIssues

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
	exampleSection := parsing.FindSectionByTitle(exampleSections, tmplSection.Title)

	// Find existing section
	var existingSection *Section
	if len(existingSections) > 0 {
		existingSection = parsing.FindSectionByTitle(existingSections, tmplSection.Title)
	}

	// Build section context for workflow
	sectionCtx := validators.SectionContext{
		SectionTitle: tmplSection.Title,
		SectionLevel: tmplSection.Level,
		PackageName:  d.manifest.Name,
		PackageTitle: d.manifest.Title,
	}

	if tmplSection.Content != "" {
		// Render template placeholders with package-specific values
		templateContent := tmplSection.GetAllContent()
		templateContent = strings.ReplaceAll(templateContent, "{[.Manifest.Title]}", d.manifest.Title)
		sectionCtx.TemplateContent = templateContent
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
	parsedGenerated := parsing.ParseSections(generatedSection.Content)
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
		WithModel(d.executor.Model()).
		WithModelID(d.executor.ModelID()).
		WithTools(d.executor.Tools()).
		WithToolsets(d.executor.Toolsets())

	// Load package context for static validation
	pkgCtx, err := validators.LoadPackageContextForDoc(d.packageRoot, d.targetDocFile)
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
	sections := parsing.ParseSections(existingContent)
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
	sections := parsing.ParseSections(existingContent)
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
	finalContent := parsing.CombineSections(sections)
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

// FixDocumentStructure programmatically fixes document structure issues
// Ensures title is correct and returns the fixed content
func (d *DocumentationAgent) FixDocumentStructure(content string, pkgCtx *validators.PackageContext) string {
	packageTitle := d.manifest.Title
	if pkgCtx != nil && pkgCtx.Manifest != nil {
		packageTitle = pkgCtx.Manifest.Title
	}

	// Ensure the title is correct
	content = parsing.EnsureDocumentTitle(content, packageTitle)

	return content
}

// min returns the smaller of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
