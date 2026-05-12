// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

// Package validators provides validation agents and types for documentation validation.
// Staged validators (structure, accuracy, completeness, quality, placeholder, style,
// accessibility) provide comprehensive static and LLM-based validation.
package validators

// ValidationResult represents a simple validation result with issues and warnings.
// For more detailed validation, use StagedValidationResult instead.
type ValidationResult struct {
	Valid    bool     `json:"valid"`
	Issues   []string `json:"issues"`
	Warnings []string `json:"warnings"`
}
