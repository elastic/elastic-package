// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package prompts

// Type represents the type of prompt to load
type Type int

const (
	TypeRevision Type = iota
	TypeSectionGeneration
)

// Load returns the embedded prompt content for the given type
func Load(promptType Type) string {
	switch promptType {
	case TypeRevision:
		return RevisionPrompt
	case TypeSectionGeneration:
		return SectionGenerationPrompt
	default:
		return ""
	}
}
