// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package validators

import (
	"context"
	"fmt"
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
		// Check if the URL appears in the generated content
		// We check the URL itself, not the link text, since text might be rephrased
		urlLower := strings.ToLower(link.URL)
		if !strings.Contains(contentLower, urlLower) {
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

