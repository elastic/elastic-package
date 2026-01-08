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
	accessibilityValidatorName        = "accessibility_validator"
	accessibilityValidatorDescription = "Validates documentation for accessibility and inclusive language"
)

const accessibilityValidatorInstruction = `You are a documentation accessibility validator for Elastic integration packages.
Your task is to validate that the documentation is accessible and uses inclusive language.

## Input
The documentation content to validate is provided in the user message.

## Accessibility Requirements (NON-NEGOTIABLE)

### Alternative Text
- ALL images must have descriptive alt text
- Alt text must describe the image content, not just say "image"

### Meaningful Links
- Link text MUST be descriptive of the destination
- NEVER use "click here", "read more", "here", or "this link"

### Directional Language
- NEVER use "above", "below", "left", "right"
- Refer to content by name: "the following code", "the Save button"

### Inclusive Language
- Use gender-neutral pronouns (they/their, not he/she)
- Address users as "you"

### Ableist and Violent Terms
- DO NOT use: kill, execute, abort, invalid, hack, sanity check, cripple
- Use instead: stop, run, cancel, not valid, workaround, soundness check, impair

## Output Format
Output a JSON object with this exact structure:
{"valid": true/false, "score": 0-100, "issues": [{"severity": "critical|major|minor", "category": "accessibility", "location": "Section Name", "message": "Issue description", "suggestion": "How to fix"}]}

Accessibility issues are critical - set valid=false for any violation.

## IMPORTANT
Output ONLY the JSON object. No other text.`

// AccessibilityValidator validates documentation accessibility and inclusive language
type AccessibilityValidator struct {
	BaseStagedValidator
}

// NewAccessibilityValidator creates a new accessibility validator
func NewAccessibilityValidator() *AccessibilityValidator {
	return &AccessibilityValidator{
		BaseStagedValidator: BaseStagedValidator{
			name:        accessibilityValidatorName,
			description: accessibilityValidatorDescription,
			stage:       StageQuality, // Accessibility is part of quality
			instruction: accessibilityValidatorInstruction,
		},
	}
}

// SupportsStaticValidation returns true - this validator has static checks
func (v *AccessibilityValidator) SupportsStaticValidation() bool {
	return true
}

// StaticValidate performs static accessibility validation
func (v *AccessibilityValidator) StaticValidate(ctx context.Context, content string, pkgCtx *PackageContext) (*StagedValidationResult, error) {
	result := &StagedValidationResult{
		Stage: StageQuality,
		Valid: true,
	}

	// Check 1: Image alt text
	result.Issues = append(result.Issues, v.checkImageAltText(content)...)

	// Check 2: Meaningful link text
	result.Issues = append(result.Issues, v.checkLinkText(content)...)

	// Check 3: Directional language
	result.Issues = append(result.Issues, v.checkDirectionalLanguage(content)...)

	// Check 4: Violent/ableist terms
	result.Issues = append(result.Issues, v.checkInclusiveLanguage(content)...)

	// Check 5: Gender-neutral language
	result.Issues = append(result.Issues, v.checkGenderNeutralLanguage(content)...)

	// Determine validity based on issues
	for _, issue := range result.Issues {
		if issue.Severity == SeverityCritical || issue.Severity == SeverityMajor {
			result.Valid = false
			break
		}
	}

	return result, nil
}

// checkImageAltText validates that images have descriptive alt text
func (v *AccessibilityValidator) checkImageAltText(content string) []ValidationIssue {
	var issues []ValidationIssue

	// Match markdown images: ![alt](url)
	imagePattern := regexp.MustCompile(`!\[([^\]]*)\]\([^)]+\)`)
	matches := imagePattern.FindAllStringSubmatch(content, -1)

	for _, match := range matches {
		if len(match) > 1 {
			altText := strings.TrimSpace(match[1])

			// Check for empty alt text
			if altText == "" {
				issues = append(issues, ValidationIssue{
					Severity:    SeverityCritical,
					Category:    CategoryAccessibility,
					Location:    "Images",
					Message:     "Image missing alt text",
					Suggestion:  "Add descriptive alt text: ![Description of image](url)",
					SourceCheck: "static",
				})
			} else if isNonDescriptiveAlt(altText) {
				// Check for non-descriptive alt text
				issues = append(issues, ValidationIssue{
					Severity:    SeverityMajor,
					Category:    CategoryAccessibility,
					Location:    "Images",
					Message:     "Image has non-descriptive alt text: '" + altText + "'",
					Suggestion:  "Replace with description of what the image shows",
					SourceCheck: "static",
				})
			}
		}
	}

	// Check for HTML images: <img src="..." alt="...">
	htmlImagePattern := regexp.MustCompile(`<img[^>]+>`)
	htmlMatches := htmlImagePattern.FindAllString(content, -1)

	for _, img := range htmlMatches {
		altMatch := regexp.MustCompile(`alt=["']([^"']*)["']`).FindStringSubmatch(img)
		if altMatch == nil || strings.TrimSpace(altMatch[1]) == "" {
			issues = append(issues, ValidationIssue{
				Severity:    SeverityCritical,
				Category:    CategoryAccessibility,
				Location:    "Images",
				Message:     "HTML image missing alt attribute",
				Suggestion:  "Add alt attribute: <img src=\"...\" alt=\"Description\">",
				SourceCheck: "static",
			})
		}
	}

	return issues
}

// checkLinkText validates that link text is meaningful
func (v *AccessibilityValidator) checkLinkText(content string) []ValidationIssue {
	var issues []ValidationIssue

	// Match markdown links: [text](url)
	linkPattern := regexp.MustCompile(`\[([^\]]+)\]\([^)]+\)`)
	matches := linkPattern.FindAllStringSubmatch(content, -1)

	badLinkTexts := []string{
		"click here", "here", "read more", "more", "this link",
		"this page", "link", "this", "learn more",
	}

	for _, match := range matches {
		if len(match) > 1 {
			linkText := strings.TrimSpace(strings.ToLower(match[1]))

			for _, bad := range badLinkTexts {
				if linkText == bad {
					issues = append(issues, ValidationIssue{
						Severity:    SeverityCritical,
						Category:    CategoryAccessibility,
						Location:    "Links",
						Message:     "Non-descriptive link text: '" + match[1] + "'",
						Suggestion:  "Use descriptive text that indicates where the link goes",
						SourceCheck: "static",
					})
					break
				}
			}
		}
	}

	return issues
}

// checkDirectionalLanguage validates no directional references
func (v *AccessibilityValidator) checkDirectionalLanguage(content string) []ValidationIssue {
	var issues []ValidationIssue

	// Directional terms to flag
	directionalPatterns := []struct {
		pattern     string
		term        string
		replacement string
	}{
		{`(?i)\b(see|shown|displayed|found)\s+(above|below)\b`, "above/below", "the following/preceding"},
		{`(?i)\bthe\s+(above|below)\s+(image|figure|table|code|example)\b`, "above/below", "the following/preceding"},
		{`(?i)\bon\s+the\s+(left|right)\b`, "left/right", "specific element name"},
		{`(?i)\b(left|right)[\s-]hand\s+side\b`, "left/right-hand side", "specific element name"},
		{`(?i)\bto\s+the\s+(left|right)\s+of\b`, "to the left/right of", "next to [element name]"},
		{`(?i)\babove\s+and\s+below\b`, "above and below", "preceding and following"},
	}

	for _, dp := range directionalPatterns {
		re := regexp.MustCompile(dp.pattern)
		if re.MatchString(content) {
			issues = append(issues, ValidationIssue{
				Severity:    SeverityMajor,
				Category:    CategoryAccessibility,
				Location:    "Content",
				Message:     "Found directional language: '" + dp.term + "'",
				Suggestion:  "Replace with content reference: " + dp.replacement,
				SourceCheck: "static",
			})
		}
	}

	return issues
}

// checkInclusiveLanguage validates no violent or ableist terms
func (v *AccessibilityValidator) checkInclusiveLanguage(content string) []ValidationIssue {
	var issues []ValidationIssue

	// Terms to replace (but with context exceptions)
	problematicTerms := map[string]string{
		`\bkill\b`:           "stop or terminate",
		`\bkills\b`:          "stops or terminates",
		`\bkilled\b`:         "stopped or terminated",
		`\bkilling\b`:        "stopping or terminating",
		`\babort\b`:          "cancel or stop",
		`\baborts\b`:         "cancels or stops",
		`\baborted\b`:        "canceled or stopped",
		`\baborting\b`:       "canceling or stopping",
		`\bhack\b`:           "workaround",
		`\bhacks\b`:          "workarounds",
		`\bhacking\b`:        "working around",
		`\bsanity\s+check\b`: "soundness check",
		`\bsanity\s+test\b`:  "soundness test",
		`\bblacklist\b`:      "blocklist or denylist",
		`\bwhitelist\b`:      "allowlist",
		`\bcripple\b`:        "impair or disable",
		`\bcrippled\b`:       "impaired or disabled",
	}

	// Terms that are problematic unless in specific technical contexts
	contextualTerms := map[string]struct {
		replacement string
		exceptions  []string // Context patterns that make the term acceptable
	}{
		`\bdummy\b`: {
			replacement: "placeholder or sample",
			exceptions: []string{
				"dummy values",     // Technical term for placeholder values from APIs
				"dummy data",       // Technical term for test data
				"dummy certificate", // Technical term for test certificates
			},
		},
		`\binvalid\b`: {
			replacement: "not valid",
			exceptions: []string{
				"invalid request",  // HTTP status code context
				"invalid response", // API response context
				"invalid input",    // Validation context
				"invalid format",   // Data format context
				"invalid json",     // JSON parsing context
			},
		},
		`\bexecute\b`: {
			replacement: "run",
			exceptions: []string{
				"execute query",   // Database context
				"execute command", // CLI context
			},
		},
		`\bmaster\b`: {
			replacement: "main or primary",
			exceptions: []string{
				"master node",   // Elasticsearch context (historical)
				"master branch", // Git context (historical)
			},
		},
		`\bslave\b`: {
			replacement: "replica or secondary",
			exceptions: []string{
				"slave node", // Database context (historical)
			},
		},
	}

	contentLower := strings.ToLower(content)

	// Check non-contextual problematic terms
	for term, replacement := range problematicTerms {
		re := regexp.MustCompile(`(?i)` + term)
		matches := re.FindAllString(content, -1)
		if len(matches) > 0 {
			issues = append(issues, ValidationIssue{
				Severity:    SeverityMajor,
				Category:    CategoryAccessibility,
				Location:    "Language",
				Message:     "Found potentially problematic term: '" + matches[0] + "'",
				Suggestion:  "Consider using: " + replacement,
				SourceCheck: "static",
			})
		}
	}

	// Check contextual terms - allow exceptions
	for term, config := range contextualTerms {
		re := regexp.MustCompile(`(?i)` + term)
		matches := re.FindAllString(content, -1)
		if len(matches) > 0 {
			// Check if any exception context exists
			hasException := false
			for _, exception := range config.exceptions {
				if strings.Contains(contentLower, strings.ToLower(exception)) {
					hasException = true
					break
				}
			}

			// Only flag if no exception context found
			if !hasException {
				issues = append(issues, ValidationIssue{
					Severity:    SeverityMinor, // Reduced severity for contextual terms
					Category:    CategoryAccessibility,
					Location:    "Language",
					Message:     "Found potentially problematic term: '" + matches[0] + "'",
					Suggestion:  "Consider using: " + config.replacement,
					SourceCheck: "static",
				})
			}
		}
	}

	return issues
}

// checkGenderNeutralLanguage validates gender-neutral language
func (v *AccessibilityValidator) checkGenderNeutralLanguage(content string) []ValidationIssue {
	var issues []ValidationIssue

	// Gendered pronouns to flag
	genderedPatterns := []struct {
		pattern     string
		replacement string
	}{
		{`\bhe\s+or\s+she\b`, "they"},
		{`\bshe\s+or\s+he\b`, "they"},
		{`\bhis\s+or\s+her\b`, "their"},
		{`\bher\s+or\s+his\b`, "their"},
		{`\bhis/her\b`, "their"},
		{`\bhe/she\b`, "they"},
		{`\bs/he\b`, "they"},
		{`\bhimself\s+or\s+herself\b`, "themselves"},
	}

	for _, gp := range genderedPatterns {
		re := regexp.MustCompile(`(?i)` + gp.pattern)
		if re.MatchString(content) {
			issues = append(issues, ValidationIssue{
				Severity:    SeverityMinor,
				Category:    CategoryAccessibility,
				Location:    "Language",
				Message:     "Found gendered language pattern",
				Suggestion:  "Use gender-neutral pronoun: " + gp.replacement,
				SourceCheck: "static",
			})
		}
	}

	return issues
}

// isNonDescriptiveAlt checks if alt text is non-descriptive
func isNonDescriptiveAlt(alt string) bool {
	nonDescriptive := []string{
		"image", "img", "picture", "photo", "screenshot",
		"figure", "diagram", "icon", "logo", "graphic",
	}

	altLower := strings.ToLower(alt)
	for _, nd := range nonDescriptive {
		if altLower == nd || altLower == nd+"s" {
			return true
		}
	}

	return false
}

