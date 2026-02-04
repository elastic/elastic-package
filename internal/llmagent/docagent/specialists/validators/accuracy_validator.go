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
	accuracyValidatorName        = "accuracy_validator"
	accuracyValidatorDescription = "Validates content accuracy against package metadata and service info"
)

const accuracyValidatorInstruction = `You are a documentation accuracy validator for Elastic integration packages.
Your task is to validate that the content is accurate and matches the source data.

## Input
The documentation content to validate is provided in the user message.
You may also receive static validation context with issues already identified.

## Context Provided (in user message)
- Package name, title, version from manifest.yml
- Data stream names from the package
- Static validation issues already found

## Checks
1. Service information aligns with any provided service_info context
2. No hallucinated features, capabilities, or version numbers
3. Configuration examples are syntactically correct (YAML, JSON)
4. All factual claims can be traced to source data
5. Version numbers and compatibility ranges are accurate
6. Feature descriptions match actual package capabilities

## DO NOT FLAG (these are acceptable):
- Private IP addresses (192.168.x.x, 10.x.x.x, 172.16-31.x.x) in examples
- ANY port numbers in examples (514, 443, 9090, 8080, etc.)
- Port privilege requirements (ports under 1024 needing root/capabilities)
- Example hostnames like "example.com" or "your-server.example.com"
- Vendor GUI field names that may vary across product versions
- Documentation links pointing to older API versions
- Missing "read-only user" or permission mentions
- Missing SSL/TLS certificate verification details
- Syslog format recommendations (RFC 3164, RFC 5424, RFC 6587, CEF, etc.)
- API URL format specifics
- Product name variations due to rebranding
- Missing vendor-specific configuration fields or CLI parameters
- Username/password permission requirements
- Global vs targeted configuration suggestions
- CLI command syntax variations (different firmware versions have different syntax)
- Process restart/reload commands (cp_log_export, systemctl, etc.)
- Binary vs text log file format details
- Feature-specific configuration steps beyond basic setup
- Log facility settings
- Missing optional configuration parameters

## Output Format
Output a JSON object with this exact structure:
{"valid": true/false, "score": 0-100, "issues": [{"severity": "critical|major|minor", "category": "accuracy", "location": "Section Name", "message": "Issue description", "suggestion": "How to fix"}]}

Set valid=false only if there are genuine factual inaccuracies.

## IMPORTANT
Output ONLY the JSON object. No other text.`

// AccuracyValidator validates content accuracy against package metadata (Section B)
type AccuracyValidator struct {
	BaseStagedValidator
}

// NewAccuracyValidator creates a new accuracy validator
func NewAccuracyValidator() *AccuracyValidator {
	return &AccuracyValidator{
		BaseStagedValidator: BaseStagedValidator{
			name:        accuracyValidatorName,
			description: accuracyValidatorDescription,
			stage:       StageAccuracy,
			scope:       ScopeBoth, // Accuracy validation works on sections and full document
			instruction: accuracyValidatorInstruction,
		},
	}
}

// SupportsStaticValidation returns true - this validator has static checks
func (v *AccuracyValidator) SupportsStaticValidation() bool {
	return true
}

// StaticValidate performs static accuracy validation using package context
func (v *AccuracyValidator) StaticValidate(ctx context.Context, content string, pkgCtx *PackageContext) (*StagedValidationResult, error) {
	result := &StagedValidationResult{
		Stage: StageAccuracy,
		Valid: true,
	}

	if pkgCtx == nil || pkgCtx.Manifest == nil {
		// Cannot do static validation without package context
		return result, nil
	}

	// Check 1: Package name mentioned correctly
	result.Issues = append(result.Issues, v.checkPackageNameAccuracy(content, pkgCtx)...)

	// Note: Data stream documentation check removed - now handled by CompletenessValidator
	// to avoid duplicate validation and conflicting severity levels.

	// Check 2: Field references exist
	result.Issues = append(result.Issues, v.checkFieldReferences(content, pkgCtx)...)

	// Check 3: Version consistency
	result.Issues = append(result.Issues, v.checkVersionAccuracy(content, pkgCtx)...)

	// Determine validity based on issues
	for _, issue := range result.Issues {
		if issue.Severity == SeverityCritical || issue.Severity == SeverityMajor {
			result.Valid = false
			break
		}
	}

	return result, nil
}

// checkPackageNameAccuracy verifies package name/title are correct
func (v *AccuracyValidator) checkPackageNameAccuracy(content string, pkgCtx *PackageContext) []ValidationIssue {
	var issues []ValidationIssue

	manifest := pkgCtx.Manifest

	// Check if package title is mentioned
	if manifest.Title != "" && !strings.Contains(content, manifest.Title) {
		issues = append(issues, ValidationIssue{
			Severity:    SeverityMajor,
			Category:    CategoryAccuracy,
			Location:    "Title/Overview",
			Message:     fmt.Sprintf("Package title '%s' not found in documentation", manifest.Title),
			Suggestion:  fmt.Sprintf("Ensure the package title '%s' is mentioned in the Overview section", manifest.Title),
			SourceCheck: "static",
		})
	}

	return issues
}

// checkFieldReferences verifies mentioned fields exist in fields.yml
func (v *AccuracyValidator) checkFieldReferences(content string, pkgCtx *PackageContext) []ValidationIssue {
	var issues []ValidationIssue

	// Get all field names from package
	allFields := pkgCtx.GetAllFieldNames()
	fieldSet := make(map[string]bool)
	for _, f := range allFields {
		fieldSet[f] = true
	}

	// If no fields loaded, skip this check
	if len(fieldSet) == 0 {
		return issues
	}

	// Extract field-like references from content
	// Look for backtick-wrapped field names or table references
	fieldPattern := regexp.MustCompile("`([a-z][a-z0-9_\\.]+)`")
	matches := fieldPattern.FindAllStringSubmatch(content, -1)

	// Track seen fields to avoid duplicate issues
	seen := make(map[string]bool)

	for _, match := range matches {
		if len(match) > 1 {
			fieldRef := match[1]
			// Skip if already seen
			if seen[fieldRef] {
				continue
			}
			seen[fieldRef] = true

			// Skip common non-field patterns
			if isCommonNonField(fieldRef) {
				continue
			}

			// Check if this looks like a field reference (contains dots)
			if strings.Contains(fieldRef, ".") && !fieldSet[fieldRef] {
				// Only flag if it looks like an ECS-style field
				if isLikelyFieldReference(fieldRef) {
					issues = append(issues, ValidationIssue{
						Severity:    SeverityMinor,
						Category:    CategoryAccuracy,
						Location:    "Field references",
						Message:     fmt.Sprintf("Field '%s' referenced but not found in fields.yml", fieldRef),
						Suggestion:  "Verify the field name is correct or is an ECS field",
						SourceCheck: "static",
					})
				}
			}
		}
	}

	return issues
}

// checkVersionAccuracy verifies package version numbers match manifest
// This only checks for explicit mentions of the PACKAGE version, not Elastic Stack
// requirements, software versions, or other version numbers
func (v *AccuracyValidator) checkVersionAccuracy(content string, pkgCtx *PackageContext) []ValidationIssue {
	var issues []ValidationIssue

	manifest := pkgCtx.Manifest
	if manifest.Version == "" {
		return issues
	}

	contentLower := strings.ToLower(content)
	packageName := strings.ToLower(manifest.Name)
	packageTitle := strings.ToLower(manifest.Title)

	// Only check for patterns that explicitly reference the package version:
	// - "package version X.Y.Z"
	// - "integration version X.Y.Z"
	// - "{package name} version X.Y.Z"
	// - "version: X.Y.Z" (only in context of the package)
	packageVersionPatterns := []string{
		`(?:package|integration)\s+version\s*[:\s]+["']?(\d+\.\d+\.\d+)["']?`,
		packageName + `\s+version\s*[:\s]+["']?(\d+\.\d+\.\d+)["']?`,
	}
	if packageTitle != packageName {
		packageVersionPatterns = append(packageVersionPatterns,
			packageTitle+`\s+version\s*[:\s]+["']?(\d+\.\d+\.\d+)["']?`)
	}

	for _, pattern := range packageVersionPatterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindAllStringSubmatch(contentLower, -1)

		for _, match := range matches {
			if len(match) > 1 {
				mentionedVersion := match[1]
				if mentionedVersion != manifest.Version {
					issues = append(issues, ValidationIssue{
						Severity:    SeverityMinor,
						Category:    CategoryAccuracy,
						Location:    "Version",
						Message:     fmt.Sprintf("Package version '%s' mentioned doesn't match manifest version '%s'", mentionedVersion, manifest.Version),
						Suggestion:  fmt.Sprintf("Update package version reference to match manifest: %s", manifest.Version),
						SourceCheck: "static",
					})
				}
			}
		}
	}

	// Note: We intentionally DO NOT flag:
	// - Elastic Stack version requirements (e.g., "requires Elastic 8.7.0+")
	// - Software/product versions (e.g., "Product 12.0")
	// - API versions
	// These are valid version numbers that should not match the package version

	return issues
}

// isCommonNonField returns true if the string is likely not a field name
func isCommonNonField(s string) bool {
	// File extensions - these are file paths, not field references
	fileExtensions := []string{
		".yml", ".yaml", ".json", ".log", ".conf", ".txt", ".md",
		".elg", ".csv", ".xml", ".html", ".py", ".sh", ".go",
	}
	for _, ext := range fileExtensions {
		if strings.HasSuffix(s, ext) {
			return true
		}
	}

	// Common file/path patterns
	commonPatterns := []string{
		"manifest.yml",
		"fields.yml",
		"README.md",
		"data_stream",
		"_dev",
		"_meta",
	}
	for _, pattern := range commonPatterns {
		if strings.Contains(s, pattern) {
			return true
		}
	}

	// Skip file paths
	if strings.Contains(s, "/") {
		return true
	}

	// Skip domain names (contain .com, .org, .io, .net, etc.)
	domainPattern := regexp.MustCompile(`\.(com|org|io|net|edu|gov|co|us|uk|de|fr|ca|au)$`)
	if domainPattern.MatchString(s) {
		return true
	}

	// Skip version strings (e.g., v13.0, v13.1, v14.1)
	versionPattern := regexp.MustCompile(`^v\d+\.\d+(\.\d+)?$`)
	if versionPattern.MatchString(s) {
		return true
	}

	// Skip plain version numbers (e.g., 8.12.0, 13.0, 14.1)
	plainVersionPattern := regexp.MustCompile(`^\d+\.\d+(\.\d+)?$`)
	return plainVersionPattern.MatchString(s)
}

// isLikelyFieldReference returns true if string looks like a field reference
// that is NOT an ECS field (ECS fields are allowed without being in fields.yml)
func isLikelyFieldReference(s string) bool {
	// ECS field prefixes - these are standard and should NOT be flagged as missing
	ecsFieldPrefixes := []string{
		"event.", "host.", "agent.", "ecs.", "message.", "log.", "error.",
		"source.", "destination.", "network.", "process.", "file.", "user.",
		"url.", "http.", "dns.", "tls.", "service.", "cloud.", "container.",
		"client.", "server.", "observer.", "geo.", "organization.", "as.",
		"threat.", "vulnerability.", "registry.", "rule.", "package.", "data_stream.",
		"@timestamp", "tags", "labels.",
	}

	// If it's an ECS field, it's valid - don't flag it
	for _, prefix := range ecsFieldPrefixes {
		if strings.HasPrefix(s, prefix) {
			return false // ECS field - don't flag as issue
		}
	}

	// Only flag non-ECS fields that look like package-specific fields
	// These typically have prefixes like the package name
	return strings.Contains(s, ".")
}
