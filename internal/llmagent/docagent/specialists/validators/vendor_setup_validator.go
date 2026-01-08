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
// This extracts setup-related information to provide context for LLM validation
func (v *VendorSetupValidator) StaticValidate(ctx context.Context, content string, pkgCtx *PackageContext) (*StagedValidationResult, error) {
	result := &StagedValidationResult{
		Stage: StageAccuracy,
		Valid: true,
		Score: 100,
	}

	if pkgCtx == nil {
		return result, nil
	}

	// Extract setup section from content
	setupSection := v.extractSetupSection(content)
	if setupSection == "" {
		// No setup section found - flag as issue
		result.Issues = append(result.Issues, ValidationIssue{
			Severity:    SeverityMajor,
			Category:    CategoryVendorSetup,
			Location:    "Document",
			Message:     "No setup/deployment section found in documentation",
			Suggestion:  "Add a setup section with vendor-specific configuration instructions",
			SourceCheck: "static",
		})
		result.Valid = false
		return result, nil
	}

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

	// Store context for LLM validation
	if len(vendorLinks) > 0 || pkgCtx.Manifest != nil {
		result.Suggestions = v.buildLLMContext(pkgCtx, vendorLinks, setupSection)
	}

	return result, nil
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

