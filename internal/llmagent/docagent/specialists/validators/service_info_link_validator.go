// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package validators

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

const (
	serviceInfoLinkValidatorName        = "service_info_link_validator"
	serviceInfoLinkValidatorDescription = "Validates that links from service_info.md are included in generated documentation"
)

// ServiceInfoLinkValidator validates that links from service_info.md appear in the generated documentation.
// This ensures important vendor documentation links, configuration references, and other
// authoritative sources from the knowledge base are preserved in the final output.
type ServiceInfoLinkValidator struct {
	BaseStagedValidator
}

// NewServiceInfoLinkValidator creates a new service info link validator.
func NewServiceInfoLinkValidator() *ServiceInfoLinkValidator {
	return &ServiceInfoLinkValidator{
		BaseStagedValidator: BaseStagedValidator{
			name:        serviceInfoLinkValidatorName,
			description: serviceInfoLinkValidatorDescription,
			stage:       StageCompleteness, // Part of completeness checking
			scope:       ScopeBoth,         // Link validation works on sections and full document
			instruction: serviceInfoLinkValidatorInstruction,
		},
	}
}

// serviceInfoLinkValidatorInstruction is the system instruction for LLM validation
const serviceInfoLinkValidatorInstruction = `You are a documentation link validator for Elastic integration packages.
Your task is to verify that important reference links from the source material appear in the generated documentation.

## Context
The service_info.md file contains authoritative information about an integration, including:
- Vendor documentation links
- Configuration reference URLs
- Community resources
- API documentation

These links are valuable for users and should be preserved in the generated README.

## Output Format
Output a JSON object with:
{
  "valid": true/false,
  "score": 0-100,
  "issues": [
    {
      "severity": "critical|major|minor",
      "category": "completeness",
      "location": "string",
      "message": "string", 
      "suggestion": "string"
    }
  ],
  "summary": "brief summary"
}

## Severity Guidelines
- critical: Key vendor documentation links are missing (e.g., main product docs, setup guides)
- major: Important reference links are missing (e.g., configuration references, troubleshooting)
- minor: Nice-to-have links are missing (e.g., community forums, blog posts)
`

// SupportsStaticValidation returns true - this validator has static checks
func (v *ServiceInfoLinkValidator) SupportsStaticValidation() bool {
	return true
}

// StaticValidate checks if links from service_info.md appear in the generated content
func (v *ServiceInfoLinkValidator) StaticValidate(ctx context.Context, content string, pkgCtx *PackageContext) (*StagedValidationResult, error) {
	result := &StagedValidationResult{
		Stage: StageCompleteness,
		Valid: true,
		Score: 100,
	}

	// Skip if no package context or no service info links
	if pkgCtx == nil || !pkgCtx.HasServiceInfoLinks() {
		return result, nil
	}

	var issues []ValidationIssue
	contentLower := strings.ToLower(content)

	for _, link := range pkgCtx.ServiceInfoLinks {
		// Check if the URL appears in the generated content using flexible matching
		// LLMs sometimes modify URLs slightly, so we use multiple strategies
		if !isLinkPresent(contentLower, link) {
			severity := classifyLinkSeverity(link)
			issues = append(issues, ValidationIssue{
				Severity:    severity,
				Category:    CategoryCompleteness,
				Location:    "documentation",
				Message:     fmt.Sprintf("Missing link from service_info.md: [%s](%s)", link.Text, link.URL),
				Suggestion:  fmt.Sprintf("Add the link: [%s](%s)", link.Text, link.URL),
				SourceCheck: "static",
			})
		}
	}

	// Calculate score
	totalLinks := len(pkgCtx.ServiceInfoLinks)
	missingLinks := len(issues)
	if totalLinks > 0 {
		result.Score = ((totalLinks - missingLinks) * 100) / totalLinks
	}

	// Set validity based on critical/major issues
	for _, issue := range issues {
		if issue.Severity == SeverityCritical || issue.Severity == SeverityMajor {
			result.Valid = false
			break
		}
	}

	result.Issues = issues

	// Add actionable suggestions for the generator
	if len(issues) > 0 {
		result.Suggestions = []string{
			"Ensure all vendor documentation links from service_info.md are included in the generated documentation",
		}

		// Group missing links by severity for clearer feedback
		var criticalLinks, majorLinks, minorLinks []string
		for _, issue := range issues {
			linkStr := issue.Suggestion
			switch issue.Severity {
			case SeverityCritical:
				criticalLinks = append(criticalLinks, linkStr)
			case SeverityMajor:
				majorLinks = append(majorLinks, linkStr)
			case SeverityMinor:
				minorLinks = append(minorLinks, linkStr)
			}
		}

		if len(criticalLinks) > 0 {
			result.Suggestions = append(result.Suggestions,
				fmt.Sprintf("CRITICAL: Add these essential links: %s", strings.Join(criticalLinks, "; ")))
		}
		if len(majorLinks) > 0 {
			result.Suggestions = append(result.Suggestions,
				fmt.Sprintf("IMPORTANT: Add these reference links: %s", strings.Join(majorLinks, "; ")))
		}
	}

	return result, nil
}

// isLinkPresent checks if a link is present in the content using flexible matching
// This handles cases where the LLM slightly modifies URLs or uses different link text
func isLinkPresent(contentLower string, link ServiceInfoLink) bool {
	urlLower := strings.ToLower(link.URL)

	// Strategy 1: Exact URL match
	if strings.Contains(contentLower, urlLower) {
		return true
	}

	// Strategy 2: Extract and match article IDs (e.g., CTX138973, KB12345)
	articleIDPattern := regexp.MustCompile(`(?i)(ctx\d+|kb\d+|doc-\d+|article[/-]\d+)`)
	if matches := articleIDPattern.FindStringSubmatch(urlLower); len(matches) > 0 {
		articleID := strings.ToLower(matches[1])
		if strings.Contains(contentLower, articleID) {
			return true
		}
	}

	// Strategy 3: Match domain + path components (more lenient)
	if urlObj, err := url.Parse(link.URL); err == nil {
		domain := strings.ToLower(urlObj.Host)
		
		// Check if domain is present
		if strings.Contains(contentLower, domain) {
			// Extract meaningful path parts (skip common ones)
			pathParts := strings.Split(urlObj.Path, "/")
			skipParts := map[string]bool{
				"en": true, "us": true, "current": true, "release": true,
				"latest": true, "12.0": true, "projects": true, "s": true,
				"article": true, "docs": true, "v1": true, "api": true,
			}
			
			meaningfulParts := 0
			matchedParts := 0
			for _, part := range pathParts {
				part = strings.ToLower(part)
				if len(part) > 3 && !skipParts[part] {
					meaningfulParts++
					// Normalize part for comparison (replace - with space)
					normalizedPart := strings.ReplaceAll(part, "-", " ")
					if strings.Contains(contentLower, part) || strings.Contains(contentLower, normalizedPart) {
						matchedParts++
					}
				}
			}
			
			// If at least one meaningful path part matches, consider it found
			if matchedParts >= 1 {
				return true
			}
		}
	}

	// Strategy 4: If link text is meaningful, check variations
	if len(link.Text) > 10 && strings.ToLower(link.Text) != "link" {
		textLower := strings.ToLower(link.Text)
		
		// Direct match
		if strings.Contains(contentLower, textLower) {
			return true
		}
		
		// Check for key words from the link text (ignore common filler words)
		fillerWords := map[string]bool{
			"the": true, "a": true, "an": true, "to": true, "and": true,
			"of": true, "in": true, "for": true, "en": true, "us": true,
			"current": true, "release": true, "docs": true, "how": true,
		}
		
		words := strings.Fields(textLower)
		meaningfulWords := 0
		matchedWords := 0
		for _, word := range words {
			word = strings.Trim(word, ".,;:!?")
			if len(word) > 3 && !fillerWords[word] {
				meaningfulWords++
				if strings.Contains(contentLower, word) {
					matchedWords++
				}
			}
		}
		
		// If most meaningful words match, consider it found
		if meaningfulWords > 0 && matchedWords >= (meaningfulWords+1)/2 {
			return true
		}
	}

	// Strategy 5: Extract key identifier from URL path and check content
	// For URLs like ".../netscaler-syslog-message-reference/..." check for "syslog message reference"
	if urlObj, err := url.Parse(link.URL); err == nil {
		pathParts := strings.Split(urlObj.Path, "/")
		for _, part := range pathParts {
			if len(part) > 15 { // Significant path component
				normalized := strings.ReplaceAll(strings.ToLower(part), "-", " ")
				if strings.Contains(contentLower, normalized) {
					return true
				}
				// Also check if the normalized version with common words removed is present
				words := strings.Fields(normalized)
				if len(words) >= 2 {
					keyPhrase := strings.Join(words[:min(3, len(words))], " ")
					if strings.Contains(contentLower, keyPhrase) {
						return true
					}
				}
			}
		}
	}

	return false
}

// classifyLinkSeverity determines the severity based on link characteristics
func classifyLinkSeverity(link ServiceInfoLink) ValidationSeverity {
	textLower := strings.ToLower(link.Text)
	urlLower := strings.ToLower(link.URL)

	// Critical: Main product documentation, official guides
	criticalKeywords := []string{
		"documentation", "guide", "official", "reference",
		"admin", "administration", "getting started",
	}
	for _, kw := range criticalKeywords {
		if strings.Contains(textLower, kw) || strings.Contains(urlLower, "/docs/") {
			return SeverityCritical
		}
	}

	// Major: Configuration, setup, troubleshooting
	majorKeywords := []string{
		"config", "setup", "install", "troubleshoot",
		"cli", "api", "ssl", "authentication",
	}
	for _, kw := range majorKeywords {
		if strings.Contains(textLower, kw) || strings.Contains(urlLower, kw) {
			return SeverityMajor
		}
	}

	// Minor: Community, blog, forum links
	minorKeywords := []string{
		"community", "forum", "blog", "tip", "technical tip",
	}
	for _, kw := range minorKeywords {
		if strings.Contains(textLower, kw) || strings.Contains(urlLower, kw) {
			return SeverityMinor
		}
	}

	// Default to major for unclassified links
	return SeverityMajor
}

