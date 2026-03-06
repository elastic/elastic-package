// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package prompts

import "fmt"

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

// ValidatorOutputSuffix returns the standard JSON output format section
// shared by all validators. category is the issue category (e.g. "style"),
// and validGuidance explains when to set valid=false.
func ValidatorOutputSuffix(category, validGuidance string) string {
	return fmt.Sprintf(`
## Output Format
Output a JSON object with this exact structure:
{"valid": true/false, "score": 0-100, "issues": [{"severity": "critical|major|minor", "category": "%s", "location": "Section Name", "message": "Issue description", "suggestion": "How to fix"}]}

%s

## IMPORTANT
Output ONLY the JSON object. No other text.`, category, validGuidance)
}
