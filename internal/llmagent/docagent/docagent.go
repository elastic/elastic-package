// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package docagent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/elastic/elastic-package/internal/llmagent/docagent/agents"
	"github.com/elastic/elastic-package/internal/llmagent/docagent/agents/validators"
	"github.com/elastic/elastic-package/internal/llmagent/docagent/executor"
	"github.com/elastic/elastic-package/internal/llmagent/docagent/parsing"
	"github.com/elastic/elastic-package/internal/llmagent/docagent/prompts"
	"github.com/elastic/elastic-package/internal/llmagent/docagent/workflow"
	"github.com/elastic/elastic-package/internal/llmagent/tools"
	"github.com/elastic/elastic-package/internal/llmagent/tracing"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/packages/archetype"
	"github.com/elastic/elastic-package/internal/profile"
)

const (
	// How far back in the conversation ResponseAnalysis will consider
	analysisLookbackCount = 5
	// emptySectionPlaceholder is the placeholder text for sections that couldn't be populated
	emptySectionPlaceholder = "<< SECTION NOT POPULATED! Add appropriate text, or remove the section. >>"
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
	PromptTypeRevision          = prompts.TypeRevision
	PromptTypeSectionGeneration = prompts.TypeSectionGeneration
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
}

type PromptContext struct {
	Manifest        *packages.PackageManifest
	TargetDocFile   string
	Changes         string
	SectionTitle    string
	SectionLevel    int
	TemplateSection string
	ExampleSection  string
	PackageContext  *validators.PackageContext // For section-specific instructions
}

// SectionGenerationContext holds all the context needed to generate a single section
type SectionGenerationContext struct {
	Section         Section
	TemplateSection *Section
	ExampleSection  *Section
	PackageInfo     PromptContext
	ExistingContent string
	PackageContext  *validators.PackageContext // For section-specific instructions
}

// AgentConfig holds configuration for creating a DocumentationAgent
type AgentConfig struct {
	Provider       string // LLM provider (e.g. gemini); default gemini
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
	serviceInfoManager := NewServiceInfoManager(cfg.PackageRoot, cfg.DocFile)
	// Attempt to load service_info (don't fail if it doesn't exist)
	_ = serviceInfoManager.Load()

	// Get package tools
	packageTools := tools.PackageTools(cfg.PackageRoot, serviceInfoManager)

	// Create executor configuration with system instructions
	provider := cfg.Provider
	if provider == "" {
		provider = "gemini"
	}
	execCfg := executor.Config{
		Provider:       provider,
		APIKey:         cfg.APIKey,
		ModelID:        cfg.ModelID,
		Instruction:    AgentInstructions,
		ThinkingBudget: cfg.ThinkingBudget,
		TracingConfig:  cfg.TracingConfig,
	}

	// Create executor with tools
	exec, err := executor.NewWithToolsets(ctx, execCfg, packageTools, nil)
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

// UpdateDocumentation runs the documentation update process using the shared generation + validation loop.
// Uses section-based generation where each section has its own generate-validate loop.
func (d *DocumentationAgent) UpdateDocumentation(ctx context.Context, nonInteractive bool) error {
	genCfg := DefaultGenerationConfig()
	ctx, sessionSpan := tracing.StartSessionSpan(ctx, "doc:generate", d.executor.ModelID(), d.executor.Provider())
	var sessionOutput string
	defer func() {
		tracing.EndSessionSpan(ctx, sessionSpan, sessionOutput)
	}()

	d.printTracingSessionID(ctx)

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
	if err := d.writeDocumentation(d.docPath(), result.Content); err != nil {
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

	return nil
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

// docPath returns the path to the built documentation file.
func (d *DocumentationAgent) docPath() string {
	return filepath.Join(d.packageRoot, "_dev", "build", "docs", d.targetDocFile)
}

// printTracingSessionID prints the tracing session ID when tracing is enabled.
func (d *DocumentationAgent) printTracingSessionID(ctx context.Context) {
	if !tracing.IsEnabled() {
		return
	}
	if sessionID, ok := tracing.SessionIDFromContext(ctx); ok {
		fmt.Printf("🔍 Tracing session ID: %s\n", sessionID)
	}
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

// placeholderSectionContent returns markdown for a section that couldn't be populated.
func placeholderSectionContent(sectionTitle string) string {
	return fmt.Sprintf("## %s\n\n%s", sectionTitle, emptySectionPlaceholder)
}

// sectionResult holds the result of generating a single section
type sectionResult struct {
	index   int
	section Section
	err     error
}

// loadTemplateExampleExistingSections loads template, example, and existing doc sections and returns top-level template sections.
func (d *DocumentationAgent) loadTemplateExampleExistingSections() (topLevelSections, exampleSections, existingSections []Section, err error) {
	templateSections := parsing.ParseSections(archetype.GetPackageDocsReadmeTemplate())
	if len(templateSections) == 0 {
		return nil, nil, nil, fmt.Errorf("no sections found in template")
	}
	exampleSections = parsing.ParseSections(tools.GetDefaultExampleContent())
	existingContent, _ := d.readCurrentReadme()
	if existingContent != "" {
		existingSections = parsing.ParseSections(existingContent)
	}
	for _, s := range templateSections {
		if s.IsTopLevel() {
			topLevelSections = append(topLevelSections, s)
		}
	}
	if len(topLevelSections) == 0 {
		return nil, nil, nil, fmt.Errorf("no top-level sections found in template")
	}
	return topLevelSections, exampleSections, existingSections, nil
}

// GenerateAllSectionsWithWorkflow generates all sections using the multi-agent workflow.
// This method uses a configurable pipeline of agents (generator, critic, validator, etc.)
// to iteratively refine each section. Sections are generated in parallel.
func (d *DocumentationAgent) GenerateAllSectionsWithWorkflow(ctx context.Context, workflowCfg workflow.Config) ([]Section, error) {
	ctx, chainSpan := tracing.StartChainSpan(ctx, "doc:generate:workflow")
	defer tracing.EndChainSpan(ctx, chainSpan)

	topLevelSections, exampleSections, existingSections, err := d.loadTemplateExampleExistingSections()
	if err != nil {
		return nil, err
	}
	return d.generateSectionsParallel(ctx, workflowCfg, topLevelSections, exampleSections, existingSections)
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
	defer tracing.EndChainSpan(ctx, chainSpan)

	topLevelSections, exampleSections, existingSections, err := d.loadTemplateExampleExistingSections()
	if err != nil {
		return nil, err
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
			sectionCtx := d.buildSectionContext(tmplSection, exampleSections, existingSections)

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
				Content:      placeholderSectionContent(topLevelSections[res.index].Title),
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
	finalContent := parsing.CombineSectionsWithTitle(generatedSections, d.packageTitle(pkgCtx))

	// Programmatic structure fixup (ensure title is correct)
	finalContent = d.FixDocumentStructure(finalContent, pkgCtx)

	// Ensure all data stream templates are present in Reference section
	finalContent = d.EnsureDataStreamTemplates(finalContent, pkgCtx)
	// Ensure Agentless deployment section is present iff the package has agentless enabled
	finalContent = d.EnsureAgentlessSection(finalContent, pkgCtx)
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
	vals := agents.AllStagedValidators()
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

// buildSectionContext builds SectionContext for a template section from example and existing sections.
func (d *DocumentationAgent) buildSectionContext(tmplSection Section, exampleSections, existingSections []Section) validators.SectionContext {
	sectionCtx := validators.SectionContext{
		SectionTitle: tmplSection.Title,
		SectionLevel: tmplSection.Level,
		PackageName:  d.manifest.Name,
		PackageTitle: d.manifest.Title,
	}
	if tmplSection.Content != "" {
		templateContent := tmplSection.GetAllContent()
		templateContent = strings.ReplaceAll(templateContent, "{[.Manifest.Title]}", d.manifest.Title)
		sectionCtx.TemplateContent = templateContent
	}
	if ex := parsing.FindSectionByTitle(exampleSections, tmplSection.Title); ex != nil {
		sectionCtx.ExampleContent = ex.GetAllContent()
	}
	if len(existingSections) > 0 {
		if ex := parsing.FindSectionByTitle(existingSections, tmplSection.Title); ex != nil {
			sectionCtx.ExistingContent = ex.GetAllContent()
		}
	}
	return sectionCtx
}

// generateSingleSection generates a single section using the workflow
func (d *DocumentationAgent) generateSingleSection(ctx context.Context, workflowCfg workflow.Config, index int, tmplSection Section, exampleSections, existingSections []Section) sectionResult {
	builder := workflow.NewBuilder(workflowCfg)
	sectionCtx := d.buildSectionContext(tmplSection, exampleSections, existingSections)

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
				Content: placeholderSectionContent(tmplSection.Title),
			},
			err: err,
		}
	}

	// Create section from result
	generatedSection := Section{
		Title:   tmplSection.Title,
		Level:   tmplSection.Level,
		Content: result.Content,
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
		WithProvider(d.executor.Provider()).
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

// CreateLLMValidateFunc creates an LLMGenerateFunc using the agent's executor
// This allows validators to call the LLM without needing direct access to the executor
func (d *DocumentationAgent) CreateLLMValidateFunc() validators.LLMGenerateFunc {
	return func(ctx context.Context, prompt string) (string, error) {
		result, err := d.executor.ExecuteTask(ctx, prompt)
		if err != nil {
			return "", err
		}
		return result.FinalContent, nil
	}
}

// Manifest returns the package manifest
func (d *DocumentationAgent) Manifest() *packages.PackageManifest {
	return d.manifest
}

// PackageRoot returns the package root path
func (d *DocumentationAgent) PackageRoot() string {
	return d.packageRoot
}

// TargetDocFile returns the target documentation file name
func (d *DocumentationAgent) TargetDocFile() string {
	return d.targetDocFile
}

// ModelID returns the model ID from the executor
func (d *DocumentationAgent) ModelID() string {
	return d.executor.ModelID()
}

// Provider returns the LLM provider name from the executor
func (d *DocumentationAgent) Provider() string {
	return d.executor.Provider()
}

// packageTitle returns the package title, preferring pkgCtx.Manifest.Title when set.
func (d *DocumentationAgent) packageTitle(pkgCtx *validators.PackageContext) string {
	if pkgCtx != nil && pkgCtx.Manifest != nil {
		return pkgCtx.Manifest.Title
	}
	return d.manifest.Title
}

// FixDocumentStructure programmatically fixes document structure issues
// Ensures title is correct and returns the fixed content
func (d *DocumentationAgent) FixDocumentStructure(content string, pkgCtx *validators.PackageContext) string {
	return parsing.EnsureDocumentTitle(content, d.packageTitle(pkgCtx))
}
