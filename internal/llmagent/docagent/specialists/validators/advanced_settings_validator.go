// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package validators

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/elastic/elastic-package/internal/packages"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/genai"
)

const (
	advancedSettingsValidatorName        = "advanced_settings_validator"
	advancedSettingsValidatorDescription = "Validates that advanced settings and their gotchas are properly documented"
)

const advancedSettingsValidatorInstruction = `You are a documentation validator specializing in advanced configuration settings.
Your task is to verify that important configuration gotchas and caveats from the manifest.yml are properly documented.

## Input
The documentation content to validate is provided in the user message.
You will also receive context about advanced settings extracted from the package manifests.

## What to Check
1. **Security Warnings**: Settings that compromise security, expose sensitive data, or should only be used in specific environments
2. **Debug/Development Settings**: Settings that should NOT be enabled in production
3. **Performance Impacts**: Settings that may affect performance, resource usage, or scalability
4. **Complex Configurations**: YAML/JSON settings that require careful formatting
5. **Sensitive Fields**: Password, secret, or credential fields that need special handling
6. **Non-obvious Defaults**: Default values that users should be aware of

## Output Format
Output a JSON object with this exact structure:
{"valid": true/false, "score": 0-100, "issues": [{"severity": "critical|major|minor", "category": "advanced_settings", "location": "Section Name", "message": "Issue description", "suggestion": "How to fix"}]}

## Severity Guidelines
- critical: Security-related gotchas not documented (e.g., debug mode exposes data)
- major: Important configuration caveats not mentioned (e.g., performance impacts)
- minor: Nice-to-have documentation improvements (e.g., better examples)

## IMPORTANT
Output ONLY the JSON object. No other text.`

// AdvancedSettingGotcha represents a setting that has important caveats
type AdvancedSettingGotcha struct {
	Name        string   // Variable name
	Title       string   // Human-readable title
	Description string   // Full description from manifest
	Type        string   // Variable type (bool, yaml, password, etc.)
	Location    string   // Where in the manifest (package-level or data stream)
	GotchaTypes []string // Types of gotchas detected (security, debug, performance, etc.)
	IsSecret    bool     // Whether this is a secret/password field
	ShowUser    bool     // Whether shown to user by default
}

// AdvancedSettingsValidator validates that advanced settings gotchas are documented
type AdvancedSettingsValidator struct {
	BaseStagedValidator
}

// NewAdvancedSettingsValidator creates a new advanced settings validator
func NewAdvancedSettingsValidator() *AdvancedSettingsValidator {
	return &AdvancedSettingsValidator{
		BaseStagedValidator: BaseStagedValidator{
			name:        advancedSettingsValidatorName,
			description: advancedSettingsValidatorDescription,
			stage:       StageCompleteness, // Part of completeness checking
			instruction: advancedSettingsValidatorInstruction,
		},
	}
}

// SupportsStaticValidation returns true - this validator has extensive static checks
func (v *AdvancedSettingsValidator) SupportsStaticValidation() bool {
	return true
}

// StaticValidate performs static validation of advanced settings documentation
func (v *AdvancedSettingsValidator) StaticValidate(ctx context.Context, content string, pkgCtx *PackageContext) (*StagedValidationResult, error) {
	result := &StagedValidationResult{
		Stage: StageCompleteness,
		Valid: true,
		Score: 100,
	}

	if pkgCtx == nil || pkgCtx.Manifest == nil {
		return result, nil
	}

	// Extract advanced settings with gotchas from manifest
	gotchas := v.extractAdvancedSettingGotchas(pkgCtx)

	if len(gotchas) == 0 {
		return result, nil
	}

	// Check if each gotcha is documented
	contentLower := strings.ToLower(content)
	var issues []ValidationIssue

	for _, gotcha := range gotchas {
		issues = append(issues, v.checkGotchaDocumented(contentLower, gotcha)...)
	}

	result.Issues = issues

	// Calculate score and validity
	if len(issues) > 0 {
		criticalCount := 0
		majorCount := 0
		for _, issue := range issues {
			switch issue.Severity {
			case SeverityCritical:
				criticalCount++
			case SeverityMajor:
				majorCount++
			}
		}

		// Adjust score based on issues
		result.Score = 100 - (criticalCount * 25) - (majorCount * 10)
		if result.Score < 0 {
			result.Score = 0
		}

		if criticalCount > 0 || majorCount > 0 {
			result.Valid = false
		}
	}

	// Add suggestions for generator
	if len(gotchas) > 0 {
		result.Suggestions = v.buildSuggestions(gotchas, issues)
	}

	return result, nil
}

// extractAdvancedSettingGotchas extracts settings with important caveats from the manifest
func (v *AdvancedSettingsValidator) extractAdvancedSettingGotchas(pkgCtx *PackageContext) []AdvancedSettingGotcha {
	var gotchas []AdvancedSettingGotcha

	// Extract from package-level policy templates
	if pkgCtx.Manifest != nil {
		for _, pt := range pkgCtx.Manifest.PolicyTemplates {
			for _, input := range pt.Inputs {
				for _, varDef := range input.Vars {
					if gotcha := v.analyzeVariableStruct(varDef, "package"); gotcha != nil {
						gotchas = append(gotchas, *gotcha)
					}
				}
			}
			// Also check policy template level vars
			for _, varDef := range pt.Vars {
				if gotcha := v.analyzeVariableStruct(varDef, "package"); gotcha != nil {
					gotchas = append(gotchas, *gotcha)
				}
			}
		}

		// Also check package-level vars
		for _, varDef := range pkgCtx.Manifest.Vars {
			if gotcha := v.analyzeVariableStruct(varDef, "package"); gotcha != nil {
				gotchas = append(gotchas, *gotcha)
			}
		}
	}

	// Extract from data stream manifests
	for _, ds := range pkgCtx.DataStreams {
		location := fmt.Sprintf("data_stream/%s", ds.Name)
		// Note: DataStreamInfo doesn't have Vars directly, but we can analyze from raw manifest
		// For now, we check common patterns from the package-level context
		_ = location // Used when we extend to read data stream manifests
	}

	return gotchas
}

// analyzeVariableStruct checks if a packages.Variable has important gotchas
func (v *AdvancedSettingsValidator) analyzeVariableStruct(varDef packages.Variable, location string) *AdvancedSettingGotcha {
	gotcha := &AdvancedSettingGotcha{
		Name:        varDef.Name,
		Title:       varDef.Title,
		Description: varDef.Description,
		Type:        varDef.Type,
		Location:    location,
		IsSecret:    varDef.Secret,
		ShowUser:    varDef.ShowUser,
		GotchaTypes: []string{},
	}

	descLower := strings.ToLower(varDef.Description)
	name := varDef.Name

	// Check for security-related gotchas
	securityPatterns := []string{
		"compromise", "security", "sensitive", "expose",
		"should only be used for debugging", "debug only",
		"not recommended for production", "production use",
	}
	for _, pattern := range securityPatterns {
		if strings.Contains(descLower, pattern) {
			gotcha.GotchaTypes = append(gotcha.GotchaTypes, "security")
			break
		}
	}

	// Check for debug/development settings
	debugPatterns := []string{
		"debug", "debugging", "development", "troubleshoot",
		"request tracer", "verbose", "trace",
	}
	for _, pattern := range debugPatterns {
		if strings.Contains(descLower, pattern) || strings.Contains(strings.ToLower(name), pattern) {
			gotcha.GotchaTypes = append(gotcha.GotchaTypes, "debug")
			break
		}
	}

	// Check for performance-related settings
	performancePatterns := []string{
		"performance", "resource", "memory", "cpu",
		"batch", "buffer", "timeout", "rate limit",
	}
	for _, pattern := range performancePatterns {
		if strings.Contains(descLower, pattern) {
			gotcha.GotchaTypes = append(gotcha.GotchaTypes, "performance")
			break
		}
	}

	// Check for complex configurations (YAML/JSON types)
	if gotcha.Type == "yaml" || gotcha.Type == "json" {
		gotcha.GotchaTypes = append(gotcha.GotchaTypes, "complex_config")
	}

	// Check for sensitive/secret fields
	if gotcha.IsSecret || gotcha.Type == "password" || strings.Contains(strings.ToLower(name), "password") ||
		strings.Contains(strings.ToLower(name), "secret") || strings.Contains(strings.ToLower(name), "api_key") {
		gotcha.GotchaTypes = append(gotcha.GotchaTypes, "sensitive")
	}

	// Check for SSL/TLS configuration
	if strings.Contains(strings.ToLower(name), "ssl") || strings.Contains(strings.ToLower(name), "tls") ||
		strings.Contains(strings.ToLower(name), "certificate") {
		gotcha.GotchaTypes = append(gotcha.GotchaTypes, "ssl_config")
	}

	// Only return if there are actual gotchas
	if len(gotcha.GotchaTypes) > 0 {
		return gotcha
	}

	return nil
}

// checkGotchaDocumented verifies that a gotcha is mentioned in the documentation
func (v *AdvancedSettingsValidator) checkGotchaDocumented(contentLower string, gotcha AdvancedSettingGotcha) []ValidationIssue {
	var issues []ValidationIssue

	// Check if the setting is mentioned at all
	settingMentioned := strings.Contains(contentLower, strings.ToLower(gotcha.Name)) ||
		strings.Contains(contentLower, strings.ToLower(gotcha.Title))

	for _, gotchaType := range gotcha.GotchaTypes {
		switch gotchaType {
		case "security":
			// Security gotchas are critical - must be documented
			if !settingMentioned || !v.isSecurityWarningDocumented(contentLower, gotcha) {
				issues = append(issues, ValidationIssue{
					Severity:    SeverityCritical,
					Category:    CategoryCompleteness,
					Location:    "Advanced Settings",
					Message:     fmt.Sprintf("Security warning for '%s' is not documented: %s", gotcha.Title, v.extractSecurityWarning(gotcha.Description)),
					Suggestion:  fmt.Sprintf("Add a warning about the security implications of enabling '%s'", gotcha.Title),
					SourceCheck: "static",
				})
			}

		case "debug":
			// Debug settings should have clear warnings
			if !settingMentioned || !v.isDebugWarningDocumented(contentLower, gotcha) {
				issues = append(issues, ValidationIssue{
					Severity:    SeverityMajor,
					Category:    CategoryCompleteness,
					Location:    "Advanced Settings",
					Message:     fmt.Sprintf("Debug setting '%s' is not documented with production warning", gotcha.Title),
					Suggestion:  fmt.Sprintf("Document that '%s' should only be used for debugging/troubleshooting", gotcha.Title),
					SourceCheck: "static",
				})
			}

		case "ssl_config":
			// SSL configuration should be documented if present
			if !v.isSSLConfigDocumented(contentLower) {
				issues = append(issues, ValidationIssue{
					Severity:    SeverityMajor,
					Category:    CategoryCompleteness,
					Location:    "Advanced Settings",
					Message:     fmt.Sprintf("SSL/TLS configuration '%s' is not documented", gotcha.Title),
					Suggestion:  "Add documentation about SSL/TLS configuration options and certificate setup",
					SourceCheck: "static",
				})
			}

		case "sensitive":
			// Sensitive fields should mention secure handling
			if settingMentioned && !v.isSensitiveHandlingDocumented(contentLower, gotcha) {
				issues = append(issues, ValidationIssue{
					Severity:    SeverityMinor,
					Category:    CategoryCompleteness,
					Location:    "Advanced Settings",
					Message:     fmt.Sprintf("Sensitive field '%s' could use better security guidance", gotcha.Title),
					Suggestion:  "Consider adding guidance on secure credential management",
					SourceCheck: "static",
				})
			}

		case "complex_config":
			// Complex YAML/JSON configs should have examples
			if settingMentioned && !v.hasConfigurationExample(contentLower, gotcha) {
				issues = append(issues, ValidationIssue{
					Severity:    SeverityMinor,
					Category:    CategoryCompleteness,
					Location:    "Advanced Settings",
					Message:     fmt.Sprintf("Complex configuration '%s' lacks examples", gotcha.Title),
					Suggestion:  fmt.Sprintf("Add a YAML/JSON example for '%s' configuration", gotcha.Title),
					SourceCheck: "static",
				})
			}
		}
	}

	return issues
}

// isSecurityWarningDocumented checks if the security warning is present in docs
func (v *AdvancedSettingsValidator) isSecurityWarningDocumented(contentLower string, gotcha AdvancedSettingGotcha) bool {
	securityTerms := []string{
		"security", "warning", "caution", "not recommended",
		"debug only", "debugging only", "troubleshooting only",
		"compromises", "exposes", "sensitive",
	}

	// Check if any security-related terms appear near the setting name
	settingLower := strings.ToLower(gotcha.Name)
	for _, term := range securityTerms {
		// Check if both the setting and a security term appear
		if strings.Contains(contentLower, settingLower) && strings.Contains(contentLower, term) {
			return true
		}
	}

	return false
}

// isDebugWarningDocumented checks if debug setting has appropriate warning
func (v *AdvancedSettingsValidator) isDebugWarningDocumented(contentLower string, gotcha AdvancedSettingGotcha) bool {
	debugWarningTerms := []string{
		"debug", "debugging", "troubleshoot", "development",
		"not for production", "disable in production",
		"temporary", "diagnostic",
	}

	for _, term := range debugWarningTerms {
		if strings.Contains(contentLower, term) {
			return true
		}
	}

	return false
}

// isSSLConfigDocumented checks if SSL configuration is documented
func (v *AdvancedSettingsValidator) isSSLConfigDocumented(contentLower string) bool {
	sslTerms := []string{
		"ssl", "tls", "certificate", "https",
		"secure connection", "encryption",
	}

	matchCount := 0
	for _, term := range sslTerms {
		if strings.Contains(contentLower, term) {
			matchCount++
		}
	}

	// Require at least 2 SSL-related terms for proper documentation
	return matchCount >= 2
}

// isSensitiveHandlingDocumented checks if sensitive data handling is documented
func (v *AdvancedSettingsValidator) isSensitiveHandlingDocumented(contentLower string, gotcha AdvancedSettingGotcha) bool {
	sensitiveTerms := []string{
		"secret", "secure", "credential", "password",
		"api key", "token", "encrypted",
	}

	for _, term := range sensitiveTerms {
		if strings.Contains(contentLower, term) {
			return true
		}
	}

	return false
}

// hasConfigurationExample checks if there's a code example for the setting
func (v *AdvancedSettingsValidator) hasConfigurationExample(contentLower string, gotcha AdvancedSettingGotcha) bool {
	// Check for code blocks with YAML or the setting name
	codeBlockPattern := regexp.MustCompile("```(?:yaml|yml|json)?[^`]*" + strings.ToLower(gotcha.Name) + "[^`]*```")
	return codeBlockPattern.MatchString(contentLower)
}

// extractSecurityWarning extracts the security-related warning from description
func (v *AdvancedSettingsValidator) extractSecurityWarning(description string) string {
	// Look for sentences containing security-related keywords
	sentences := strings.Split(description, ".")
	for _, sentence := range sentences {
		lower := strings.ToLower(sentence)
		if strings.Contains(lower, "security") || strings.Contains(lower, "compromise") ||
			strings.Contains(lower, "should only") || strings.Contains(lower, "debug") {
			return strings.TrimSpace(sentence) + "."
		}
	}
	return description
}

// buildSuggestions creates actionable suggestions for the generator
func (v *AdvancedSettingsValidator) buildSuggestions(gotchas []AdvancedSettingGotcha, issues []ValidationIssue) []string {
	var suggestions []string

	suggestions = append(suggestions, "## Advanced Settings Documentation Required")

	// Group gotchas by type
	securityGotchas := []AdvancedSettingGotcha{}
	debugGotchas := []AdvancedSettingGotcha{}
	sslGotchas := []AdvancedSettingGotcha{}

	for _, g := range gotchas {
		for _, t := range g.GotchaTypes {
			switch t {
			case "security":
				securityGotchas = append(securityGotchas, g)
			case "debug":
				debugGotchas = append(debugGotchas, g)
			case "ssl_config":
				sslGotchas = append(sslGotchas, g)
			}
		}
	}

	if len(securityGotchas) > 0 {
		suggestions = append(suggestions, "\n### Security Warnings (CRITICAL)")
		for _, g := range securityGotchas {
			suggestions = append(suggestions, fmt.Sprintf("- **%s**: %s", g.Title, v.extractSecurityWarning(g.Description)))
		}
	}

	if len(debugGotchas) > 0 {
		suggestions = append(suggestions, "\n### Debug/Development Settings")
		for _, g := range debugGotchas {
			suggestions = append(suggestions, fmt.Sprintf("- **%s**: Document that this should only be used for debugging", g.Title))
		}
	}

	if len(sslGotchas) > 0 {
		suggestions = append(suggestions, "\n### SSL/TLS Configuration")
		suggestions = append(suggestions, "Document SSL configuration options including certificate setup")
	}

	return suggestions
}

// Build creates the underlying ADK agent for LLM validation.
func (v *AdvancedSettingsValidator) Build(ctx context.Context, cfg AgentConfig) (agent.Agent, error) {
	return llmagent.New(llmagent.Config{
		Name:        v.Name(),
		Description: v.Description(),
		Model:       cfg.Model,
		Instruction: v.instruction,
		Tools:       cfg.Tools,
		Toolsets:    cfg.Toolsets,
		GenerateContentConfig: &genai.GenerateContentConfig{
			ResponseMIMEType: "application/json",
		},
	})
}

