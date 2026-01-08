// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package validators

import (
	"context"
	"regexp"
	"strings"
	"unicode"
)

const (
	styleValidatorName        = "style_validator"
	styleValidatorDescription = "Validates documentation against Elastic style guide (voice, tone, formatting, grammar)"
)

const styleValidatorInstruction = `You are a documentation style validator for Elastic integration packages.
Your task is to validate that the documentation follows the Elastic Style Guide.

## Input
The documentation content to validate is provided in the user message.

## Style Guide Rules

### Voice and Tone
- Use friendly, helpful, conversational tone
- Address users directly with "you" and "your"
- Use contractions (don't, it's, you're) for friendly tone
- Prefer active voice over passive voice

### Emphasis Rules
- **Bold** ONLY for UI elements (buttons, tabs, menu items)
- *Italic* ONLY for introducing new terms
- Backticks for code, commands, file paths, field names

### Grammar Rules
- American English spelling (-ize, -or, -ense)
- Present tense
- Oxford comma (A, B, and C)
- Sentence case for headings

## Output Format
Output a JSON object with this exact structure:
{"valid": true/false, "score": 0-100, "issues": [{"severity": "critical|major|minor", "category": "style", "location": "Section Name", "message": "Issue description", "suggestion": "How to fix"}]}

Set valid=false only for major style violations. Minor style issues should be warnings.

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

	// Check 1: Voice and tone (contractions, direct address)
	result.Issues = append(result.Issues, v.checkVoiceAndTone(content)...)

	// Check 2: Emphasis misuse (bold/italic)
	result.Issues = append(result.Issues, v.checkEmphasisUsage(content)...)

	// Check 3: British vs American English
	result.Issues = append(result.Issues, v.checkAmericanEnglish(content)...)

	// Check 4: Heading case
	result.Issues = append(result.Issues, v.checkHeadingCase(content)...)

	// Check 5: Oxford comma
	result.Issues = append(result.Issues, v.checkOxfordComma(content)...)

	// Determine validity based on issues
	for _, issue := range result.Issues {
		if issue.Severity == SeverityCritical || issue.Severity == SeverityMajor {
			result.Valid = false
			break
		}
	}

	return result, nil
}

// checkVoiceAndTone validates voice and tone requirements
func (v *StyleValidator) checkVoiceAndTone(content string) []ValidationIssue {
	var issues []ValidationIssue
	contentLower := strings.ToLower(content)

	// Check for formal constructions that should use contractions
	formalPatterns := map[string]string{
		`\bdo not\b`:   "don't",
		`\bcannot\b`:   "can't",
		`\bwill not\b`: "won't",
		`\bis not\b`:   "isn't",
		`\bare not\b`:  "aren't",
		`\bdoes not\b`: "doesn't",
		`\bwould not\b`: "wouldn't",
		`\bshould not\b`: "shouldn't",
		`\bcould not\b`: "couldn't",
	}

	formalCount := 0
	for pattern, contraction := range formalPatterns {
		re := regexp.MustCompile(`(?i)` + pattern)
		matches := re.FindAllString(content, -1)
		formalCount += len(matches)
		if len(matches) > 2 {
			issues = append(issues, ValidationIssue{
				Severity:    SeverityMinor,
				Category:    CategoryStyle,
				Location:    "Document",
				Message:     "Consider using contraction '" + contraction + "' for friendlier tone",
				Suggestion:  "Replace formal constructions with contractions where appropriate",
				SourceCheck: "static",
			})
			break // Only report once
		}
	}

	// Check if document addresses user directly
	youCount := strings.Count(contentLower, " you ")
	yourCount := strings.Count(contentLower, " your ")
	if youCount+yourCount < 3 && len(content) > 500 {
		issues = append(issues, ValidationIssue{
			Severity:    SeverityMinor,
			Category:    CategoryStyle,
			Location:    "Document",
			Message:     "Document rarely addresses users directly",
			Suggestion:  "Use 'you' and 'your' to address users directly",
			SourceCheck: "static",
		})
	}

	// Check for impersonal constructions
	impersonalPatterns := []string{
		`(?i)\bthe user\s+(should|must|can|will)\b`,
		`(?i)\busers\s+(should|must|can|will)\b`,
		`(?i)\bone\s+(should|must|can|will)\b`,
	}
	for _, pattern := range impersonalPatterns {
		re := regexp.MustCompile(pattern)
		if re.MatchString(content) {
			issues = append(issues, ValidationIssue{
				Severity:    SeverityMinor,
				Category:    CategoryStyle,
				Location:    "Document",
				Message:     "Found impersonal construction - prefer 'you' over 'the user' or 'users'",
				Suggestion:  "Change 'the user should' to 'you should'",
				SourceCheck: "static",
			})
			break
		}
	}

	return issues
}

// checkEmphasisUsage validates proper use of bold and italic
func (v *StyleValidator) checkEmphasisUsage(content string) []ValidationIssue {
	var issues []ValidationIssue

	// Check for bold text that doesn't look like UI elements
	boldPattern := regexp.MustCompile(`\*\*([^*]+)\*\*`)
	boldMatches := boldPattern.FindAllStringSubmatch(content, -1)

	for _, match := range boldMatches {
		if len(match) > 1 {
			boldText := match[1]
			// UI elements typically: start with capital, are short, may have specific patterns
			// Non-UI elements: long phrases, full sentences, emphasis for importance
			if len(boldText) > 30 || strings.Contains(boldText, ".") {
				issues = append(issues, ValidationIssue{
					Severity:    SeverityMinor,
					Category:    CategoryStyle,
					Location:    "Formatting",
					Message:     "Bold text should only be used for UI elements, not emphasis",
					Suggestion:  "Remove bold from '" + truncateString(boldText, 30) + "' or use *italic* for emphasis",
					SourceCheck: "static",
				})
				break // Only report once
			}
		}
	}

	return issues
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

// checkHeadingCase validates heading case (should be sentence case)
func (v *StyleValidator) checkHeadingCase(content string) []ValidationIssue {
	var issues []ValidationIssue

	// Extract headings
	headingPattern := regexp.MustCompile(`(?m)^(#{1,6})\s+(.+)$`)
	matches := headingPattern.FindAllStringSubmatch(content, -1)

	titleCaseCount := 0
	for _, match := range matches {
		if len(match) > 2 {
			heading := strings.TrimSpace(match[2])
			// Remove any trailing anchor like [anchor-name]
			heading = regexp.MustCompile(`\s*\[[\w-]+\]\s*$`).ReplaceAllString(heading, "")

			if isTitleCase(heading) && !isAllowedTitleCase(heading) {
				titleCaseCount++
			}
		}
	}

	if titleCaseCount > 2 {
		issues = append(issues, ValidationIssue{
			Severity:    SeverityMinor,
			Category:    CategoryStyle,
			Location:    "Headings",
			Message:     "Headings should use sentence case, not Title Case",
			Suggestion:  "Change 'Getting Started With Elastic' to 'Getting started with Elastic'",
			SourceCheck: "static",
		})
	}

	return issues
}

// checkOxfordComma validates Oxford comma usage
func (v *StyleValidator) checkOxfordComma(content string) []ValidationIssue {
	var issues []ValidationIssue

	// Pattern for lists without Oxford comma: "A, B and C" (should be "A, B, and C")
	missingOxfordPattern := regexp.MustCompile(`\b\w+,\s+\w+\s+and\s+\w+`)
	properOxfordPattern := regexp.MustCompile(`\b\w+,\s+\w+,\s+and\s+\w+`)

	missingCount := len(missingOxfordPattern.FindAllString(content, -1))
	properCount := len(properOxfordPattern.FindAllString(content, -1))

	// Only flag if there are missing Oxford commas and none proper ones
	if missingCount > 2 && properCount == 0 {
		issues = append(issues, ValidationIssue{
			Severity:    SeverityMinor,
			Category:    CategoryStyle,
			Location:    "Punctuation",
			Message:     "Use Oxford comma in lists (A, B, and C)",
			Suggestion:  "Add comma before 'and' in lists: 'A, B, and C' instead of 'A, B and C'",
			SourceCheck: "static",
		})
	}

	return issues
}

// Helper functions

// isTitleCase checks if a string appears to be in Title Case
func isTitleCase(s string) bool {
	words := strings.Fields(s)
	if len(words) < 3 {
		return false // Too short to tell
	}

	capitalCount := 0
	for i, word := range words {
		if len(word) == 0 {
			continue
		}
		// Skip common lowercase words in titles
		if i > 0 && isMinorWord(word) {
			continue
		}
		if unicode.IsUpper(rune(word[0])) {
			capitalCount++
		}
	}

	// If most words are capitalized, it's likely Title Case
	return capitalCount > len(words)/2
}

// isMinorWord checks if a word is typically lowercase in titles
func isMinorWord(word string) bool {
	minorWords := []string{"a", "an", "the", "and", "but", "or", "for", "nor",
		"on", "at", "to", "from", "by", "in", "of", "with", "as"}
	wordLower := strings.ToLower(word)
	for _, minor := range minorWords {
		if wordLower == minor {
			return true
		}
	}
	return false
}

// isAllowedTitleCase checks if title case is acceptable (e.g., proper nouns)
func isAllowedTitleCase(heading string) bool {
	allowedPatterns := []string{
		"Elastic", "Elasticsearch", "Kibana", "Logstash", "Beats",
		"Fleet", "Agent", "API", "REST", "HTTP", "JSON", "YAML",
	}
	for _, pattern := range allowedPatterns {
		if strings.Contains(heading, pattern) {
			return true
		}
	}
	return false
}

// truncateString truncates a string to max length with ellipsis
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// min returns the smaller of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

