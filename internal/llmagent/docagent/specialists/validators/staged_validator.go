// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package validators

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/genai"
)

// LLMGenerateFunc is a function type for generating LLM responses
// This allows callers to provide their own LLM implementation
type LLMGenerateFunc func(ctx context.Context, prompt string) (string, error)

// ValidatorStage defines when a validator runs in the pipeline
type ValidatorStage int

const (
	// StageStructure validates document structure and format (Section A)
	StageStructure ValidatorStage = iota
	// StageAccuracy validates content accuracy against package metadata (Section B)
	StageAccuracy
	// StageCompleteness validates all required content is present (Section C)
	StageCompleteness
	// StageURLs validates URLs are valid and accessible (Section D)
	StageURLs
	// StageQuality validates writing quality and clarity (Section E)
	StageQuality
	// StagePlaceholders validates proper placeholder usage (Section F)
	StagePlaceholders
)

// String returns the stage name
func (s ValidatorStage) String() string {
	switch s {
	case StageStructure:
		return "structure"
	case StageAccuracy:
		return "accuracy"
	case StageCompleteness:
		return "completeness"
	case StageURLs:
		return "urls"
	case StageQuality:
		return "quality"
	case StagePlaceholders:
		return "placeholders"
	default:
		return "unknown"
	}
}

// ValidationSeverity indicates the severity of a validation issue
type ValidationSeverity string

const (
	SeverityCritical ValidationSeverity = "critical"
	SeverityMajor    ValidationSeverity = "major"
	SeverityMinor    ValidationSeverity = "minor"
)

// ValidationCategory indicates the category of a validation issue
type ValidationCategory string

const (
	CategoryStructure     ValidationCategory = "structure"
	CategoryAccuracy      ValidationCategory = "accuracy"
	CategoryCompleteness  ValidationCategory = "completeness"
	CategoryURLs          ValidationCategory = "urls"
	CategoryQuality       ValidationCategory = "quality"
	CategoryPlaceholders  ValidationCategory = "placeholders"
	CategoryStyle         ValidationCategory = "style"
	CategoryAccessibility ValidationCategory = "accessibility"
)

// ValidationIssue represents a single validation problem
type ValidationIssue struct {
	Severity    ValidationSeverity `json:"severity"`
	Category    ValidationCategory `json:"category"`
	Location    string             `json:"location"`
	Message     string             `json:"message"`
	Suggestion  string             `json:"suggestion"`
	SourceCheck string             `json:"source_check"` // "static" or "llm"
}

// StagedValidationResult captures the result of a validation stage
type StagedValidationResult struct {
	Stage       ValidatorStage    `json:"stage"`
	Valid       bool              `json:"valid"`
	Score       int               `json:"score"`      // 0-100
	Iterations  int               `json:"iterations"` // Number of validation iterations for this stage
	Issues      []ValidationIssue `json:"issues"`
	Warnings    []string          `json:"warnings"`
	Suggestions []string          `json:"suggestions"` // Actionable feedback for generator
}

// HasCriticalIssues returns true if any critical issues exist
func (r *StagedValidationResult) HasCriticalIssues() bool {
	for _, issue := range r.Issues {
		if issue.Severity == SeverityCritical {
			return true
		}
	}
	return false
}

// GetFeedbackForGenerator formats issues as feedback for the generator
func (r *StagedValidationResult) GetFeedbackForGenerator() string {
	if r.Valid {
		return ""
	}

	var parts []string
	parts = append(parts, fmt.Sprintf("## %s Validation Issues\n", r.Stage.String()))

	for _, issue := range r.Issues {
		parts = append(parts, fmt.Sprintf("- [%s] %s: %s",
			strings.ToUpper(string(issue.Severity)),
			issue.Location,
			issue.Message))
		if issue.Suggestion != "" {
			parts = append(parts, fmt.Sprintf("  → Fix: %s", issue.Suggestion))
		}
	}

	return strings.Join(parts, "\n")
}

// StagedValidator interface for validators that support both static and LLM validation
type StagedValidator interface {
	SectionAgent // Embeds the base agent interface

	// Stage returns which pipeline stage this validator belongs to
	Stage() ValidatorStage

	// StaticValidate runs validation without LLM (fast, deterministic)
	// Returns issues found through static analysis of content and package context
	StaticValidate(ctx context.Context, content string, pkgCtx *PackageContext) (*StagedValidationResult, error)

	// SupportsStaticValidation returns true if this validator has static checks
	SupportsStaticValidation() bool

	// LLMValidate runs validation using the LLM (slower, semantic understanding)
	// Returns issues found through LLM analysis of content quality and accuracy
	LLMValidate(ctx context.Context, content string, pkgCtx *PackageContext, generate LLMGenerateFunc) (*StagedValidationResult, error)

	// SupportsLLMValidation returns true if this validator has LLM-based checks
	SupportsLLMValidation() bool

	// Instruction returns the LLM instruction for this validator
	Instruction() string
}

// BaseStagedValidator provides common functionality for staged validators
type BaseStagedValidator struct {
	name        string
	description string
	stage       ValidatorStage
	instruction string
}

// Name returns the validator name
func (v *BaseStagedValidator) Name() string {
	return v.name
}

// Description returns the validator description
func (v *BaseStagedValidator) Description() string {
	return v.description
}

// Stage returns the validation stage
func (v *BaseStagedValidator) Stage() ValidatorStage {
	return v.stage
}

// Build creates the LLM agent for this validator
func (v *BaseStagedValidator) Build(ctx context.Context, cfg AgentConfig) (agent.Agent, error) {
	return llmagent.New(llmagent.Config{
		Name:                     v.name,
		Description:              v.description,
		Model:                    cfg.Model,
		Instruction:              v.instruction,
		DisallowTransferToParent: true,
		DisallowTransferToPeers:  true,
		GenerateContentConfig: &genai.GenerateContentConfig{
			ResponseMIMEType: "application/json",
		},
	})
}

// StaticValidate default implementation returns empty result (no static checks)
func (v *BaseStagedValidator) StaticValidate(ctx context.Context, content string, pkgCtx *PackageContext) (*StagedValidationResult, error) {
	return &StagedValidationResult{
		Stage: v.stage,
		Valid: true,
	}, nil
}

// SupportsStaticValidation default implementation returns false
func (v *BaseStagedValidator) SupportsStaticValidation() bool {
	return false
}

// LLMValidate runs validation using the LLM
func (v *BaseStagedValidator) LLMValidate(ctx context.Context, content string, pkgCtx *PackageContext, generate LLMGenerateFunc) (*StagedValidationResult, error) {
	if v.instruction == "" || generate == nil {
		return &StagedValidationResult{
			Stage: v.stage,
			Valid: true,
		}, nil
	}

	// Build the prompt for the LLM
	prompt := v.buildLLMPrompt(content, pkgCtx)

	// Call the LLM using the provided generate function
	responseText, err := generate(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("LLM validation failed: %w", err)
	}

	if responseText == "" {
		return &StagedValidationResult{
			Stage:    v.stage,
			Valid:    true,
			Warnings: []string{"LLM returned empty response"},
		}, nil
	}

	// Parse the JSON response
	return ParseLLMValidationResult(responseText, v.stage)
}

// buildLLMPrompt creates the prompt for LLM validation
func (v *BaseStagedValidator) buildLLMPrompt(content string, pkgCtx *PackageContext) string {
	var contextInfo string
	if pkgCtx != nil && pkgCtx.Manifest != nil {
		contextInfo = fmt.Sprintf(`
Package Context:
- Package Name: %s
- Package Title: %s
- Package Version: %s
`, pkgCtx.Manifest.Name, pkgCtx.Manifest.Title, pkgCtx.Manifest.Version)
	}

	return fmt.Sprintf(`%s
%s
Document to validate:
---
%s
---

Respond with a JSON object in this exact format:
{
  "valid": true/false,
  "score": 0-100,
  "issues": [
    {
      "severity": "critical|major|minor",
      "category": "%s",
      "location": "Section or line description",
      "message": "Description of the issue",
      "suggestion": "How to fix it"
    }
  ],
  "summary": "Brief summary of validation"
}

Be thorough but fair. Only flag real issues that would confuse users or violate documentation standards.`,
		v.instruction, contextInfo, content, v.stage.String())
}

// SupportsLLMValidation default implementation returns true if instruction is set
func (v *BaseStagedValidator) SupportsLLMValidation() bool {
	return v.instruction != ""
}

// Instruction returns the LLM instruction for this validator
func (v *BaseStagedValidator) Instruction() string {
	return v.instruction
}

// LLMValidationResult is the expected JSON output from LLM validators
type LLMValidationResult struct {
	Valid   bool              `json:"valid"`
	Score   int               `json:"score"`
	Issues  []ValidationIssue `json:"issues"`
	Summary string            `json:"summary,omitempty"`
}

// ParseLLMValidationResult parses JSON output from an LLM validator
func ParseLLMValidationResult(output string, stage ValidatorStage) (*StagedValidationResult, error) {
	var llmResult LLMValidationResult
	if err := json.Unmarshal([]byte(output), &llmResult); err != nil {
		// If parsing fails, assume valid with warning
		return &StagedValidationResult{
			Stage:    stage,
			Valid:    true,
			Warnings: []string{"Failed to parse LLM validation output: " + err.Error()},
		}, nil
	}

	// Mark all issues as coming from LLM
	for i := range llmResult.Issues {
		llmResult.Issues[i].SourceCheck = "llm"
	}

	return &StagedValidationResult{
		Stage:  stage,
		Valid:  llmResult.Valid,
		Score:  llmResult.Score,
		Issues: llmResult.Issues,
	}, nil
}

// MergeValidationResults combines static and LLM validation results
func MergeValidationResults(static, llm *StagedValidationResult) *StagedValidationResult {
	if static == nil && llm == nil {
		return &StagedValidationResult{Valid: true}
	}
	if static == nil {
		return llm
	}
	if llm == nil {
		return static
	}

	merged := &StagedValidationResult{
		Stage:    static.Stage,
		Valid:    static.Valid && llm.Valid,
		Score:    llm.Score, // Use LLM score
		Issues:   append(static.Issues, llm.Issues...),
		Warnings: append(static.Warnings, llm.Warnings...),
	}

	// Deduplicate suggestions
	seen := make(map[string]bool)
	for _, s := range append(static.Suggestions, llm.Suggestions...) {
		if !seen[s] {
			seen[s] = true
			merged.Suggestions = append(merged.Suggestions, s)
		}
	}

	return merged
}
