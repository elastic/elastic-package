// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package docagent

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/elastic/elastic-package/internal/llmagent/docagent/specialists"
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
	APIKey         string
	ModelID        string
	PackageRoot    string
	RepositoryRoot *os.Root
	DocFile        string
	Profile        *profile.Profile
	ThinkingBudget *int32         // Optional thinking budget for Gemini models
	TracingConfig  tracing.Config // Tracing configuration
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

// UpdateDocumentation runs the documentation update process using section-based generation
func (d *DocumentationAgent) UpdateDocumentation(ctx context.Context, nonInteractive bool) error {
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

	// Generate all sections using multi-agent workflow (generator → critic → validator)
	fmt.Println("📊 Using multi-agent workflow (generator → critic → validator)")
	workflowCfg := d.buildWorkflowConfig()
	sections, err := d.GenerateAllSectionsWithWorkflow(ctx, workflowCfg)
	if err != nil {
		return fmt.Errorf("failed to generate sections: %w", err)
	}

	// Combine sections into final document
	finalContent := CombineSections(sections)
	sessionOutput = fmt.Sprintf("Generated %d sections, %d characters for %s", len(sections), len(finalContent), d.targetDocFile)

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

// GenerateAllSectionsWithWorkflow generates all sections using the multi-agent workflow.
// This method uses a configurable pipeline of agents (generator, critic, validator, etc.)
// to iteratively refine each section.
func (d *DocumentationAgent) GenerateAllSectionsWithWorkflow(ctx context.Context, workflowCfg workflow.Config) ([]Section, error) {
	ctx, chainSpan := tracing.StartChainSpan(ctx, "doc:generate:workflow")
	defer chainSpan.End()

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

	// Create workflow builder
	builder := workflow.NewBuilder(workflowCfg)

	// Generate ONLY top-level sections
	var generatedSections []Section

	for i, templateSection := range templateSections {
		// Skip subsections - they're generated with their parent
		if !templateSection.IsTopLevel() {
			continue
		}

		fmt.Printf("📝 Generating section %d: %s (using multi-agent workflow)\n", i+1, templateSection.Title)

		// Find corresponding example section
		exampleSection := FindSectionByTitle(exampleSections, templateSection.Title)

		// Find existing section
		var existingSection *Section
		if len(existingSections) > 0 {
			existingSection = FindSectionByTitle(existingSections, templateSection.Title)
		}

		// Build section context for workflow
		sectionCtx := specialists.SectionContext{
			SectionTitle: templateSection.Title,
			SectionLevel: templateSection.Level,
			PackageName:  d.manifest.Name,
			PackageTitle: d.manifest.Title,
		}

		if templateSection.Content != "" {
			sectionCtx.TemplateContent = templateSection.GetAllContent()
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
			logger.Debugf("Workflow failed for section %s: %v", templateSection.Title, err)
			// Fall back to placeholder on error
			generatedSections = append(generatedSections, Section{
				Title:   templateSection.Title,
				Level:   templateSection.Level,
				Content: fmt.Sprintf("## %s\n\n%s", templateSection.Title, emptySectionPlaceholder),
			})
			continue
		}

		// Create section from result
		generatedSection := Section{
			Title:           templateSection.Title,
			Level:           templateSection.Level,
			Content:         result.Content,
			HasPreserve:     templateSection.HasPreserve,
			PreserveContent: templateSection.PreserveContent,
		}

		// Parse to extract hierarchical structure
		parsedGenerated := ParseSections(generatedSection.Content)
		if len(parsedGenerated) > 0 {
			generatedSection = parsedGenerated[0]
		}

		logger.Debugf("Section %s generated (iterations: %d, approved: %v)",
			templateSection.Title, result.Iterations, result.Approved)

		generatedSections = append(generatedSections, generatedSection)
	}

	return generatedSections, nil
}

// GetWorkflowConfig returns a workflow configuration suitable for this agent
func (d *DocumentationAgent) GetWorkflowConfig() workflow.Config {
	return d.buildWorkflowConfig()
}

// buildWorkflowConfig creates a workflow configuration with the agent's model and tools
func (d *DocumentationAgent) buildWorkflowConfig() workflow.Config {
	return workflow.DefaultConfig().
		WithModel(d.executor.llmModel).
		WithModelID(d.executor.ModelID()).
		WithTools(d.executor.tools).
		WithToolsets(d.executor.toolsets)
}
