// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package validators

import (
	"context"
	"regexp"
	"strings"

	"github.com/elastic/elastic-package/internal/llmagent/docagent/stylerules"
)

const (
	styleValidatorName        = "style_validator"
	styleValidatorDescription = "Validates documentation against Elastic style guide (voice, tone, formatting, grammar)"
)

// styleValidatorInstructionPrefix is the first part of the style validator instruction
const styleValidatorInstructionPrefix = `You are a documentation style validator for Elastic integration packages.
Your task is to validate critical style issues that significantly impact readability.

## Input
The documentation content to validate is provided in the user message.

`

// styleValidatorInstructionSuffix is the final part of the style validator instruction
const styleValidatorInstructionSuffix = `
## Output Format
Output a JSON object with this exact structure:
{"valid": true/false, "score": 0-100, "issues": [{"severity": "critical|major|minor", "category": "style", "location": "Section Name", "message": "Issue description", "suggestion": "How to fix"}]}

Set valid=false for critical formatting issues.

## IMPORTANT
Output ONLY the JSON object. No other text.`

// StyleValidator validates documentation against the Elastic Style Guide
type StyleValidator struct {
	BaseStagedValidator
}

// NewStyleValidator creates a new style validator
func NewStyleValidator() *StyleValidator {
	// Build the full instruction by combining prefix, shared formatting rules, and suffix
	instruction := styleValidatorInstructionPrefix + stylerules.FullFormattingRules + styleValidatorInstructionSuffix

	return &StyleValidator{
		BaseStagedValidator: BaseStagedValidator{
			name:        styleValidatorName,
			description: styleValidatorDescription,
			stage:       StageQuality, // Style is part of quality
			scope:       ScopeBoth,    // Style validation works on sections and full document
			instruction: instruction,
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

	prose := stripCodeBlocks(content)

	result.Issues = append(result.Issues, v.checkAmericanEnglish(content)...)
	result.Issues = append(result.Issues, v.checkDontUse(prose)...)
	result.Issues = append(result.Issues, v.checkLatinisms(prose)...)
	result.Issues = append(result.Issues, v.checkExclamation(prose)...)
	result.Issues = append(result.Issues, v.checkEllipses(prose)...)
	result.Issues = append(result.Issues, v.checkVersionTerms(prose)...)
	result.Issues = append(result.Issues, v.checkArticles(prose)...)

	for _, issue := range result.Issues {
		if issue.Severity == SeverityCritical || issue.Severity == SeverityMajor {
			result.Valid = false
			break
		}
	}

	return result, nil
}

// stripCodeBlocks removes fenced code blocks so static checks only run on prose.
func stripCodeBlocks(content string) string {
	re := regexp.MustCompile("(?s)```[^\n]*\n.*?```")
	return re.ReplaceAllString(content, "")
}

// checkAmericanEnglish validates American English spelling
func (v *StyleValidator) checkAmericanEnglish(content string) []ValidationIssue {
	var issues []ValidationIssue

	britishToAmerican := map[string]string{
		// -ise → -ize
		`\banalyse\b`: "analyze", `\bauthorise\b`: "authorize",
		`\bcustomise\b`: "customize", `\bemphasise\b`: "emphasize",
		`\bfinalise\b`: "finalize", `\binitialise\b`: "initialize",
		`\boptimise\b`: "optimize", `\borganise\b`: "organize",
		`\bprioritise\b`: "prioritize", `\brealise\b`: "realize",
		`\brecognise\b`: "recognize", `\bspecialise\b`: "specialize",
		`\bstandardise\b`: "standardize", `\bsynchronise\b`: "synchronize",
		`\butilise\b`: "utilize", `\bvisualise\b`: "visualize",
		// -isation → -ization
		`\bcustomisation\b`: "customization", `\binitialisation\b`: "initialization",
		`\boptimisation\b`: "optimization", `\borganisation\b`: "organization",
		`\bstandardisation\b`: "standardization", `\bvisualisation\b`: "visualization",
		// -our → -or
		`\bbehaviour\b`: "behavior", `\bcolour\b`: "color",
		`\bfavourite\b`: "favorite", `\bflavour\b`: "flavor",
		`\bhonour\b`: "honor", `\blabour\b`: "labor",
		`\bneighbour\b`: "neighbor",
		// -ce → -se
		`\bdefence\b`: "defense", `\blicence\b`: "license",
		`\boffence\b`: "offense", `\bpretence\b`: "pretense",
		// -ogue → -og
		`\bcatalogue\b`: "catalog", `\bdialogue\b`: "dialog",
		`\bepilogue\b`: "epilog",
		// -re → -er
		`\bcentre\b`: "center", `\bfibre\b`: "fiber",
		`\blitre\b`: "liter", `\bmetre\b`: "meter",
		// -st → plain
		`\bamongst\b`: "among", `\bwhilst\b`: "while",
		// misc
		`\bgrey\b`: "gray", `\bprogramme\b`: "program",
		`\btowards\b`: "toward", `\backnowledgement\b`: "acknowledgment",
		`\bpractise\b`:  "practice",
		`\bcancelled\b`: "canceled", `\bmodelled\b`: "modeled",
		`\btravelled\b`: "traveled",
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

// checkDontUse flags words/phrases the Elastic style guide prohibits.
func (v *StyleValidator) checkDontUse(prose string) []ValidationIssue {
	var issues []ValidationIssue

	banned := []struct {
		pattern string
		term    string
	}{
		{`(?i)\bjust\b`, "just"},
		{`(?i)\bplease\b`, "please"},
		{`(?i)\band/or\b`, "and/or"},
		{`(?i)\bnote that\b`, "note that"},
		{`(?i)\brealtime\b`, "realtime (use real time / real-time)"},
		{`(?i)\bthus\b`, "thus"},
		{`(?i)\bvery\b`, "very"},
		{`(?i)\bquite\b`, "quite"},
		{`(?i)\bat this point\b`, "at this point"},
		{`(?i)\ba\.k\.a\.\b`, "a.k.a."},
		{`(?i)\baka\b`, "aka"},
	}

	found := []string{}
	for _, b := range banned {
		re := regexp.MustCompile(b.pattern)
		if re.MatchString(prose) {
			found = append(found, b.term)
		}
	}

	if len(found) > 0 {
		issues = append(issues, ValidationIssue{
			Severity:    SeverityMinor,
			Category:    CategoryStyle,
			Location:    "Word choice",
			Message:     "Found discouraged words/phrases: " + strings.Join(found[:min(5, len(found))], ", "),
			Suggestion:  "Remove or replace per Elastic style guide",
			SourceCheck: "static",
		})
	}
	return issues
}

// checkLatinisms flags Latin abbreviations that should be replaced.
func (v *StyleValidator) checkLatinisms(prose string) []ValidationIssue {
	var issues []ValidationIssue

	latinisms := []struct {
		pattern     string
		term        string
		replacement string
	}{
		{`(?i)\be\.?g\.?\b`, "e.g.", "for example"},
		{`(?i)\bi\.?e\.?\b`, "i.e.", "that is"},
		{`(?i)\bvia\b`, "via", "using / through"},
		{`(?i)\bvs\.?\b`, "vs", "versus"},
		{`(?i)\bad[\s-]hoc\b`, "ad hoc", "if needed"},
		{`(?i)\bvice\s+versa\b`, "vice versa", "and the reverse"},
	}

	for _, l := range latinisms {
		re := regexp.MustCompile(l.pattern)
		if re.MatchString(prose) {
			issues = append(issues, ValidationIssue{
				Severity:    SeverityMinor,
				Category:    CategoryStyle,
				Location:    "Latin terms",
				Message:     "Found Latin term '" + l.term + "'",
				Suggestion:  "Use '" + l.replacement + "' instead",
				SourceCheck: "static",
			})
		}
	}
	return issues
}

// checkExclamation flags exclamation marks in prose.
func (v *StyleValidator) checkExclamation(prose string) []ValidationIssue {
	var issues []ValidationIssue
	re := regexp.MustCompile(`\w+!(?:\s|$)`)
	if re.MatchString(prose) {
		issues = append(issues, ValidationIssue{
			Severity:    SeverityMinor,
			Category:    CategoryStyle,
			Location:    "Punctuation",
			Message:     "Found exclamation point in body text",
			Suggestion:  "Use exclamation points sparingly - consider removing",
			SourceCheck: "static",
		})
	}
	return issues
}

// checkEllipses flags ellipses in prose.
func (v *StyleValidator) checkEllipses(prose string) []ValidationIssue {
	var issues []ValidationIssue
	re := regexp.MustCompile(`\.\.\.`)
	if re.MatchString(prose) {
		issues = append(issues, ValidationIssue{
			Severity:    SeverityMinor,
			Category:    CategoryStyle,
			Location:    "Punctuation",
			Message:     "Found ellipsis (...) in text",
			Suggestion:  "Remove the ellipsis - rewrite the sentence instead",
			SourceCheck: "static",
		})
	}
	return issues
}

// checkVersionTerms flags incorrect version comparison wording.
func (v *StyleValidator) checkVersionTerms(prose string) []ValidationIssue {
	var issues []ValidationIssue

	versionSwaps := []struct {
		pattern     string
		replacement string
	}{
		{`(?i)\band\s+higher\b`, "and later"},
		{`(?i)\bor\s+higher\b`, "or later"},
		{`(?i)\band\s+newer\b`, "and later"},
		{`(?i)\bor\s+newer\b`, "or later"},
		{`(?i)\band\s+lower\b`, "and earlier"},
		{`(?i)\bor\s+lower\b`, "or earlier"},
		{`(?i)\band\s+older\b`, "and earlier"},
		{`(?i)\bor\s+older\b`, "or earlier"},
		{`(?i)\bnewer\s+versions?\b`, "later version(s)"},
		{`(?i)\bhigher\s+versions?\b`, "later version(s)"},
		{`(?i)\bolder\s+versions?\b`, "earlier version(s)"},
		{`(?i)\blower\s+versions?\b`, "earlier version(s)"},
	}

	found := []string{}
	for _, vs := range versionSwaps {
		re := regexp.MustCompile(vs.pattern)
		if re.MatchString(prose) {
			found = append(found, vs.replacement)
		}
	}

	if len(found) > 0 {
		issues = append(issues, ValidationIssue{
			Severity:    SeverityMinor,
			Category:    CategoryStyle,
			Location:    "Version terminology",
			Message:     "Use 'later'/'earlier' for version comparisons",
			Suggestion:  "Replace with: " + strings.Join(found[:min(3, len(found))], ", "),
			SourceCheck: "static",
		})
	}
	return issues
}

// checkArticles flags incorrect articles before acronyms (a/an).
func (v *StyleValidator) checkArticles(prose string) []ValidationIssue {
	var issues []ValidationIssue

	// Acronyms that start with a vowel sound → require "an"
	needAn := []string{"FAQ", "HTML", "HTTP", "HTTPS", "SQL", "SSH", "SSL", "SDK", "XML", "API", "IDE", "RSS", "SSD", "SVG", "XSS"}
	// Acronyms that start with a consonant sound → require "a"
	needA := []string{"GUI", "PDF", "USB", "URL", "URI", "CPU", "GPU", "RAM", "CSV", "JSON"}

	for _, acr := range needAn {
		re := regexp.MustCompile(`(?i)\ba\s+` + acr + `\b`)
		if re.MatchString(prose) {
			issues = append(issues, ValidationIssue{
				Severity:    SeverityMinor,
				Category:    CategoryStyle,
				Location:    "Articles",
				Message:     "Use 'an " + acr + "' instead of 'a " + acr + "'",
				Suggestion:  "The article depends on pronunciation, not spelling",
				SourceCheck: "static",
			})
		}
	}

	for _, acr := range needA {
		re := regexp.MustCompile(`(?i)\ban\s+` + acr + `\b`)
		if re.MatchString(prose) {
			issues = append(issues, ValidationIssue{
				Severity:    SeverityMinor,
				Category:    CategoryStyle,
				Location:    "Articles",
				Message:     "Use 'a " + acr + "' instead of 'an " + acr + "'",
				Suggestion:  "The article depends on pronunciation, not spelling",
				SourceCheck: "static",
			})
		}
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
