// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

// Package validation provides a simplified entry point for documentation validation.
// It re-exports types from docagent/specialists/validators for convenience.
package validation

import (
	"github.com/elastic/elastic-package/internal/llmagent/docagent/specialists/validators"
)

// Re-export core types from validators package for convenience
type (
	// SectionContext holds the context needed for section generation
	SectionContext = validators.SectionContext
	// AgentConfig holds common configuration for building agents
	AgentConfig = validators.AgentConfig
	// SectionAgent defines the interface for documentation workflow agents
	SectionAgent = validators.SectionAgent
	// AgentResult represents the result of an agent's execution
	AgentResult = validators.AgentResult
	// ValidationScope indicates what level of validation a validator performs
	ValidationScope = validators.ValidationScope
)

// Re-export state key constants
const (
	StateKeyContent        = validators.StateKeyContent
	StateKeyFeedback       = validators.StateKeyFeedback
	StateKeyValidation     = validators.StateKeyValidation
	StateKeyURLCheck       = validators.StateKeyURLCheck
	StateKeyApproved       = validators.StateKeyApproved
	StateKeyIteration      = validators.StateKeyIteration
	StateKeySectionContext = validators.StateKeySectionContext
)

// Re-export scope constants
const (
	ScopeSectionLevel = validators.ScopeSectionLevel
	ScopeFullDocument = validators.ScopeFullDocument
	ScopeBoth         = validators.ScopeBoth
)
