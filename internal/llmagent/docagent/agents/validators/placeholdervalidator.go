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
	placeholderValidatorName        = "placeholder_validator"
	placeholderValidatorDescription = "Validates proper use of placeholders for missing information"
)

// StandardPlaceholder is the expected format for missing information
const StandardPlaceholder = "<!-- INFORMATION NOT AVAILABLE - PLEASE UPDATE -->"

const placeholderValidatorInstruction = `You are a documentation placeholder validator for Elastic integration packages.
Your task is to validate that placeholders are used correctly for missing information.

## Input
The documentation content to validate is provided in the user message.

## Checks
1. Placeholders should only be used when information is genuinely unavailable
2. The exact format must be: <!-- INFORMATION NOT AVAILABLE - PLEASE UPDATE -->
3. No TODO comments or informal placeholders (e.g., [TBD], <INSERT>, etc.)
4. Critical missing information should be flagged for research
5. Placeholders should not appear in code blocks (invalid syntax)
6. Each placeholder should be in a logical location

## Placeholder Rules
- Use placeholders sparingly - only for truly unavailable information
- Never use placeholders for information that can be derived from the package
- If a URL is invalid, use the standard placeholder
- If a feature is unknown, use the standard placeholder

## Output Format
Output a JSON object with this exact structure:
{"valid": true/false, "score": 0-100, "issues": [{"severity": "critical|major|minor", "category": "placeholders", "location": "Section Name", "message": "Issue description", "suggestion": "How to fix"}]}

Set valid=false if informal placeholders are found or if critical information uses placeholders.

## IMPORTANT
Output ONLY the JSON object. No other text.`

// PlaceholderValidator validates placeholder usage (Section F)
type PlaceholderValidator struct {
	BaseStagedValidator
}

// NewPlaceholderValidator creates a new placeholder validator
func NewPlaceholderValidator() *PlaceholderValidator {
	return &PlaceholderValidator{
		BaseStagedValidator: BaseStagedValidator{
			name:        placeholderValidatorName,
			description: placeholderValidatorDescription,
			stage:       StagePlaceholders,
			scope:       ScopeBoth, // Placeholder validation works on sections and full document
			instruction: placeholderValidatorInstruction,
		},
	}
}

// SupportsStaticValidation returns true - this validator has static checks
func (v *PlaceholderValidator) SupportsStaticValidation() bool {
	return true
}

// StaticValidate performs static placeholder validation
func (v *PlaceholderValidator) StaticValidate(ctx context.Context, content string, pkgCtx *PackageContext) (*StagedValidationResult, error) {
	result := &StagedValidationResult{
		Stage: StagePlaceholders,
		Valid: true,
	}

	// Check 1: Informal placeholders
	result.Issues = append(result.Issues, v.checkInformalPlaceholders(content)...)

	// Check 2: Standard placeholder count and locations
	result.Issues = append(result.Issues, v.checkStandardPlaceholders(content)...)

	// Check 3: Placeholders in code blocks
	result.Issues = append(result.Issues, v.checkPlaceholdersInCode(content)...)

	// Check 4: Critical sections with placeholders
	result.Issues = append(result.Issues, v.checkCriticalPlaceholders(content)...)

	// Determine validity based on issues
	for _, issue := range result.Issues {
		if issue.Severity == SeverityCritical || issue.Severity == SeverityMajor {
			result.Valid = false
			break
		}
	}

	return result, nil
}

// checkInformalPlaceholders finds non-standard placeholder formats
func (v *PlaceholderValidator) checkInformalPlaceholders(content string) []ValidationIssue {
	var issues []ValidationIssue

	// Patterns for informal placeholders
	informalPatterns := []struct {
		pattern string
		name    string
	}{
		{`\[TBD\]`, "[TBD]"},
		{`\[TODO\]`, "[TODO]"},
		{`\[PLACEHOLDER\]`, "[PLACEHOLDER]"},
		{`\[INSERT.*?\]`, "[INSERT...]"},
		{`\[ADD.*?\]`, "[ADD...]"},
		{`\[FILL.*?\]`, "[FILL...]"},
		{`\[YOUR.*?\]`, "[YOUR...]"},
		{`<INSERT.*?>`, "<INSERT...>"},
		{`<ADD.*?>`, "<ADD...>"},
		{`<YOUR.*?>`, "<YOUR...>"},
		{`<TBD>`, "<TBD>"},
		{`<PLACEHOLDER>`, "<PLACEHOLDER>"},
		{`___+`, "blank line (___...)"},
		{`\.\.\.\s*\(to be added\)`, "...(to be added)"},
		{`\?\?\?`, "???"},
	}

	for _, informal := range informalPatterns {
		re := regexp.MustCompile(`(?i)` + informal.pattern)
		if re.MatchString(content) {
			matches := re.FindAllString(content, 3) // Limit to first 3
			issues = append(issues, ValidationIssue{
				Severity:    SeverityCritical,
				Category:    CategoryPlaceholders,
				Location:    "Document",
				Message:     "Found informal placeholder: " + informal.name,
				Suggestion:  "Use standard format: " + StandardPlaceholder,
				SourceCheck: "static",
			})
			// Only report first few matches
			if len(matches) > 1 {
				issues[len(issues)-1].Message += " (and more)"
			}
		}
	}

	return issues
}

// checkStandardPlaceholders counts and validates standard placeholders
func (v *PlaceholderValidator) checkStandardPlaceholders(content string) []ValidationIssue {
	var issues []ValidationIssue

	// Count standard placeholders
	standardPattern := regexp.MustCompile(regexp.QuoteMeta(StandardPlaceholder))
	matches := standardPattern.FindAllStringIndex(content, -1)

	placeholderCount := len(matches)

	// Flag if too many placeholders (suggests incomplete documentation)
	if placeholderCount > 5 {
		issues = append(issues, ValidationIssue{
			Severity:    SeverityMajor,
			Category:    CategoryPlaceholders,
			Location:    "Document",
			Message:     "High number of placeholders found: " + string(rune('0'+placeholderCount)),
			Suggestion:  "Research and fill in missing information where possible",
			SourceCheck: "static",
		})
	}

	// Check for slight variations of standard placeholder
	variations := []string{
		`<<\s*INFORMATION\s+NOT\s+AVAILABLE\s*>>`,
		`<\s*INFORMATION\s+NOT\s+AVAILABLE\s*>`,
		`\[\s*INFORMATION\s+NOT\s+AVAILABLE\s*\]`,
	}

	for _, variation := range variations {
		re := regexp.MustCompile(`(?i)` + variation)
		if re.MatchString(content) {
			// Check if it's not the exact standard format
			exactMatches := standardPattern.FindAllString(content, -1)
			varMatches := re.FindAllString(content, -1)
			if len(varMatches) > len(exactMatches) {
				issues = append(issues, ValidationIssue{
					Severity:    SeverityMajor,
					Category:    CategoryPlaceholders,
					Location:    "Document",
					Message:     "Non-standard placeholder format detected",
					Suggestion:  "Use exact format: " + StandardPlaceholder,
					SourceCheck: "static",
				})
			}
		}
	}

	return issues
}

// checkPlaceholdersInCode finds placeholders inside code blocks
func (v *PlaceholderValidator) checkPlaceholdersInCode(content string) []ValidationIssue {
	var issues []ValidationIssue

	// Extract code blocks
	codeBlockPattern := regexp.MustCompile("(?s)```[a-z]*\n(.*?)```")
	codeBlocks := codeBlockPattern.FindAllStringSubmatch(content, -1)

	placeholderPattern := regexp.MustCompile(regexp.QuoteMeta(StandardPlaceholder))

	for _, block := range codeBlocks {
		if len(block) > 1 {
			if placeholderPattern.MatchString(block[1]) {
				issues = append(issues, ValidationIssue{
					Severity:    SeverityCritical,
					Category:    CategoryPlaceholders,
					Location:    "Code block",
					Message:     "Placeholder found inside code block (invalid syntax)",
					Suggestion:  "Move placeholder outside code block or provide actual value",
					SourceCheck: "static",
				})
			}
		}
	}

	return issues
}

// checkCriticalPlaceholders identifies placeholders in critical sections
func (v *PlaceholderValidator) checkCriticalPlaceholders(content string) []ValidationIssue {
	var issues []ValidationIssue

	// Critical sections that should not have placeholders
	criticalSections := []struct {
		pattern string
		name    string
	}{
		{`(?s)##\s*Overview.*?(?:##|$)`, "Overview"},
		{`(?s)##\s*Setup.*?(?:##|$)`, "Setup"},
		{`(?s)##\s*Prerequisites.*?(?:##|$)`, "Prerequisites"},
	}

	placeholderPattern := regexp.MustCompile(regexp.QuoteMeta(StandardPlaceholder))

	for _, section := range criticalSections {
		re := regexp.MustCompile(`(?i)` + section.pattern)
		match := re.FindString(content)
		if match != "" && placeholderPattern.MatchString(match) {
			issues = append(issues, ValidationIssue{
				Severity:    SeverityMajor,
				Category:    CategoryPlaceholders,
				Location:    section.name,
				Message:     "Critical section contains placeholder",
				Suggestion:  "The " + section.name + " section should not have missing information",
				SourceCheck: "static",
			})
		}
	}

	return issues
}

// CountPlaceholders returns the number of standard placeholders in content
func CountPlaceholders(content string) int {
	pattern := regexp.MustCompile(regexp.QuoteMeta(StandardPlaceholder))
	return len(pattern.FindAllString(content, -1))
}

// ReplacePlaceholder replaces a placeholder with actual content
func ReplacePlaceholder(content, replacement string) string {
	return strings.Replace(content, StandardPlaceholder, replacement, 1)
}
