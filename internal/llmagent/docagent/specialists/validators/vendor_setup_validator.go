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
	vendorSetupValidatorName        = "vendor_setup_validator"
	vendorSetupValidatorDescription = "Validates vendor setup instructions against service_info.md links and known procedures"
)

const vendorSetupValidatorInstruction = `You are a vendor setup validation expert for Elastic integration packages.
Your task is to validate that the setup instructions in the documentation are accurate and match vendor documentation.

## Input
You will receive:
1. The documentation content to validate
2. Vendor setup links extracted from service_info.md
3. The vendor/product name

## Your Validation Tasks

### Task 1: Validate Against Vendor Documentation Links
For each vendor setup link provided:
- Check if the setup instructions in the README align with what those links would describe
- Flag if critical setup steps from vendor docs appear to be missing
- Flag if the order of steps seems incorrect based on typical vendor workflows

### Task 2: Validate Against Your Knowledge
Using your knowledge of the vendor/product:
- Check if the setup instructions are technically accurate
- Flag any steps that are known to be incorrect or outdated
- Flag any missing prerequisite steps
- Flag incorrect configuration values, ports, paths, or commands
- Flag deprecated features or settings being used

### Task 3: Identify Specific Conflicts
For each issue found, provide:
- The exact section/location in the document
- What the document says (quote it)
- What it SHOULD say (your correction)
- Why this is incorrect (brief explanation)

## Output Format
Output a JSON object with this exact structure:
{
  "valid": true/false,
  "score": 0-100,
  "issues": [
    {
      "severity": "critical|major|minor",
      "category": "vendor_setup",
      "location": "Section Name or line reference",
      "message": "What is wrong",
      "suggestion": "Specific correction with exact text to use",
      "source_check": "llm",
      "conflict_type": "incorrect_step|missing_step|wrong_order|outdated|incorrect_value"
    }
  ],
  "vendor_links_checked": ["url1", "url2"],
  "summary": "Brief summary of validation"
}

## Severity Guidelines
- critical: Setup will fail if user follows these instructions (wrong commands, incorrect configs)
- major: Setup might work but with issues (missing important steps, wrong order)
- minor: Suboptimal setup (missing best practices, incomplete explanations)

## IMPORTANT
- Be specific about what is wrong and how to fix it
- Quote the problematic text from the document
- Provide the corrected text that should replace it
- Only flag issues you are confident about
- Output ONLY the JSON object. No other text.`

// VendorSetupValidator validates setup instructions against vendor documentation and LLM knowledge
type VendorSetupValidator struct {
	BaseStagedValidator
}

// NewVendorSetupValidator creates a new vendor setup validator
func NewVendorSetupValidator() *VendorSetupValidator {
	return &VendorSetupValidator{
		BaseStagedValidator: BaseStagedValidator{
			name:        vendorSetupValidatorName,
			description: vendorSetupValidatorDescription,
			stage:       StageAccuracy, // Part of accuracy checking
			instruction: vendorSetupValidatorInstruction,
		},
	}
}

// SupportsStaticValidation returns true - this validator has static checks
func (v *VendorSetupValidator) SupportsStaticValidation() bool {
	return true
}

// StaticValidate performs static vendor setup validation
// This validates vendor setup documentation ONLY when service_info.md contains vendor setup content.
// If there's no service_info.md or it lacks vendor setup sections, this validator passes automatically.
func (v *VendorSetupValidator) StaticValidate(ctx context.Context, content string, pkgCtx *PackageContext) (*StagedValidationResult, error) {
	result := &StagedValidationResult{
		Stage: StageAccuracy,
		Valid: true,
		Score: 100,
	}

	if pkgCtx == nil {
		return result, nil
	}

	// CRITICAL: Only require vendor setup validation if service_info.md has vendor setup content
	// If no vendor setup content exists in service_info.md, skip this validation
	if !pkgCtx.HasVendorSetupContent() {
		result.Suggestions = []string{
			"No vendor setup content detected in service_info.md - vendor setup validation skipped",
		}
		return result, nil
	}

	// service_info.md HAS vendor setup content - now we MUST validate the generated doc includes it

	// Extract setup section from content
	setupSection := v.extractSetupSection(content)
	if setupSection == "" {
		// CRITICAL: service_info.md has vendor setup, but generated doc has no setup section
		result.Issues = append(result.Issues, ValidationIssue{
			Severity:    SeverityCritical,
			Category:    CategoryVendorSetup,
			Location:    "Document",
			Message:     "Missing setup/deployment section - service_info.md contains vendor setup instructions that MUST be included",
			Suggestion:  "Add a comprehensive setup section incorporating vendor setup instructions from service_info.md",
			SourceCheck: "static",
		})
		result.Valid = false

		// Provide the vendor setup content for the generator
		result.Suggestions = []string{pkgCtx.GetVendorSetupForGenerator()}
		return result, nil
	}

	// Validate that the setup section adequately covers the vendor setup from service_info.md
	result.Issues = append(result.Issues, v.validateAgainstServiceInfo(setupSection, pkgCtx)...)

	// Extract vendor setup links from service_info
	vendorLinks := v.extractVendorSetupLinks(pkgCtx)

	// Check if setup section references any vendor links
	if len(vendorLinks) > 0 {
		result.Issues = append(result.Issues, v.checkVendorLinksReferenced(setupSection, vendorLinks)...)
	}

	// Check for common setup instruction issues
	result.Issues = append(result.Issues, v.checkCommonSetupIssues(setupSection, pkgCtx)...)

	// Determine validity based on issues
	for _, issue := range result.Issues {
		if issue.Severity == SeverityCritical || issue.Severity == SeverityMajor {
			result.Valid = false
			break
		}
	}

	// Store context for LLM validation - include the full vendor setup content
	if pkgCtx.HasVendorSetupContent() {
		result.Suggestions = []string{pkgCtx.GetVendorSetupForGenerator()}
	}

	return result, nil
}

// validateAgainstServiceInfo checks if the generated setup section covers the content from service_info.md
func (v *VendorSetupValidator) validateAgainstServiceInfo(setupSection string, pkgCtx *PackageContext) []ValidationIssue {
	var issues []ValidationIssue

	if pkgCtx.VendorSetup == nil {
		return issues
	}

	contentLower := strings.ToLower(setupSection)

	// Check if vendor prerequisites are mentioned
	if pkgCtx.VendorSetup.HasVendorPrerequisites {
		prereqKeywords := []string{"prerequisite", "requirement", "before", "need"}
		hasPrereqs := false
		for _, kw := range prereqKeywords {
			if strings.Contains(contentLower, kw) {
				hasPrereqs = true
				break
			}
		}
		if !hasPrereqs {
			issues = append(issues, ValidationIssue{
				Severity:    SeverityMajor,
				Category:    CategoryVendorSetup,
				Location:    "Setup section",
				Message:     "Missing vendor prerequisites - service_info.md contains prerequisite information",
				Suggestion:  "Include vendor prerequisites from service_info.md: credentials, network access, permissions",
				SourceCheck: "static",
			})
		}
	}

	// Check if vendor setup steps are adequately covered
	if pkgCtx.VendorSetup.HasVendorSetupSteps {
		// Check for key indicators of vendor-side setup
		vendorSetupIndicators := []string{
			"navigate", "gui", "cli", "console", "admin", "settings", "configure on",
			"in the", "on the", "click", "select", "enable", "log in",
		}
		vendorIndicatorsFound := 0
		for _, indicator := range vendorSetupIndicators {
			if strings.Contains(contentLower, indicator) {
				vendorIndicatorsFound++
			}
		}

		if vendorIndicatorsFound < 3 {
			issues = append(issues, ValidationIssue{
				Severity:    SeverityCritical,
				Category:    CategoryVendorSetup,
				Location:    "Setup section",
				Message:     "Insufficient vendor-side setup instructions - service_info.md contains detailed vendor setup steps",
				Suggestion:  "Include comprehensive vendor-side configuration instructions with GUI or CLI steps from service_info.md",
				SourceCheck: "static",
			})
		}
	}

	// Check if Kibana setup steps are mentioned
	if pkgCtx.VendorSetup.HasKibanaSetupSteps {
		kibanaKeywords := []string{"kibana", "fleet", "integration", "add", "management"}
		kibanaFound := 0
		for _, kw := range kibanaKeywords {
			if strings.Contains(contentLower, kw) {
				kibanaFound++
			}
		}
		if kibanaFound < 2 {
			issues = append(issues, ValidationIssue{
				Severity:    SeverityMajor,
				Category:    CategoryVendorSetup,
				Location:    "Setup section",
				Message:     "Missing Kibana/Fleet setup instructions - service_info.md contains Kibana setup steps",
				Suggestion:  "Include Kibana/Fleet setup instructions from service_info.md",
				SourceCheck: "static",
			})
		}
	}

	// Check if validation steps are included
	if pkgCtx.VendorSetup.HasValidationSteps {
		validationKeywords := []string{"verify", "validate", "confirm", "check", "test"}
		hasValidation := false
		for _, kw := range validationKeywords {
			if strings.Contains(contentLower, kw) {
				hasValidation = true
				break
			}
		}
		if !hasValidation {
			issues = append(issues, ValidationIssue{
				Severity:    SeverityMajor,
				Category:    CategoryVendorSetup,
				Location:    "Setup section",
				Message:     "Missing validation steps - service_info.md contains validation instructions",
				Suggestion:  "Include validation steps from service_info.md to verify the setup is working",
				SourceCheck: "static",
			})
		}
	}

	// Check if vendor documentation links are included
	if len(pkgCtx.VendorSetup.VendorLinks) > 0 {
		missingLinks := 0
		for _, link := range pkgCtx.VendorSetup.VendorLinks {
			if !strings.Contains(setupSection, link.URL) {
				missingLinks++
			}
		}

		if missingLinks > 0 && missingLinks == len(pkgCtx.VendorSetup.VendorLinks) {
			issues = append(issues, ValidationIssue{
				Severity:    SeverityMajor,
				Category:    CategoryVendorSetup,
				Location:    "Setup section",
				Message:     fmt.Sprintf("Missing vendor documentation links - service_info.md contains %d vendor links", len(pkgCtx.VendorSetup.VendorLinks)),
				Suggestion:  "Include vendor documentation links from service_info.md",
				SourceCheck: "static",
			})
		}
	}

	return issues
}

// extractSetupSection extracts the setup/deployment section from content
func (v *VendorSetupValidator) extractSetupSection(content string) string {
	// Find the start of setup-related sections
	startPatterns := []string{
		`(?i)##\s*How do I deploy this integration`,
		`(?i)##\s*Setup`,
		`(?i)##\s*Installation`,
		`(?i)##\s*Getting Started`,
		`(?i)##\s*Configuration`,
	}

	var startIdx int = -1
	for _, pattern := range startPatterns {
		re := regexp.MustCompile(pattern)
		loc := re.FindStringIndex(content)
		if loc != nil {
			startIdx = loc[0]
			break
		}
	}

	if startIdx == -1 {
		return ""
	}

	// Find the next H2 section after the start
	remainingContent := content[startIdx:]
	// Skip past the first ## to find the next one
	nextH2Pattern := regexp.MustCompile(`(?m)^##\s+[^#]`)
	// Find all H2 headers
	matches := nextH2Pattern.FindAllStringIndex(remainingContent, -1)

	if len(matches) > 1 {
		// Return from start to the second H2 (which is the next section)
		return remainingContent[:matches[1][0]]
	}

	// No next section found, return rest of document
	return remainingContent
}

// extractVendorSetupLinks extracts setup-related links from service_info
func (v *VendorSetupValidator) extractVendorSetupLinks(pkgCtx *PackageContext) []ServiceInfoLink {
	if pkgCtx == nil || !pkgCtx.HasServiceInfoLinks() {
		return nil
	}

	var setupLinks []ServiceInfoLink

	setupKeywords := []string{
		"setup", "install", "config", "configure", "getting started",
		"quick start", "deployment", "admin", "guide", "tutorial",
		"how to", "enable", "logging", "syslog", "api", "credential",
	}

	for _, link := range pkgCtx.ServiceInfoLinks {
		textLower := strings.ToLower(link.Text)
		urlLower := strings.ToLower(link.URL)

		for _, keyword := range setupKeywords {
			if strings.Contains(textLower, keyword) || strings.Contains(urlLower, keyword) {
				setupLinks = append(setupLinks, link)
				break
			}
		}
	}

	return setupLinks
}

// checkVendorLinksReferenced verifies setup section references vendor documentation
func (v *VendorSetupValidator) checkVendorLinksReferenced(setupSection string, vendorLinks []ServiceInfoLink) []ValidationIssue {
	var issues []ValidationIssue

	contentLower := strings.ToLower(setupSection)
	missingCriticalLinks := []string{}

	for _, link := range vendorLinks {
		urlLower := strings.ToLower(link.URL)

		// Check if link is referenced
		if !strings.Contains(contentLower, urlLower) {
			// Classify the importance of this link
			if v.isCriticalSetupLink(link) {
				missingCriticalLinks = append(missingCriticalLinks,
					fmt.Sprintf("[%s](%s)", link.Text, link.URL))
			}
		}
	}

	if len(missingCriticalLinks) > 0 {
		issues = append(issues, ValidationIssue{
			Severity:    SeverityMajor,
			Category:    CategoryVendorSetup,
			Location:    "Setup section",
			Message:     fmt.Sprintf("Setup section missing %d critical vendor documentation links", len(missingCriticalLinks)),
			Suggestion:  fmt.Sprintf("Add references to: %s", strings.Join(missingCriticalLinks, ", ")),
			SourceCheck: "static",
		})
	}

	return issues
}

// isCriticalSetupLink determines if a link is critical for setup
func (v *VendorSetupValidator) isCriticalSetupLink(link ServiceInfoLink) bool {
	criticalKeywords := []string{
		"official", "documentation", "admin guide", "setup guide",
		"getting started", "installation", "configure logging",
		"enable api", "authentication",
	}

	textLower := strings.ToLower(link.Text)
	for _, keyword := range criticalKeywords {
		if strings.Contains(textLower, keyword) {
			return true
		}
	}

	return false
}

// checkCommonSetupIssues checks for common problems in setup instructions
func (v *VendorSetupValidator) checkCommonSetupIssues(setupSection string, pkgCtx *PackageContext) []ValidationIssue {
	var issues []ValidationIssue

	contentLower := strings.ToLower(setupSection)

	// Check for placeholder values that shouldn't be in setup instructions
	placeholderPatterns := []struct {
		pattern string
		desc    string
	}{
		{`\b(?:your|my)[-_]?(?:host|server|ip|domain)\b`, "placeholder hostname"},
		{`\b(?:your|my)[-_]?(?:username|user|login)\b`, "placeholder username"},
		{`\b(?:your|my)[-_]?(?:password|secret|key|token)\b`, "placeholder credential"},
		{`\bexample\.com\b`, "example.com domain"},
		{`\b192\.168\.\d+\.\d+\b`, "private IP address"},
		{`\b10\.\d+\.\d+\.\d+\b`, "private IP address"},
		{`\blocalhost\b`, "localhost reference"},
	}

	for _, pp := range placeholderPatterns {
		re := regexp.MustCompile(`(?i)` + pp.pattern)
		if re.MatchString(setupSection) && !strings.Contains(contentLower, "replace") && !strings.Contains(contentLower, "substitute") {
			issues = append(issues, ValidationIssue{
				Severity:    SeverityMinor,
				Category:    CategoryVendorSetup,
				Location:    "Setup section",
				Message:     fmt.Sprintf("Found %s without clear instruction to replace it", pp.desc),
				Suggestion:  "Clarify that users should replace placeholder values with their own",
				SourceCheck: "static",
			})
			break // Only report once
		}
	}

	// Check if setup mentions the product name (flexible matching)
	if pkgCtx != nil && pkgCtx.Manifest != nil && pkgCtx.Manifest.Title != "" {
		titleLower := strings.ToLower(pkgCtx.Manifest.Title)
		productMentioned := false

		// Check exact match first
		if strings.Contains(contentLower, titleLower) {
			productMentioned = true
		} else {
			// Check for significant words from title (2+ chars, not common words)
			commonWords := map[string]bool{"the": true, "and": true, "for": true, "with": true, "from": true}
			words := strings.Fields(titleLower)
			for _, word := range words {
				if len(word) >= 2 && !commonWords[word] && strings.Contains(contentLower, word) {
					productMentioned = true
					break
				}
			}
		}

		if !productMentioned {
			issues = append(issues, ValidationIssue{
				Severity:    SeverityMinor,
				Category:    CategoryVendorSetup,
				Location:    "Setup section",
				Message:     fmt.Sprintf("Setup section doesn't mention the product '%s'", pkgCtx.Manifest.Title),
				Suggestion:  "Reference the product name when describing vendor-side setup steps",
				SourceCheck: "static",
			})
		}
	}

	// Check for numbered steps (good practice)
	hasNumberedSteps := regexp.MustCompile(`(?m)^\s*\d+\.\s+`).MatchString(setupSection)
	hasBulletSteps := regexp.MustCompile(`(?m)^\s*[-*]\s+`).MatchString(setupSection)

	if !hasNumberedSteps && hasBulletSteps {
		issues = append(issues, ValidationIssue{
			Severity:    SeverityMinor,
			Category:    CategoryVendorSetup,
			Location:    "Setup section",
			Message:     "Setup instructions use bullet points instead of numbered steps",
			Suggestion:  "Use numbered steps (1. 2. 3.) for sequential setup instructions",
			SourceCheck: "static",
		})
	}

	// Check for verification step
	verifyKeywords := []string{"verify", "verification", "test", "confirm", "check", "validate", "validation"}
	hasVerification := false
	for _, keyword := range verifyKeywords {
		if strings.Contains(contentLower, keyword) {
			hasVerification = true
			break
		}
	}

	if !hasVerification {
		issues = append(issues, ValidationIssue{
			Severity:    SeverityMajor,
			Category:    CategoryVendorSetup,
			Location:    "Setup section",
			Message:     "Setup instructions missing verification step",
			Suggestion:  "Add steps to verify the setup is working correctly",
			SourceCheck: "static",
		})
	}

	// Check for vendor-side configuration subsection
	issues = append(issues, v.checkVendorSideConfiguration(setupSection, pkgCtx)...)

	return issues
}

// checkVendorSideConfiguration validates that vendor-side setup is comprehensively documented
func (v *VendorSetupValidator) checkVendorSideConfiguration(setupSection string, pkgCtx *PackageContext) []ValidationIssue {
	var issues []ValidationIssue

	contentLower := strings.ToLower(setupSection)

	// Check for vendor-side configuration section
	vendorConfigIndicators := []string{
		"vendor", "side configuration", "side setup", "configure on",
		"on the " + strings.ToLower(pkgCtx.Manifest.Title),
		"in the " + strings.ToLower(pkgCtx.Manifest.Title),
	}

	hasVendorConfig := false
	for _, indicator := range vendorConfigIndicators {
		if strings.Contains(contentLower, indicator) {
			hasVendorConfig = true
			break
		}
	}

	// Also check for common vendor configuration patterns
	vendorConfigPatterns := []string{
		"gui", "console", "admin", "portal", "dashboard",
		"navigate to", "click on", "select", "enable",
	}
	vendorPatternsFound := 0
	for _, pattern := range vendorConfigPatterns {
		if strings.Contains(contentLower, pattern) {
			vendorPatternsFound++
		}
	}

	if !hasVendorConfig && vendorPatternsFound < 3 {
		issues = append(issues, ValidationIssue{
			Severity:    SeverityMajor,
			Category:    CategoryVendorSetup,
			Location:    "Setup section",
			Message:     "Missing or insufficient vendor-side configuration instructions",
			Suggestion:  "Add a dedicated subsection for vendor/product-side configuration with step-by-step GUI or CLI instructions",
			SourceCheck: "static",
		})
	}

	// Check for prerequisites section
	prereqKeywords := []string{"prerequisite", "requirement", "what you need", "before you begin", "what do i need"}
	hasPrereqs := false
	for _, keyword := range prereqKeywords {
		if strings.Contains(contentLower, keyword) {
			hasPrereqs = true
			break
		}
	}

	// Check for common prerequisites content
	prereqContentIndicators := []string{
		"credential", "username", "password", "api key", "token",
		"network", "connectivity", "firewall", "port",
		"permission", "access", "admin",
	}
	prereqContentFound := 0
	for _, indicator := range prereqContentIndicators {
		if strings.Contains(contentLower, indicator) {
			prereqContentFound++
		}
	}

	if !hasPrereqs && prereqContentFound < 2 {
		issues = append(issues, ValidationIssue{
			Severity:    SeverityMajor,
			Category:    CategoryVendorSetup,
			Location:    "Setup section",
			Message:     "Missing prerequisites documentation",
			Suggestion:  "Add a section listing prerequisites: credentials, network access, required permissions",
			SourceCheck: "static",
		})
	}

	// Check for Kibana/Fleet setup instructions
	kibanaSetupIndicators := []string{
		"kibana", "fleet", "management", "integrations",
		"add integration", "add the integration",
	}
	hasKibanaSetup := false
	for _, indicator := range kibanaSetupIndicators {
		if strings.Contains(contentLower, indicator) {
			hasKibanaSetup = true
			break
		}
	}

	if !hasKibanaSetup {
		issues = append(issues, ValidationIssue{
			Severity:    SeverityMajor,
			Category:    CategoryVendorSetup,
			Location:    "Setup section",
			Message:     "Missing Kibana/Fleet setup instructions",
			Suggestion:  "Add step-by-step instructions for adding the integration in Kibana: Management > Integrations > Search > Add",
			SourceCheck: "static",
		})
	}

	// Check for configuration parameters documentation
	configParamIndicators := []string{
		"host", "url", "endpoint", "address", "port",
		"username", "password", "api key", "credential",
		"interval", "timeout", "period",
	}
	configParamsFound := 0
	for _, indicator := range configParamIndicators {
		if strings.Contains(contentLower, indicator) {
			configParamsFound++
		}
	}

	if configParamsFound < 3 {
		issues = append(issues, ValidationIssue{
			Severity:    SeverityMinor,
			Category:    CategoryVendorSetup,
			Location:    "Setup section",
			Message:     "Limited configuration parameter documentation",
			Suggestion:  "Document key configuration parameters: host/URL, credentials, polling interval, etc.",
			SourceCheck: "static",
		})
	}

	return issues
}

// buildLLMContext creates context information for the LLM validator
func (v *VendorSetupValidator) buildLLMContext(pkgCtx *PackageContext, vendorLinks []ServiceInfoLink, setupSection string) []string {
	var context []string

	// Add product information
	if pkgCtx.Manifest != nil {
		context = append(context,
			fmt.Sprintf("PRODUCT: %s", pkgCtx.Manifest.Title),
			fmt.Sprintf("DESCRIPTION: %s", pkgCtx.Manifest.Description),
		)
	}

	// Add vendor links for reference
	if len(vendorLinks) > 0 {
		linkList := []string{}
		for _, link := range vendorLinks {
			linkList = append(linkList, fmt.Sprintf("- %s: %s", link.Text, link.URL))
		}
		context = append(context,
			"VENDOR DOCUMENTATION LINKS:",
			strings.Join(linkList, "\n"),
		)
	}

	// Add data streams for context
	if len(pkgCtx.DataStreams) > 0 {
		dsNames := []string{}
		for _, ds := range pkgCtx.DataStreams {
			dsNames = append(dsNames, ds.Name)
		}
		context = append(context,
			fmt.Sprintf("DATA STREAMS: %s", strings.Join(dsNames, ", ")),
		)
	}

	context = append(context,
		"",
		"VALIDATION INSTRUCTIONS:",
		"1. Compare the setup instructions against your knowledge of how to configure " + pkgCtx.Manifest.Title,
		"2. Check if the steps match what the vendor documentation links would describe",
		"3. Flag any incorrect commands, paths, ports, or configuration values",
		"4. Flag any missing critical setup steps",
		"5. Be specific about what is wrong and provide corrected text",
	)

	return context
}

// CategoryVendorSetup is the category for vendor setup validation issues
const CategoryVendorSetup ValidationCategory = "vendor_setup"

