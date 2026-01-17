// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package validators

import (
	"context"
	"regexp"
	"strings"
)

const (
	styleValidatorName        = "style_validator"
	styleValidatorDescription = "Validates documentation against Elastic style guide (voice, tone, formatting, grammar)"
)

const styleValidatorInstruction = `You are a documentation style validator for Elastic integration packages.
Your task is to validate ONLY critical style issues that significantly impact readability.

## Input
The documentation content to validate is provided in the user message.

## Style Guide Rules - CHECK ONLY THESE:

### American English
- Use American English spelling (-ize, -or, -ense)
- Example: "organization" not "organisation", "color" not "colour"

### Technical Accuracy
- Code examples should be complete and syntactically correct
- File paths and commands should be properly formatted with backticks
- API endpoints and URLs should be valid format

## DO NOT CHECK (these are acceptable):
- Contractions vs formal language (both are fine)
- Bold vs italic usage (author's choice)
- Heading case (Title Case or sentence case are both acceptable)
- Oxford comma (optional)
- AI-generated disclaimers (these are intentionally added)

## Output Format
Output a JSON object with this exact structure:
{"valid": true/false, "score": 0-100, "issues": [{"severity": "critical|major|minor", "category": "style", "location": "Section Name", "message": "Issue description", "suggestion": "How to fix"}]}

Set valid=true unless there are significant readability issues.
Most documents should pass - only flag truly problematic content.

## IMPORTANT
Output ONLY the JSON object. No other text.`

// StyleValidator validates documentation against the Elastic Style Guide
type StyleValidator struct {
	BaseStagedValidator
}

// NewStyleValidator creates a new style validator
func NewStyleValidator() *StyleValidator {
	return &StyleValidator{
		BaseStagedValidator: BaseStagedValidator{
			name:        styleValidatorName,
			description: styleValidatorDescription,
			stage:       StageQuality, // Style is part of quality
			scope:       ScopeBoth,    // Style validation works on sections and full document
			instruction: styleValidatorInstruction,
		},
	}
}

// SupportsStaticValidation returns true - this validator has static checks
func (v *StyleValidator) SupportsStaticValidation() bool {
	return true
}

// StaticValidate performs static style validation
func (v *StyleValidator) StaticValidate(ctx context.Context, content string, pkgCtx *PackageContext) (*StagedValidationResult, error) {
	result := &StagedValidationResult{
		Stage: StageQuality,
		Valid: true,
	}

	result.Issues = append(result.Issues, v.checkAmericanEnglish(content)...)

	for _, issue := range result.Issues {
		if issue.Severity == SeverityCritical || issue.Severity == SeverityMajor {
			result.Valid = false
			break
		}
	}

	return result, nil
}

// checkAmericanEnglish validates American English spelling
func (v *StyleValidator) checkAmericanEnglish(content string) []ValidationIssue {
	var issues []ValidationIssue

	// British spellings and their American equivalents
	britishToAmerican := map[string]string{
		`\bcolour\b`:       "color",
		`\bfavourite\b`:    "favorite",
		`\bhonour\b`:       "honor",
		`\blabour\b`:       "labor",
		`\bneighbour\b`:    "neighbor",
		`\borganisation\b`: "organization",
		`\borganise\b`:     "organize",
		`\brecognise\b`:    "recognize",
		`\brealise\b`:      "realize",
		`\bauthorise\b`:    "authorize",
		`\bcustomise\b`:    "customize",
		`\boptimise\b`:     "optimize",
		`\bsynchronise\b`:  "synchronize",
		`\banalyse\b`:      "analyze",
		`\bcatalogue\b`:    "catalog",
		`\bdialogue\b`:     "dialog",
		`\bcentre\b`:       "center",
		`\bfibre\b`:        "fiber",
		`\blitre\b`:        "liter",
		`\bmetre\b`:        "meter",
		`\blicence\b`:      "license",
		`\bdefence\b`:      "defense",
		`\boffence\b`:      "offense",
		`\bpractise\b`:     "practice",
		`\bcancelled\b`:    "canceled",
		`\bmodelled\b`:     "modeled",
		`\btravelled\b`:    "traveled",
	}

	britishFound := []string{}
	for british, american := range britishToAmerican {
		re := regexp.MustCompile(`(?i)` + british)
		if re.MatchString(content) {
			britishFound = append(britishFound, american)
		}
	}

	if len(britishFound) > 0 {
		issues = append(issues, ValidationIssue{
			Severity:    SeverityMinor,
			Category:    CategoryStyle,
			Location:    "Spelling",
			Message:     "Found British English spelling - use American English",
			Suggestion:  "Use American spellings: " + strings.Join(britishFound[:min(3, len(britishFound))], ", "),
			SourceCheck: "static",
		})
	}

	return issues
}

// min returns the smaller of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
