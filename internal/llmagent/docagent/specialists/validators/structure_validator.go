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
	structureValidatorName        = "structure_validator"
	structureValidatorDescription = "Validates README structure and format compliance"
)

// RequiredSection defines a required section with its expected subsections
type RequiredSection struct {
	Name        string
	Subsections []string // Expected subsections (empty if none required)
}

// Required top-level sections (H2) that must be present in the README
// Based on the official package-docs-readme.md.tmpl template
var requiredSections = []RequiredSection{
	{
		Name:        "Overview",
		Subsections: []string{"Compatibility", "How it works"},
	},
	{
		Name:        "What data does this integration collect?",
		Subsections: []string{"Supported use cases"},
	},
	{
		Name:        "What do I need to use this integration?",
		Subsections: nil,
	},
	{
		Name: "How do I deploy this integration?",
		Subsections: []string{
			"Agent-based deployment",
			"Onboard and configure",
			"Validation",
		},
	},
	{
		Name:        "Troubleshooting",
		Subsections: nil,
	},
	{
		Name:        "Performance and scaling",
		Subsections: nil,
	},
	{
		Name:        "Reference",
		Subsections: []string{"Inputs used"},
	},
}

// Optional but recommended sections
var recommendedSections = []string{
	"API usage", // Under Reference, for integrations using APIs
	// Note: "Agentless deployment" is NOT included here - it's only applicable
	// to integrations with agentless enabled in manifest.yml
}

// Alternative section names that are acceptable
var sectionAliases = map[string][]string{
	"overview": {"introduction", "about"},
	"what data does this integration collect?": {"data streams", "data collected", "collected data"},
	"what do i need to use this integration?":  {"prerequisites", "requirements"},
	"how do i deploy this integration?":        {"setup", "installation", "getting started", "configuration"},
	"troubleshooting":                          {"common issues", "faq"},
	"performance and scaling":                  {"scaling", "performance"},
	"reference":                                {"appendix", "field reference"},
}

const structureValidatorInstruction = `You are a documentation structure validator for Elastic integration packages.
Your task is to validate that the README follows the expected structure and format per the official template.

## Expected Section Structure (from package-docs-readme.md.tmpl)
The documentation MUST include these sections in order:

1. **## Overview** (with subsections: ### Compatibility, ### How it works)
2. **## What data does this integration collect?** (with subsection: ### Supported use cases)
3. **## What do I need to use this integration?**
4. **## How do I deploy this integration?** (with subsections: ### Agent-based deployment, ### Onboard and configure, ### Set up steps in {Product}, ### Set up steps in Kibana, ### Validation)
5. **## Troubleshooting**
6. **## Performance and scaling**
7. **## Reference** (with subsections: ### {Data stream name}, ### Inputs used, ### API usage if applicable)

## Input
The documentation content to validate is provided directly in the user message.
Static validation has already checked for required sections - focus on semantic structure and order.

## Checks
1. Section order follows the template (Overview → Data collection → Prerequisites → Deployment → Troubleshooting → Performance → Reference)
2. Heading hierarchy is correct (# for title, ## for main sections, ### for subsections)
3. Required subsections are present under their parent sections
4. Code blocks are properly formatted with language tags
5. Tables are well-formed with headers
6. Lists are consistent (bullet or numbered as appropriate)
7. No orphaned content outside sections

## Output Format
Output a JSON object with this exact structure:
{"valid": true/false, "score": 0-100, "issues": [{"severity": "critical|major|minor", "category": "structure", "location": "Section Name", "message": "Issue description", "suggestion": "How to fix"}]}

Set valid=false if any major or critical issues are found. Minor issues alone do not invalidate.

## IMPORTANT
Output ONLY the JSON object. No other text.`

// StructureValidator validates README structure and format (Section A)
type StructureValidator struct {
	BaseStagedValidator
}

// NewStructureValidator creates a new structure validator
func NewStructureValidator() *StructureValidator {
	return &StructureValidator{
		BaseStagedValidator: BaseStagedValidator{
			name:        structureValidatorName,
			description: structureValidatorDescription,
			stage:       StageStructure,
			instruction: structureValidatorInstruction,
		},
	}
}

// SupportsStaticValidation returns true - this validator has static checks
func (v *StructureValidator) SupportsStaticValidation() bool {
	return true
}

// StaticValidate performs static structure validation
func (v *StructureValidator) StaticValidate(ctx context.Context, content string, pkgCtx *PackageContext) (*StagedValidationResult, error) {
	result := &StagedValidationResult{
		Stage: StageStructure,
		Valid: true,
	}

	// Check 1: Required sections present
	result.Issues = append(result.Issues, v.checkRequiredSections(content)...)

	// Check 2: Heading hierarchy
	result.Issues = append(result.Issues, v.checkHeadingHierarchy(content)...)

	// Check 3: Empty code blocks
	result.Issues = append(result.Issues, v.checkEmptyCodeBlocks(content)...)

	// Check 4: Markdown formatting issues
	result.Issues = append(result.Issues, v.checkMarkdownFormatting(content)...)

	// Check 5: Duplicate sections
	result.Issues = append(result.Issues, v.checkDuplicateSections(content)...)

	// Determine validity based on issues
	for _, issue := range result.Issues {
		if issue.Severity == SeverityCritical || issue.Severity == SeverityMajor {
			result.Valid = false
			break
		}
	}

	return result, nil
}

// checkRequiredSections verifies all required sections and subsections exist
func (v *StructureValidator) checkRequiredSections(content string) []ValidationIssue {
	var issues []ValidationIssue

	// Extract all H2 section headings
	h2Pattern := regexp.MustCompile(`(?m)^##\s+([^#\n]+)$`)
	h2Matches := h2Pattern.FindAllStringSubmatch(content, -1)

	// Extract all H3 section headings (subsections)
	h3Pattern := regexp.MustCompile(`(?m)^###\s+([^#\n]+)$`)
	h3Matches := h3Pattern.FindAllStringSubmatch(content, -1)

	// Build maps of found sections
	foundH2Sections := make(map[string]bool)
	for _, match := range h2Matches {
		if len(match) > 1 {
			sectionName := strings.TrimSpace(match[1])
			foundH2Sections[strings.ToLower(sectionName)] = true
		}
	}

	foundH3Sections := make(map[string]bool)
	for _, match := range h3Matches {
		if len(match) > 1 {
			sectionName := strings.TrimSpace(match[1])
			foundH3Sections[strings.ToLower(sectionName)] = true
		}
	}

	// Check required H2 sections
	for _, required := range requiredSections {
		sectionFound := v.sectionExists(required.Name, foundH2Sections)

		if !sectionFound {
			issues = append(issues, ValidationIssue{
				Severity:    SeverityCritical,
				Category:    CategoryStructure,
				Location:    "Document",
				Message:     "Missing required section: " + required.Name,
				Suggestion:  "Add a '## " + required.Name + "' section to the document",
				SourceCheck: "static",
			})
		} else {
			// Check required subsections for this section
			for _, subsection := range required.Subsections {
				if !v.sectionExists(subsection, foundH3Sections) {
					issues = append(issues, ValidationIssue{
						Severity:    SeverityMajor,
						Category:    CategoryStructure,
						Location:    required.Name,
						Message:     "Missing required subsection: " + subsection,
						Suggestion:  "Add a '### " + subsection + "' subsection under '" + required.Name + "'",
						SourceCheck: "static",
					})
				}
			}
		}
	}

	// Check recommended sections (minor warnings) - these are typically H3 subsections
	for _, recommended := range recommendedSections {
		if !foundH3Sections[strings.ToLower(recommended)] && !foundH2Sections[strings.ToLower(recommended)] {
			issues = append(issues, ValidationIssue{
				Severity:    SeverityMinor,
				Category:    CategoryStructure,
				Location:    "Document",
				Message:     "Missing recommended section: " + recommended,
				Suggestion:  "Consider adding a '### " + recommended + "' section if applicable",
				SourceCheck: "static",
			})
		}
	}

	return issues
}

// sectionExists checks if a section exists, including aliases
func (v *StructureValidator) sectionExists(sectionName string, foundSections map[string]bool) bool {
	normalizedName := strings.ToLower(sectionName)

	// Check exact match
	if foundSections[normalizedName] {
		return true
	}

	// Check aliases
	if aliases, ok := sectionAliases[normalizedName]; ok {
		for _, alias := range aliases {
			if foundSections[alias] {
				return true
			}
		}
	}

	// Also check if the section name is an alias for something else
	for _, aliases := range sectionAliases {
		for _, alias := range aliases {
			if alias == normalizedName {
				return true
			}
		}
	}

	return false
}

// checkHeadingHierarchy validates heading levels are sequential
func (v *StructureValidator) checkHeadingHierarchy(content string) []ValidationIssue {
	var issues []ValidationIssue

	headingPattern := regexp.MustCompile(`(?m)^(#{1,6})\s+`)
	matches := headingPattern.FindAllStringSubmatch(content, -1)

	if len(matches) == 0 {
		issues = append(issues, ValidationIssue{
			Severity:    SeverityCritical,
			Category:    CategoryStructure,
			Location:    "Document",
			Message:     "No headings found in document",
			Suggestion:  "Add a title heading (#) and section headings (##)",
			SourceCheck: "static",
		})
		return issues
	}

	// Check first heading is H1
	if len(matches) > 0 && len(matches[0][1]) != 1 {
		issues = append(issues, ValidationIssue{
			Severity:    SeverityMajor,
			Category:    CategoryStructure,
			Location:    "Title",
			Message:     "Document should start with a single # heading (H1)",
			Suggestion:  "Change the first heading to use single #",
			SourceCheck: "static",
		})
	}

	// Check for heading level jumps (e.g., H1 -> H3)
	prevLevel := 0
	for _, match := range matches {
		level := len(match[1])
		if prevLevel > 0 && level > prevLevel+1 {
			issues = append(issues, ValidationIssue{
				Severity:    SeverityMajor,
				Category:    CategoryStructure,
				Location:    "Headings",
				Message:     "Heading level jumps from H" + string(rune('0'+prevLevel)) + " to H" + string(rune('0'+level)),
				Suggestion:  "Use sequential heading levels without skipping",
				SourceCheck: "static",
			})
		}
		prevLevel = level
	}

	return issues
}

// checkEmptyCodeBlocks finds code blocks with no content
func (v *StructureValidator) checkEmptyCodeBlocks(content string) []ValidationIssue {
	var issues []ValidationIssue

	// Match code blocks (``` followed by optional language, then ```)
	codeBlockPattern := regexp.MustCompile("(?s)```[a-z]*\\s*```")
	if codeBlockPattern.MatchString(content) {
		issues = append(issues, ValidationIssue{
			Severity:    SeverityCritical,
			Category:    CategoryStructure,
			Location:    "Code blocks",
			Message:     "Empty code block found",
			Suggestion:  "Add content to the code block or remove it",
			SourceCheck: "static",
		})
	}

	return issues
}

// checkMarkdownFormatting validates basic markdown formatting
func (v *StructureValidator) checkMarkdownFormatting(content string) []ValidationIssue {
	var issues []ValidationIssue

	// Check for code blocks without language specification
	codeBlockNoLang := regexp.MustCompile("(?m)^```$")
	matches := codeBlockNoLang.FindAllString(content, -1)
	if len(matches) > 0 {
		issues = append(issues, ValidationIssue{
			Severity:    SeverityMinor,
			Category:    CategoryStructure,
			Location:    "Code blocks",
			Message:     "Code block without language specification found",
			Suggestion:  "Add language identifier after ``` (e.g., ```yaml)",
			SourceCheck: "static",
		})
	}

	// Check for malformed links
	malformedLink := regexp.MustCompile(`\[([^\]]*)\]\s+\(`)
	if malformedLink.MatchString(content) {
		issues = append(issues, ValidationIssue{
			Severity:    SeverityMajor,
			Category:    CategoryStructure,
			Location:    "Links",
			Message:     "Malformed markdown link (space between ] and ()",
			Suggestion:  "Remove space between link text ] and URL (",
			SourceCheck: "static",
		})
	}

	return issues
}

// checkDuplicateSections detects when the same section heading appears multiple times
func (v *StructureValidator) checkDuplicateSections(content string) []ValidationIssue {
	var issues []ValidationIssue

	// Check for duplicate H1 headings (title)
	h1Pattern := regexp.MustCompile(`(?m)^#\s+([^#\n]+)$`)
	h1Matches := h1Pattern.FindAllStringSubmatch(content, -1)
	if len(h1Matches) > 1 {
		issues = append(issues, ValidationIssue{
			Severity:    SeverityCritical,
			Category:    CategoryStructure,
			Location:    "Document Title",
			Message:     fmt.Sprintf("Multiple H1 titles found (%d occurrences) - document should have exactly one title", len(h1Matches)),
			Suggestion:  "Remove duplicate titles, keeping only the first one",
			SourceCheck: "static",
		})
	}

	// Check for duplicate H2 sections
	h2Pattern := regexp.MustCompile(`(?m)^##\s+([^#\n]+)$`)
	h2Matches := h2Pattern.FindAllStringSubmatch(content, -1)

	h2Count := make(map[string]int)
	for _, match := range h2Matches {
		if len(match) > 1 {
			sectionName := strings.TrimSpace(match[1])
			h2Count[sectionName]++
		}
	}

	for section, count := range h2Count {
		if count > 1 {
			issues = append(issues, ValidationIssue{
				Severity:    SeverityCritical,
				Category:    CategoryStructure,
				Location:    fmt.Sprintf("Section: %s", section),
				Message:     fmt.Sprintf("Duplicate section '## %s' found %d times", section, count),
				Suggestion:  fmt.Sprintf("Remove duplicate '## %s' sections - each section should appear only once", section),
				SourceCheck: "static",
			})
		}
	}

	// Check for duplicate H3 subsections within the same context
	// This is a bit more complex as subsections can legitimately repeat across different H2 sections
	// But if a subsection like "### Compatibility" appears 6 times, that's likely a problem
	h3Pattern := regexp.MustCompile(`(?m)^###\s+([^#\n]+)$`)
	h3Matches := h3Pattern.FindAllStringSubmatch(content, -1)

	h3Count := make(map[string]int)
	for _, match := range h3Matches {
		if len(match) > 1 {
			subsectionName := strings.TrimSpace(match[1])
			h3Count[subsectionName]++
		}
	}

	// Only flag H3 duplicates if they appear more than 3 times (suggests parallel generation issue)
	for subsection, count := range h3Count {
		if count > 3 {
			issues = append(issues, ValidationIssue{
				Severity:    SeverityMajor,
				Category:    CategoryStructure,
				Location:    fmt.Sprintf("Subsection: %s", subsection),
				Message:     fmt.Sprintf("Subsection '### %s' appears %d times - likely duplicate content", subsection, count),
				Suggestion:  "Review and consolidate duplicate subsections",
				SourceCheck: "static",
			})
		}
	}

	return issues
}
