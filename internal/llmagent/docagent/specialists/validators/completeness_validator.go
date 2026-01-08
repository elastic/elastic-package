// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package validators

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

const (
	completenessValidatorName        = "completeness_validator"
	completenessValidatorDescription = "Validates that all required content is present in the documentation"
)

const completenessValidatorInstruction = `You are a documentation completeness validator for Elastic integration packages.
Your task is to validate that all required content is present and comprehensive.

## Input
The documentation content to validate is provided in the user message.
You may also receive static validation context including data stream names.

## Checks
1. All data streams from the package are documented
2. Setup instructions cover both vendor-side and Kibana-side configuration
3. Validation steps are provided to verify the integration works
4. Troubleshooting section addresses common issues
5. Reference section includes field documentation
6. First line should indicate LLM-generated documentation (if applicable)

## Output Format
Output a JSON object with this exact structure:
{"valid": true/false, "score": 0-100, "issues": [{"severity": "critical|major|minor", "category": "completeness", "location": "Section Name", "message": "Issue description", "suggestion": "How to fix"}]}

Set valid=false if critical completeness issues are found (missing data streams, no setup instructions).
Minor gaps like missing troubleshooting can be warnings.

## IMPORTANT
Output ONLY the JSON object. No other text.`

// CompletenessValidator validates documentation completeness (Section C)
type CompletenessValidator struct {
	BaseStagedValidator
}

// NewCompletenessValidator creates a new completeness validator
func NewCompletenessValidator() *CompletenessValidator {
	return &CompletenessValidator{
		BaseStagedValidator: BaseStagedValidator{
			name:        completenessValidatorName,
			description: completenessValidatorDescription,
			stage:       StageCompleteness,
			instruction: completenessValidatorInstruction,
		},
	}
}

// SupportsStaticValidation returns true - this validator has static checks
func (v *CompletenessValidator) SupportsStaticValidation() bool {
	return true
}

// StaticValidate performs static completeness validation
func (v *CompletenessValidator) StaticValidate(ctx context.Context, content string, pkgCtx *PackageContext) (*StagedValidationResult, error) {
	result := &StagedValidationResult{
		Stage: StageCompleteness,
		Valid: true,
	}

	// Check 1: All data streams documented
	if pkgCtx != nil {
		result.Issues = append(result.Issues, v.checkDataStreamsCovered(content, pkgCtx)...)
	}

	// Check 2: Setup section has both vendor and Kibana steps
	result.Issues = append(result.Issues, v.checkSetupCompleteness(content)...)

	// Check 3: Validation/verification steps present
	result.Issues = append(result.Issues, v.checkValidationSteps(content)...)

	// Check 4: LLM-generated marker (if applicable)
	result.Issues = append(result.Issues, v.checkLLMMarker(content)...)

	// Check 5: Reference/fields documentation
	result.Issues = append(result.Issues, v.checkReferenceSection(content, pkgCtx)...)

	// Determine validity based on issues
	for _, issue := range result.Issues {
		if issue.Severity == SeverityCritical || issue.Severity == SeverityMajor {
			result.Valid = false
			break
		}
	}

	return result, nil
}

// checkDataStreamsCovered verifies all data streams are documented
func (v *CompletenessValidator) checkDataStreamsCovered(content string, pkgCtx *PackageContext) []ValidationIssue {
	var issues []ValidationIssue

	if pkgCtx == nil || len(pkgCtx.DataStreams) == 0 {
		return issues
	}

	contentLower := strings.ToLower(content)

	for _, ds := range pkgCtx.DataStreams {
		// Check if data stream name or title is mentioned
		nameMentioned := strings.Contains(contentLower, strings.ToLower(ds.Name))
		titleMentioned := ds.Title != "" && strings.Contains(contentLower, strings.ToLower(ds.Title))

		if !nameMentioned && !titleMentioned {
			issues = append(issues, ValidationIssue{
				Severity:    SeverityCritical,
				Category:    CategoryCompleteness,
				Location:    "Data streams",
				Message:     fmt.Sprintf("Data stream '%s' (%s) not documented", ds.Name, ds.Type),
				Suggestion:  fmt.Sprintf("Add documentation for the '%s' data stream in the Data streams section", ds.Name),
				SourceCheck: "static",
			})
		}
	}

	return issues
}

// checkSetupCompleteness verifies setup section has required parts
func (v *CompletenessValidator) checkSetupCompleteness(content string) []ValidationIssue {
	var issues []ValidationIssue

	contentLower := strings.ToLower(content)

	// Check for setup section (using aliases from structure_validator)
	// The canonical name is "How do I deploy this integration?"
	// Aliases: setup, installation, getting started, configuration
	setupSectionPatterns := []string{
		"## how do i deploy this integration",
		"## setup",
		"## installation",
		"## getting started",
		"## configuration",
		"## deployment",
	}

	hasSetupSection := false
	for _, pattern := range setupSectionPatterns {
		if strings.Contains(contentLower, pattern) {
			hasSetupSection = true
			break
		}
	}

	if !hasSetupSection {
		issues = append(issues, ValidationIssue{
			Severity:    SeverityCritical,
			Category:    CategoryCompleteness,
			Location:    "Setup",
			Message:     "Missing deployment/setup section",
			Suggestion:  "Add a '## How do I deploy this integration?' section with installation and configuration instructions",
			SourceCheck: "static",
		})
		return issues
	}

	// Check for vendor-side setup indicators
	vendorIndicators := []string{
		"configure", "enable logging", "syslog", "api key",
		"credentials", "prerequisite", "vendor", "service",
	}
	hasVendorSetup := false
	for _, indicator := range vendorIndicators {
		if strings.Contains(contentLower, indicator) {
			hasVendorSetup = true
			break
		}
	}

	if !hasVendorSetup {
		issues = append(issues, ValidationIssue{
			Severity:    SeverityMajor,
			Category:    CategoryCompleteness,
			Location:    "Setup",
			Message:     "Setup section may be missing vendor-side configuration steps",
			Suggestion:  "Add instructions for configuring the external service (credentials, logging, API setup)",
			SourceCheck: "static",
		})
	}

	// Check for Kibana/Elastic setup indicators
	kibanaIndicators := []string{
		"kibana", "elastic agent", "fleet", "add integration",
		"enroll", "policy", "index pattern",
	}
	hasKibanaSetup := false
	for _, indicator := range kibanaIndicators {
		if strings.Contains(contentLower, indicator) {
			hasKibanaSetup = true
			break
		}
	}

	if !hasKibanaSetup {
		issues = append(issues, ValidationIssue{
			Severity:    SeverityMajor,
			Category:    CategoryCompleteness,
			Location:    "Setup",
			Message:     "Setup section may be missing Kibana/Fleet configuration steps",
			Suggestion:  "Add instructions for adding the integration in Kibana/Fleet",
			SourceCheck: "static",
		})
	}

	return issues
}

// checkValidationSteps verifies validation/testing instructions exist
func (v *CompletenessValidator) checkValidationSteps(content string) []ValidationIssue {
	var issues []ValidationIssue

	contentLower := strings.ToLower(content)

	// Look for validation-related content
	validationIndicators := []string{
		"verify", "validate", "confirm", "check", "test",
		"discover", "data appears", "logs are", "metrics are",
	}

	hasValidation := false
	for _, indicator := range validationIndicators {
		if strings.Contains(contentLower, indicator) {
			hasValidation = true
			break
		}
	}

	if !hasValidation {
		issues = append(issues, ValidationIssue{
			Severity:    SeverityMajor,
			Category:    CategoryCompleteness,
			Location:    "Validation",
			Message:     "No validation steps found to verify the integration works",
			Suggestion:  "Add steps for users to verify data is being collected (e.g., check Discover, dashboards)",
			SourceCheck: "static",
		})
	}

	return issues
}

// checkLLMMarker verifies the LLM-generated marker is present
func (v *CompletenessValidator) checkLLMMarker(content string) []ValidationIssue {
	var issues []ValidationIssue

	// Check first few lines for LLM marker
	lines := strings.Split(content, "\n")
	firstFewLines := ""
	for i, line := range lines {
		if i >= 5 {
			break
		}
		firstFewLines += strings.ToLower(line) + " "
	}

	llmMarkers := []string{
		"llm-generated",
		"ai-generated",
		"auto-generated",
		"automatically generated",
		"generated by",
	}

	hasMarker := false
	for _, marker := range llmMarkers {
		if strings.Contains(firstFewLines, marker) {
			hasMarker = true
			break
		}
	}

	if !hasMarker {
		issues = append(issues, ValidationIssue{
			Severity:    SeverityMinor,
			Category:    CategoryCompleteness,
			Location:    "Header",
			Message:     "Missing LLM-generated documentation marker",
			Suggestion:  "Add a note at the beginning indicating this is LLM-generated documentation",
			SourceCheck: "static",
		})
	}

	return issues
}

// checkReferenceSection verifies reference/fields documentation exists
func (v *CompletenessValidator) checkReferenceSection(content string, pkgCtx *PackageContext) []ValidationIssue {
	var issues []ValidationIssue

	contentLower := strings.ToLower(content)

	// Check for reference section
	hasReference := strings.Contains(contentLower, "## reference") ||
		strings.Contains(contentLower, "## fields") ||
		strings.Contains(contentLower, "## exported fields") ||
		strings.Contains(contentLower, "## field reference")

	// Check for field documentation indicators
	hasFieldDocs := regexp.MustCompile(`\|\s*field\s*\|`).MatchString(contentLower) ||
		strings.Contains(contentLower, "| name | type |") ||
		strings.Contains(contentLower, "exported fields")

	if !hasReference && !hasFieldDocs {
		// Only warn if package has fields
		if pkgCtx != nil && len(pkgCtx.Fields) > 0 {
			issues = append(issues, ValidationIssue{
				Severity:    SeverityMinor,
				Category:    CategoryCompleteness,
				Location:    "Reference",
				Message:     "Missing field reference documentation",
				Suggestion:  "Consider adding a Reference section with field documentation",
				SourceCheck: "static",
			})
		}
	}

	// Check for sample events
	hasSampleEvents := strings.Contains(contentLower, "sample event") ||
		strings.Contains(contentLower, "example event") ||
		strings.Contains(contentLower, "```json") // JSON code blocks often contain sample events

	if !hasSampleEvents && pkgCtx != nil && len(pkgCtx.DataStreams) > 0 {
		issues = append(issues, ValidationIssue{
			Severity:    SeverityMinor,
			Category:    CategoryCompleteness,
			Location:    "Reference",
			Message:     "No sample events found in documentation",
			Suggestion:  "Consider adding example events for each data stream",
			SourceCheck: "static",
		})
	}

	return issues
}

