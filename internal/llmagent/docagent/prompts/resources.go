// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

// Package prompts provides embedded prompt templates, shared style rules,
// and loading utilities for LLM documentation generation.
package prompts

import _ "embed"

//go:embed revision_prompt.txt
var RevisionPrompt string

//go:embed section_generation_prompt.txt
var SectionGenerationPrompt string

//go:embed critical_formatting_rules.txt
var CriticalFormattingRules string

//go:embed critic_rejection_criteria.txt
var CriticRejectionCriteria string

//go:embed full_formatting_rules.txt
var FullFormattingRules string
