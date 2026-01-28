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
	qualityValidatorName        = "quality_validator"
	qualityValidatorDescription = "Validates writing quality, clarity, and professional tone"
)

const qualityValidatorInstruction = `You are a documentation quality validator for Elastic integration packages.
Your task is to validate ONLY critical quality issues that significantly impact usability.

## Input
The documentation content to validate is provided in the user message.

## CHECK ONLY THESE CRITICAL ISSUES:
1. Hallucinated features - capabilities that don't exist
2. Incorrect version numbers or compatibility claims
3. Missing critical setup steps that would cause failure
4. Code examples that are syntactically broken
5. Instructions that would lead to errors

## DO NOT CHECK (these are acceptable and should NOT be flagged):
- YAML frontmatter or "applies_to" fields (README.md files don't use frontmatter)
- Private IP addresses like 192.168.x.x, 10.x.x.x in examples (RFC 1918 addresses are standard practice)
- Passive vs active voice (both are fine)
- Use of "simply", "just", "easily" (common and acceptable)
- Writing style preferences
- Section length (author's discretion)
- AI-generated disclaimers (intentionally added)
- Minor phrasing variations
- Vendor GUI field names that may vary by version
- Documentation links pointing to older API versions - users can navigate to current docs
- Capitalization of UI element names - these match what users see in the vendor GUI
- Product name variations due to vendor rebranding - all historical names are valid
- Minor version mismatches in external documentation links

## Output Format
Output a JSON object with this exact structure:
{"valid": true/false, "score": 0-100, "issues": [{"severity": "critical|major|minor", "category": "quality", "location": "Section Name", "message": "Issue description", "suggestion": "How to fix"}]}

Set valid=true unless there are CRITICAL issues that would cause user failure.
Most documentation should pass validation.

## IMPORTANT
Output ONLY the JSON object. No other text.`

// QualityValidator validates writing quality and clarity (Section E)
type QualityValidator struct {
	BaseStagedValidator
}

// NewQualityValidator creates a new quality validator
func NewQualityValidator() *QualityValidator {
	return &QualityValidator{
		BaseStagedValidator: BaseStagedValidator{
			name:        qualityValidatorName,
			description: qualityValidatorDescription,
			stage:       StageQuality,
			scope:       ScopeBoth, // Quality validation works on sections and full document
			instruction: qualityValidatorInstruction,
		},
	}
}

// SupportsStaticValidation returns true - this validator has static checks
func (v *QualityValidator) SupportsStaticValidation() bool {
	return true
}

// StaticValidate performs static quality validation
func (v *QualityValidator) StaticValidate(ctx context.Context, content string, pkgCtx *PackageContext) (*StagedValidationResult, error) {
	result := &StagedValidationResult{
		Stage: StageQuality,
		Valid: true,
	}

	// Check 1: TODO/FIXME markers (inline comments like "// TODO:")
	result.Issues = append(result.Issues, v.checkTodoMarkers(content)...)

	// Check 2: Generic/vague phrases
	result.Issues = append(result.Issues, v.checkVaguePhrases(content)...)

	// Check 2b: Misleading phrases (major issues)
	result.Issues = append(result.Issues, v.checkMisleadingPhrases(content)...)

	// Note: Incomplete content/placeholder checks removed - now handled by PlaceholderValidator
	// to avoid duplicate validation of [INSERT...], <ADD...>, ???, etc.

	// Check 3: Excessive passive voice (basic check)
	result.Issues = append(result.Issues, v.checkPassiveVoice(content)...)

	// Check 4: Very short sections
	result.Issues = append(result.Issues, v.checkSectionLength(content)...)

	// Determine validity based on issues
	for _, issue := range result.Issues {
		if issue.Severity == SeverityCritical || issue.Severity == SeverityMajor {
			result.Valid = false
			break
		}
	}

	return result, nil
}

// checkTodoMarkers finds TODO/FIXME comments that shouldn't be in final docs
func (v *QualityValidator) checkTodoMarkers(content string) []ValidationIssue {
	var issues []ValidationIssue

	todoPatterns := []string{
		`(?i)\bTODO\b`,
		`(?i)\bFIXME\b`,
		`(?i)\bHACK\b`,
		`(?i)\bXXX\b`,
		`(?i)\bWIP\b`,
		`(?i)\bTBD\b`,
	}

	for _, pattern := range todoPatterns {
		re := regexp.MustCompile(pattern)
		if re.MatchString(content) {
			matches := re.FindAllString(content, -1)
			issues = append(issues, ValidationIssue{
				Severity:    SeverityCritical,
				Category:    CategoryQuality,
				Location:    "Document",
				Message:     "Found TODO/development marker: " + matches[0],
				Suggestion:  "Remove or resolve TODO comments before publishing",
				SourceCheck: "static",
			})
		}
	}

	return issues
}

// checkVaguePhrases identifies vague or non-specific language
func (v *QualityValidator) checkVaguePhrases(content string) []ValidationIssue {
	var issues []ValidationIssue

	vaguePhrases := map[string]string{
		`(?i)\bsimply\s+`:          "Avoid 'simply' - it dismisses complexity",
		`(?i)\bobviously\s+`:       "Avoid 'obviously' - explain clearly instead",
		`(?i)\bas\s+needed\b`:      "Be specific about when/what is needed",
		`(?i)\bas\s+appropriate\b`: "Be specific about what is appropriate",
	}

	for pattern, suggestion := range vaguePhrases {
		re := regexp.MustCompile(pattern)
		if re.MatchString(content) {
			issues = append(issues, ValidationIssue{
				Severity:    SeverityMinor,
				Category:    CategoryQuality,
				Location:    "Document",
				Message:     suggestion,
				Suggestion:  "Replace with specific, actionable language",
				SourceCheck: "static",
			})
		}
	}

	return issues
}

// checkMisleadingPhrases identifies phrases that could confuse users
func (v *QualityValidator) checkMisleadingPhrases(content string) []ValidationIssue {
	var issues []ValidationIssue

	misleadingPhrases := map[string]string{
		`(?i)choose\s+your\s+destination`: "In Kibana setup, use 'choose the setup instructions that match your configuration' instead of 'choose your destination' (destination is configured in vendor setup)",
		`(?i)select\s+your\s+destination`: "In Kibana setup, use 'choose the setup instructions that match your configuration' instead of 'select your destination' (destination is configured in vendor setup)",
	}

	for pattern, message := range misleadingPhrases {
		re := regexp.MustCompile(pattern)
		if re.MatchString(content) {
			issues = append(issues, ValidationIssue{
				Severity:    SeverityMajor,
				Category:    CategoryQuality,
				Location:    "Set up steps in Kibana",
				Message:     message,
				Suggestion:  "Rewrite to clarify that users are choosing configuration instructions, not setting the data destination",
				SourceCheck: "static",
			})
		}
	}

	return issues
}

// checkPassiveVoice does basic passive voice detection
func (v *QualityValidator) checkPassiveVoice(content string) []ValidationIssue {
	var issues []ValidationIssue

	// Common passive voice patterns
	passivePatterns := []string{
		`(?i)\b(?:is|are|was|were|be|been|being)\s+(?:configured|installed|enabled|set|defined|used|required|needed|supported)\b`,
	}

	passiveCount := 0
	for _, pattern := range passivePatterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindAllString(content, -1)
		passiveCount += len(matches)
	}

	// Only flag if excessive passive voice (more than 10 instances)
	if passiveCount > 10 {
		issues = append(issues, ValidationIssue{
			Severity:    SeverityMinor,
			Category:    CategoryQuality,
			Location:    "Document",
			Message:     "High use of passive voice detected",
			Suggestion:  "Consider rewriting in active voice (e.g., 'Configure X' instead of 'X is configured')",
			SourceCheck: "static",
		})
	}

	return issues
}

// checkSectionLength identifies very short sections that may need expansion
func (v *QualityValidator) checkSectionLength(content string) []ValidationIssue {
	var issues []ValidationIssue

	// Split by major headings (##)
	sectionPattern := regexp.MustCompile(`(?m)^##\s+(.+)$`)
	matches := sectionPattern.FindAllStringSubmatchIndex(content, -1)

	for i, match := range matches {
		sectionStart := match[0]
		sectionEnd := len(content)
		if i+1 < len(matches) {
			sectionEnd = matches[i+1][0]
		}

		// Get section name
		sectionName := content[match[2]:match[3]]

		// Get section content
		sectionContent := content[sectionStart:sectionEnd]

		// Remove the heading line
		lines := strings.Split(sectionContent, "\n")
		if len(lines) > 1 {
			contentLines := lines[1:]
			nonEmptyLines := 0
			for _, line := range contentLines {
				if strings.TrimSpace(line) != "" {
					nonEmptyLines++
				}
			}

			// Flag very short sections (less than 3 non-empty lines)
			if nonEmptyLines < 3 && nonEmptyLines > 0 {
				issues = append(issues, ValidationIssue{
					Severity:    SeverityMinor,
					Category:    CategoryQuality,
					Location:    sectionName,
					Message:     "Section is very short and may need more detail",
					Suggestion:  "Consider expanding this section with more specific information",
					SourceCheck: "static",
				})
			}
		}
	}

	return issues
}
