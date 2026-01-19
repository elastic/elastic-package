// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package generation

import (
	"github.com/elastic/elastic-package/internal/llmagent/docagent/workflow"
)

// Config holds configuration for documentation generation
type Config struct {
	MaxIterations          uint                      // Max iterations per section (default: 3)
	EnableStagedValidation bool                      // Enable validation after generation
	EnableLLMValidation    bool                      // Enable LLM-based semantic validation
	SnapshotManager        *workflow.SnapshotManager // For saving iteration snapshots
}

// DefaultConfig returns default generation configuration
func DefaultConfig() Config {
	return Config{
		MaxIterations:          3,
		EnableStagedValidation: true,
		EnableLLMValidation:    false, // Off by default for faster generation
	}
}

// SectionResult represents the result of generating a single section
type SectionResult struct {
	SectionTitle    string // Title of the section
	SectionLevel    int    // Heading level (2 = ##, 3 = ###, etc.)
	Content         string // Best generated content
	Approved        bool   // Whether validation passed
	TotalIterations int    // Iterations performed
	BestIteration   int    // Iteration that produced best content
	IssueHistory    []int  // Issue counts per iteration
}

// Result represents the result of documentation generation
type Result struct {
	Content            string                 // Final generated documentation
	Approved           bool                   // Whether all validation stages passed
	TotalIterations    int                    // Total iterations across all sections
	BestIteration      int                    // Iteration that produced best content
	SectionResults     []SectionResult        // Per-section results
	StageResults       map[string]interface{} // Per-stage validation results
	ValidationFeedback string                 // Last validation feedback
	IssueHistory       []int                  // Issue counts per iteration
	ConvergenceBonus   bool                   // Whether bonus iteration was granted
}
