// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package validators

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/elastic/elastic-package/internal/llmagent/docagent/parsing"
)

const (
	completenessValidatorName        = "completeness_validator"
	completenessValidatorDescription = "Validates that all required content is present in the documentation"
)

const completenessValidatorInstruction = `You are a documentation completeness validator for Elastic integration packages.
Your task is to validate that all required content is present and comprehensive.

## Input
The documentation content to validate is provided in the user message.
You may also receive static validation context including data stream names and input types.

## Checks
1. All data streams from the package are documented
2. Setup instructions cover both vendor-side and Kibana-side configuration
3. Validation steps are provided to verify the integration works
4. Troubleshooting section addresses common issues specific to this integration
5. Reference section includes field documentation
6. Agent deployment section includes Fleet enrollment and integration setup steps
7. Validation section includes agent status, Discover data check, and dashboard verification

## DO NOT FLAG (these are acceptable):
- LLM-generated content disclosure format (any mention of AI/LLM generation is fine)
- SSL/TLS configuration without inline examples (link to guide is sufficient)
- UDP warnings that mention "data loss" - any warning format is acceptable
- Scaling recommendations that are general rather than highly specific

## Output Format
Output a JSON object with this exact structure:
{"valid": true/false, "score": 0-100, "issues": [{"severity": "critical|major|minor", "category": "completeness", "location": "Section Name", "message": "Issue description", "suggestion": "How to fix"}]}

Set valid=false only if:
- Missing data streams documentation (critical)
- Missing setup instructions (critical)
- Completely missing troubleshooting section (major)

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
			scope:       ScopeFullDocument, // Completeness validation requires full document
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

	// Check 4: LLM-generated disclosure (required for AI-generated content)
	result.Issues = append(result.Issues, v.checkLLMGeneratedDisclosure(content)...)

	// Check 5: Reference/fields documentation
	result.Issues = append(result.Issues, v.checkReferenceSection(content, pkgCtx)...)

	// Check 6: Agent deployment section completeness (network requirements)
	result.Issues = append(result.Issues, v.checkAgentDeploymentCompleteness(content, pkgCtx)...)

	// Check 7: Troubleshooting section completeness (input-specific guidance)
	result.Issues = append(result.Issues, v.checkTroubleshootingCompleteness(content, pkgCtx)...)

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

// checkValidationSteps verifies validation/testing instructions exist with required content
func (v *CompletenessValidator) checkValidationSteps(content string) []ValidationIssue {
	var issues []ValidationIssue

	contentLower := strings.ToLower(content)

	// Check for validation section
	hasValidationSection := strings.Contains(contentLower, "### validation") ||
		strings.Contains(contentLower, "## validation") ||
		strings.Contains(contentLower, "### verification")

	if !hasValidationSection {
		issues = append(issues, ValidationIssue{
			Severity:    SeverityMajor,
			Category:    CategoryCompleteness,
			Location:    "Validation",
			Message:     "Missing Validation section",
			Suggestion:  "Add a '### Validation' subsection under 'How do I deploy this integration?'",
			SourceCheck: "static",
		})
		return issues
	}

	// Extract validation section for detailed checks
	validationSection := parsing.ExtractSectionByKeyword(content, []string{"validation"})
	if validationSection == "" {
		validationSection = content // Fall back to full content
	}
	validationLower := strings.ToLower(validationSection)

	// Check 1: Agent status verification
	hasAgentStatus := strings.Contains(validationLower, "agent") &&
		(strings.Contains(validationLower, "status") ||
			strings.Contains(validationLower, "healthy") ||
			strings.Contains(validationLower, "fleet"))

	if !hasAgentStatus {
		issues = append(issues, ValidationIssue{
			Severity:    SeverityMajor,
			Category:    CategoryCompleteness,
			Location:    "Validation",
			Message:     "Missing agent status verification step",
			Suggestion:  "Add step to verify Elastic Agent status in Fleet (e.g., 'Navigate to Management → Fleet → Agents')",
			SourceCheck: "static",
		})
	}

	// Check 2: Discover/data verification
	hasDiscoverCheck := strings.Contains(validationLower, "discover") ||
		(strings.Contains(validationLower, "data") && strings.Contains(validationLower, "appear"))

	if !hasDiscoverCheck {
		issues = append(issues, ValidationIssue{
			Severity:    SeverityMajor,
			Category:    CategoryCompleteness,
			Location:    "Validation",
			Message:     "Missing data verification in Discover",
			Suggestion:  "Add step to check for incoming data in Analytics → Discover",
			SourceCheck: "static",
		})
	}

	// Check 3: Dataset filter guidance
	hasDatasetFilter := strings.Contains(validationLower, "data_stream.dataset") ||
		strings.Contains(validationLower, "dataset") ||
		strings.Contains(validationLower, "logs-") ||
		strings.Contains(validationLower, "metrics-")

	if !hasDatasetFilter {
		issues = append(issues, ValidationIssue{
			Severity:    SeverityMinor,
			Category:    CategoryCompleteness,
			Location:    "Validation",
			Message:     "Missing dataset filtering guidance",
			Suggestion:  "Add guidance on filtering by data_stream.dataset to verify specific data streams",
			SourceCheck: "static",
		})
	}

	// Check 4: Dashboard verification
	hasDashboardCheck := strings.Contains(validationLower, "dashboard") ||
		strings.Contains(validationLower, "visualization") ||
		strings.Contains(validationLower, "assets")

	if !hasDashboardCheck {
		issues = append(issues, ValidationIssue{
			Severity:    SeverityMinor,
			Category:    CategoryCompleteness,
			Location:    "Validation",
			Message:     "Missing dashboard verification step",
			Suggestion:  "Add step to verify data appears in the integration's dashboards (Assets tab)",
			SourceCheck: "static",
		})
	}

	return issues
}

// checkLLMGeneratedDisclosure verifies that LLM-generated content includes a disclosure note
func (v *CompletenessValidator) checkLLMGeneratedDisclosure(content string) []ValidationIssue {
	var issues []ValidationIssue

	contentLower := strings.ToLower(content)

	// Look for LLM/AI generation disclosure markers
	disclosureMarkers := []string{
		"generated by ai",
		"ai-generated",
		"llm-generated",
		"generated using ai",
		"generated with ai",
		"auto-generated",
		"automatically generated",
		"machine-generated",
		"generated by a large language model",
		"generated by an llm",
		"this documentation was generated",
		"this document was generated",
		"generated documentation",
	}

	hasDisclosure := false
	for _, marker := range disclosureMarkers {
		if strings.Contains(contentLower, marker) {
			hasDisclosure = true
			break
		}
	}

	if !hasDisclosure {
		issues = append(issues, ValidationIssue{
			Severity:    SeverityMajor,
			Category:    CategoryCompleteness,
			Location:    "Header or Footer",
			Message:     "Missing LLM-generated content disclosure",
			Suggestion:  "Add a note indicating this documentation was generated by AI (e.g., 'This documentation was generated using AI and should be reviewed for accuracy.')",
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

	// Check for sample events (including template syntax)
	hasSampleEvents := strings.Contains(contentLower, "sample event") ||
		strings.Contains(contentLower, "example event") ||
		strings.Contains(contentLower, "```json") || // JSON code blocks often contain sample events
		strings.Contains(content, "{{event") || // Template syntax for events
		strings.Contains(content, "{{fields") // Template syntax for fields (implies event context)

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

	// Check that {{event}} and {{fields}} templates use actual data stream names
	if pkgCtx != nil && len(pkgCtx.DataStreams) > 0 {
		issues = append(issues, v.checkDataStreamTemplates(content, pkgCtx)...)
	}

	return issues
}

// checkDataStreamTemplates validates that {{event}} and {{fields}} templates use actual data stream names
func (v *CompletenessValidator) checkDataStreamTemplates(content string, pkgCtx *PackageContext) []ValidationIssue {
	var issues []ValidationIssue

	// Build a set of valid data stream names
	validNames := make(map[string]bool)
	dataStreamsWithExamples := make(map[string]bool)
	for _, ds := range pkgCtx.DataStreams {
		validNames[ds.Name] = true
		if ds.HasExampleEvent {
			dataStreamsWithExamples[ds.Name] = true
		}
	}

	// Pattern to match {{fields "name"}} and {{event "name"}} templates
	fieldsPattern := regexp.MustCompile(`\{\{fields\s+"([^"]+)"\}\}`)
	eventPattern := regexp.MustCompile(`\{\{event\s+"([^"]+)"\}\}`)

	// Check {{fields}} templates
	fieldsMatches := fieldsPattern.FindAllStringSubmatch(content, -1)
	foundFields := make(map[string]bool)
	for _, match := range fieldsMatches {
		if len(match) > 1 {
			dsName := match[1]
			foundFields[dsName] = true
			// Check if "datastream" is used literally (common mistake)
			if dsName == "datastream" {
				issues = append(issues, ValidationIssue{
					Severity:    SeverityCritical,
					Category:    CategoryCompleteness,
					Location:    "Reference",
					Message:     "{{fields \"datastream\"}} uses literal 'datastream' instead of actual data stream name",
					Suggestion:  "Replace with actual data stream name, e.g., {{fields \"" + pkgCtx.DataStreams[0].Name + "\"}}",
					SourceCheck: "static",
				})
			} else if !validNames[dsName] {
				issues = append(issues, ValidationIssue{
					Severity:    SeverityMajor,
					Category:    CategoryCompleteness,
					Location:    "Reference",
					Message:     "{{fields \"" + dsName + "\"}} references unknown data stream '" + dsName + "'",
					Suggestion:  "Use one of the valid data stream names: " + v.joinDataStreamNames(pkgCtx.DataStreams),
					SourceCheck: "static",
				})
			}
		}
	}

	// Check {{event}} templates
	eventMatches := eventPattern.FindAllStringSubmatch(content, -1)
	foundEvents := make(map[string]bool)
	for _, match := range eventMatches {
		if len(match) > 1 {
			dsName := match[1]
			foundEvents[dsName] = true
			// Check if "datastream" is used literally (common mistake)
			if dsName == "datastream" {
				issues = append(issues, ValidationIssue{
					Severity:    SeverityCritical,
					Category:    CategoryCompleteness,
					Location:    "Reference",
					Message:     "{{event \"datastream\"}} uses literal 'datastream' instead of actual data stream name",
					Suggestion:  "Replace with actual data stream name, e.g., {{event \"" + pkgCtx.DataStreams[0].Name + "\"}}",
					SourceCheck: "static",
				})
			} else if !validNames[dsName] {
				issues = append(issues, ValidationIssue{
					Severity:    SeverityMajor,
					Category:    CategoryCompleteness,
					Location:    "Reference",
					Message:     "{{event \"" + dsName + "\"}} references unknown data stream '" + dsName + "'",
					Suggestion:  "Use one of the valid data stream names: " + v.joinDataStreamNames(pkgCtx.DataStreams),
					SourceCheck: "static",
				})
			}
		}
	}

	// Check for missing {{fields}} templates for each data stream
	for _, ds := range pkgCtx.DataStreams {
		if !foundFields[ds.Name] {
			issues = append(issues, ValidationIssue{
				Severity:    SeverityMajor,
				Category:    CategoryCompleteness,
				Location:    "Reference",
				Message:     "Missing {{fields \"" + ds.Name + "\"}} template for data stream '" + ds.Name + "'",
				Suggestion:  "Add {{fields \"" + ds.Name + "\"}} in the Reference section under the data stream heading",
				SourceCheck: "static",
			})
		}
		// Check for missing {{event}} templates for data streams that have example events
		if ds.HasExampleEvent && !foundEvents[ds.Name] {
			issues = append(issues, ValidationIssue{
				Severity:    SeverityMajor,
				Category:    CategoryCompleteness,
				Location:    "Reference",
				Message:     "Missing {{event \"" + ds.Name + "\"}} template for data stream '" + ds.Name + "' (has sample_event.json)",
				Suggestion:  "Add {{event \"" + ds.Name + "\"}} in the Reference section before {{fields \"" + ds.Name + "\"}}",
				SourceCheck: "static",
			})
		}
	}

	return issues
}

// joinDataStreamNames returns a comma-separated list of data stream names
func (v *CompletenessValidator) joinDataStreamNames(dataStreams []DataStreamInfo) string {
	names := make([]string, len(dataStreams))
	for i, ds := range dataStreams {
		names[i] = ds.Name
	}
	return strings.Join(names, ", ")
}

// checkAgentDeploymentCompleteness validates the agent deployment section has proper content
func (v *CompletenessValidator) checkAgentDeploymentCompleteness(content string, pkgCtx *PackageContext) []ValidationIssue {
	var issues []ValidationIssue

	contentLower := strings.ToLower(content)

	// Check for agent deployment section
	hasAgentDeployment := strings.Contains(contentLower, "### agent-based deployment") ||
		strings.Contains(contentLower, "### agent deployment") ||
		strings.Contains(contentLower, "### elastic agent")

	if !hasAgentDeployment {
		// Structure validator handles this, don't duplicate
		return issues
	}

	// Extract the agent deployment section
	deploymentSection := parsing.ExtractSectionByKeyword(content, []string{"agent"})

	// Check for key agent deployment content
	deploymentLower := strings.ToLower(deploymentSection)

	// Must have Fleet mention
	if !strings.Contains(deploymentLower, "fleet") {
		issues = append(issues, ValidationIssue{
			Severity:    SeverityMajor,
			Category:    CategoryCompleteness,
			Location:    "Agent-based deployment",
			Message:     "Missing Fleet enrollment instructions",
			Suggestion:  "Add instructions for enrolling the Elastic Agent in Fleet",
			SourceCheck: "static",
		})
	}

	// Must have integration addition steps
	if !strings.Contains(deploymentLower, "add") || !strings.Contains(deploymentLower, "integration") {
		issues = append(issues, ValidationIssue{
			Severity:    SeverityMajor,
			Category:    CategoryCompleteness,
			Location:    "Agent-based deployment",
			Message:     "Missing steps to add the integration",
			Suggestion:  "Add instructions for adding the integration to an agent policy",
			SourceCheck: "static",
		})
	}

	// Check for network requirements based on input types
	if pkgCtx != nil && pkgCtx.Manifest != nil {
		inputTypes := v.extractInputTypes(pkgCtx)
		networkInputs := v.getNetworkSensitiveInputs()

		hasNetworkSensitiveInput := false
		for inputType := range inputTypes {
			if _, ok := networkInputs[inputType]; ok {
				hasNetworkSensitiveInput = true
				break
			}
		}

		// If the integration uses network-sensitive inputs, check for network requirements
		if hasNetworkSensitiveInput {
			hasNetworkSection := strings.Contains(deploymentLower, "network") ||
				strings.Contains(deploymentLower, "port") ||
				strings.Contains(deploymentLower, "firewall") ||
				strings.Contains(contentLower, "| direction |") ||
				strings.Contains(contentLower, "| protocol |")

			if !hasNetworkSection {
				issues = append(issues, ValidationIssue{
					Severity:    SeverityMajor,
					Category:    CategoryCompleteness,
					Location:    "Agent-based deployment",
					Message:     "Missing network requirements for network-dependent inputs",
					Suggestion:  "Add a network requirements table specifying ports and protocols needed",
					SourceCheck: "static",
				})
			}
		}
	}

	return issues
}

// checkTroubleshootingCompleteness validates troubleshooting section has input-specific content
func (v *CompletenessValidator) checkTroubleshootingCompleteness(content string, pkgCtx *PackageContext) []ValidationIssue {
	var issues []ValidationIssue

	contentLower := strings.ToLower(content)

	// Check for troubleshooting section
	hasTroubleshooting := strings.Contains(contentLower, "## troubleshooting") ||
		strings.Contains(contentLower, "## common issues")

	if !hasTroubleshooting {
		// Structure validator handles this, don't duplicate
		return issues
	}

	// Extract the troubleshooting section
	troubleshootingSection := parsing.ExtractSectionByKeyword(content, []string{"troubleshooting"})
	troubleshootingLower := strings.ToLower(troubleshootingSection)

	// If section extraction failed, fall back to searching entire content from troubleshooting header
	// This handles cases where the section extraction has edge case issues
	if len(troubleshootingLower) < 100 {
		// Section seems too short, use full content from troubleshooting onwards
		idx := strings.Index(contentLower, "## troubleshooting")
		if idx == -1 {
			idx = strings.Index(contentLower, "## common issues")
		}
		if idx != -1 {
			troubleshootingLower = contentLower[idx:]
		}
	}

	// Check for link to common troubleshooting documentation
	hasCommonTroubleshootingLink := strings.Contains(troubleshootingLower, "common-problems") ||
		strings.Contains(troubleshootingLower, "common problems") ||
		strings.Contains(troubleshootingLower, "elastic.co/docs/troubleshoot")

	if !hasCommonTroubleshootingLink {
		issues = append(issues, ValidationIssue{
			Severity:    SeverityMinor,
			Category:    CategoryCompleteness,
			Location:    "Troubleshooting",
			Message:     "Missing link to common Elastic troubleshooting documentation",
			Suggestion:  "Add: 'For help with Elastic ingest tools, check [Common problems](https://www.elastic.co/docs/troubleshoot/ingest/fleet/common-problems).'",
			SourceCheck: "static",
		})
	}

	// Check for vendor-specific issues subsection
	hasVendorSpecificIssues := strings.Contains(troubleshootingLower, "vendor-specific") ||
		strings.Contains(troubleshootingLower, "vendor resources") ||
		strings.Contains(troubleshootingLower, "common configuration")

	if !hasVendorSpecificIssues {
		issues = append(issues, ValidationIssue{
			Severity:    SeverityMinor,
			Category:    CategoryCompleteness,
			Location:    "Troubleshooting",
			Message:     "Missing vendor-specific or configuration issues subsection",
			Suggestion:  "Add '### Common configuration issues' or '### Vendor-specific issues' with integration-specific problems and solutions",
			SourceCheck: "static",
		})
	}

	return issues
}

// extractInputTypes gets all input types from the manifest
func (v *CompletenessValidator) extractInputTypes(pkgCtx *PackageContext) map[string]bool {
	inputTypes := make(map[string]bool)

	if pkgCtx == nil || pkgCtx.Manifest == nil {
		return inputTypes
	}

	for _, pt := range pkgCtx.Manifest.PolicyTemplates {
		for _, input := range pt.Inputs {
			if input.Type != "" {
				inputTypes[input.Type] = true
			}
		}
	}

	return inputTypes
}

// getNetworkSensitiveInputs returns inputs that require network configuration documentation
func (v *CompletenessValidator) getNetworkSensitiveInputs() map[string]string {
	return map[string]string{
		"tcp":                "TCP listener - requires port configuration",
		"udp":                "UDP listener - requires port configuration",
		"httpjson":           "API polling - requires outbound HTTPS access",
		"http_endpoint":      "Webhook receiver - requires inbound port",
		"kafka":              "Kafka consumer - requires broker connectivity",
		"aws-s3":             "AWS S3 - requires AWS API access",
		"aws-cloudwatch":     "CloudWatch - requires AWS API access",
		"gcs":                "GCS - requires GCP API access",
		"azure-blob-storage": "Azure Blob - requires Azure API access",
		"azure-eventhub":     "Event Hub - requires Azure connectivity",
		"gcp-pubsub":         "Pub/Sub - requires GCP API access",
		"cel":                "CEL - requires API access",
		"sql":                "SQL - requires database connectivity",
		"netflow":            "NetFlow - requires UDP port",
		"lumberjack":         "Beats protocol - requires TCP port",
		"entity-analytics":   "Entity provider - requires API access",
		"o365audit":          "Office 365 - requires Microsoft API access",
	}
}

