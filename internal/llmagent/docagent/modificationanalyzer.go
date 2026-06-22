// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package docagent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/elastic/elastic-package/internal/logger"
)

// ScopeType indicates the scope of a modification request
type ScopeType int

const (
	// ScopeGlobal indicates the modification affects the entire document
	ScopeGlobal ScopeType = iota
	// ScopeSpecific indicates the modification affects specific sections
	ScopeSpecific
	// ScopeAmbiguous indicates the scope is unclear
	ScopeAmbiguous
)

// String returns the string representation of ScopeType
func (s ScopeType) String() string {
	switch s {
	case ScopeGlobal:
		return "global"
	case ScopeSpecific:
		return "specific"
	case ScopeAmbiguous:
		return "ambiguous"
	default:
		return "unknown"
	}
}

// ModificationScope represents the analyzed scope of a modification request
type ModificationScope struct {
	Type             ScopeType // global, specific, or ambiguous
	AffectedSections []string  // Section titles to modify
	Confidence       float64   // 0.0 to 1.0
	Reasoning        string    // Explanation of classification
}

// scopeAnalysisResponse is the expected JSON response from the LLM
type scopeAnalysisResponse struct {
	Type       string   `json:"type"`
	Sections   []string `json:"sections"`
	Confidence float64  `json:"confidence"`
	Reasoning  string   `json:"reasoning"`
}

// analyzeModificationScope determines which sections a modification request affects
func (d *DocumentationAgent) analyzeModificationScope(ctx context.Context, modificationPrompt string, availableSections []Section) *ModificationScope {
	// Build the analysis prompt with hierarchical structure
	prompt := d.buildScopeAnalysisPromptHierarchical(modificationPrompt, availableSections)

	// Execute the analysis
	logger.Debugf("Analyzing modification scope for prompt: %s", modificationPrompt)
	result, err := d.executor.ExecuteTask(ctx, prompt)
	if err != nil {
		logger.Debugf("Scope analysis failed, defaulting to global: %v", err)
		return defaultModificationScope("Analysis failed, defaulting to global scope")
	}

	// Parse the response
	scope, err := d.parseScopeAnalysisResponse(result.FinalContent)
	if err != nil {
		logger.Debugf("Failed to parse scope analysis, defaulting to global: %v", err)
		return defaultModificationScope("Failed to parse analysis, defaulting to global scope")
	}

	logger.Debugf("Scope analysis complete: type=%s, sections=%v, confidence=%.2f", scope.Type, scope.AffectedSections, scope.Confidence)
	return scope
}

// buildScopeAnalysisPromptHierarchical creates the prompt for scope analysis with hierarchical structure
func (d *DocumentationAgent) buildScopeAnalysisPromptHierarchical(modificationPrompt string, sections []Section) string {
	// Build numbered list of sections with subsections indented
	var sectionsBuilder strings.Builder
	counter := 1

	for _, section := range sections {
		fmt.Fprintf(&sectionsBuilder, "%d. %s\n", counter, section.Title)
		counter++

		// List subsections with indentation
		for _, subsection := range section.Subsections {
			fmt.Fprintf(&sectionsBuilder, "   %d. %s (subsection of %s)\n", counter, subsection.Title, section.Title)
			counter++
		}
	}

	promptCtx := PromptContext{
		Manifest:      d.manifest,
		TargetDocFile: d.targetDocFile,
		Changes:       modificationPrompt,
	}

	promptCtx.SectionList = sectionsBuilder.String()

	return d.buildPrompt(PromptTypeModificationAnalysis, promptCtx)
}

// parseScopeAnalysisResponse parses the LLM's scope analysis response
func (d *DocumentationAgent) parseScopeAnalysisResponse(content string) (*ModificationScope, error) {
	// Try to find JSON in the response
	jsonStart := strings.Index(content, "{")
	jsonEnd := strings.LastIndex(content, "}")

	if jsonStart == -1 || jsonEnd == -1 {
		return nil, fmt.Errorf("no JSON found in response")
	}

	jsonContent := content[jsonStart : jsonEnd+1]

	// Parse JSON
	var response scopeAnalysisResponse
	if err := json.Unmarshal([]byte(jsonContent), &response); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Convert string type to ScopeType
	var scopeType ScopeType
	switch strings.ToLower(response.Type) {
	case "global":
		scopeType = ScopeGlobal
	case "specific":
		scopeType = ScopeSpecific
	case "ambiguous":
		scopeType = ScopeAmbiguous
	default:
		scopeType = ScopeGlobal
	}

	// If specific scope but no sections listed, treat as global
	if scopeType == ScopeSpecific && len(response.Sections) == 0 {
		scopeType = ScopeGlobal
	}

	// If specific scope with low confidence, treat as ambiguous so the user sees a warning
	if scopeType == ScopeSpecific && response.Confidence < 0.6 {
		scopeType = ScopeAmbiguous
	}

	return &ModificationScope{
		Type:             scopeType,
		AffectedSections: response.Sections,
		Confidence:       response.Confidence,
		Reasoning:        response.Reasoning,
	}, nil
}

// isSectionAffected checks if a section title matches any of the affected section titles
func isSectionAffected(sectionTitle string, affectedTitles []string) bool {
	titleLower := strings.ToLower(strings.TrimSpace(sectionTitle))

	for _, affected := range affectedTitles {
		affectedLower := strings.ToLower(strings.TrimSpace(affected))

		// Exact match
		if titleLower == affectedLower {
			return true
		}

		// Fuzzy match: check if the affected title from the LLM contains the actual section title
		// (handles cases where the LLM returns a longer name like "Data Streams Configuration" for section "Configuration")
		// We do NOT check the reverse direction to avoid matching short terms like "Data" against "Data streams".
		if strings.Contains(affectedLower, titleLower) {
			return true
		}
	}

	return false
}
