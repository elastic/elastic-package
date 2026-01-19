// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

// Package generation provides documentation generation orchestration for LLM-based workflows.
package generation

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/elastic/elastic-package/internal/llmagent/docagent/parsing"
	"github.com/elastic/elastic-package/internal/llmagent/docagent/specialists/validators"
)

// StructuralIssue represents a structural problem in the document
type StructuralIssue struct {
	Type       string // "title", "order", "missing", "duplicate"
	Location   string // Where the issue occurs
	Message    string // Description of the issue
	Suggestion string // How to fix it
}

// RequiredSections is the list of required H2 sections in order
var RequiredSections = []string{
	"overview",
	"what data does this integration collect?",
	"what do i need to use this integration?",
	"how do i deploy this integration?",
	"troubleshooting",
	"performance and scaling",
	"reference",
}

// ValidateDocumentStructure checks the document for structural issues.
// It validates title format, AI notice, section presence, duplicates, and order.
func ValidateDocumentStructure(content string, packageTitle string, pkgCtx *validators.PackageContext) []StructuralIssue {
	var issues []StructuralIssue

	if pkgCtx != nil && pkgCtx.Manifest != nil {
		packageTitle = pkgCtx.Manifest.Title
	}

	lines := strings.Split(content, "\n")

	// Check title format
	expectedTitle := fmt.Sprintf("# %s Integration for Elastic", packageTitle)
	if len(lines) == 0 || !strings.HasPrefix(lines[0], "# ") {
		issues = append(issues, StructuralIssue{
			Type:       "title",
			Location:   "Document start",
			Message:    "Document does not start with H1 title",
			Suggestion: fmt.Sprintf("Add title: %s", expectedTitle),
		})
	} else if strings.TrimSpace(lines[0]) != expectedTitle {
		issues = append(issues, StructuralIssue{
			Type:       "title",
			Location:   "Document title",
			Message:    fmt.Sprintf("Title format incorrect: got '%s'", strings.TrimSpace(lines[0])),
			Suggestion: fmt.Sprintf("Use: %s", expectedTitle),
		})
	}

	// Check for AI notice
	aiNotice := "> **Note**: This documentation was generated using AI and should be reviewed for accuracy."
	hasAINotice := false
	for i := 1; i < min(5, len(lines)); i++ {
		if strings.TrimSpace(lines[i]) == aiNotice {
			hasAINotice = true
			break
		}
	}
	if !hasAINotice {
		issues = append(issues, StructuralIssue{
			Type:       "title",
			Location:   "After title",
			Message:    "Missing AI-generated notice",
			Suggestion: "Add notice after title: " + aiNotice,
		})
	}

	// Parse H2 sections and check order
	h2Pattern := regexp.MustCompile(`(?m)^##\s+(.+)$`)
	h2Matches := h2Pattern.FindAllStringSubmatch(content, -1)

	foundSections := make(map[string]int) // section name -> count
	var sectionOrder []string

	for _, match := range h2Matches {
		if len(match) > 1 {
			sectionName := strings.ToLower(strings.TrimSpace(match[1]))
			foundSections[sectionName]++
			if foundSections[sectionName] == 1 {
				sectionOrder = append(sectionOrder, sectionName)
			}
		}
	}

	// Check for missing sections
	for _, required := range RequiredSections {
		if foundSections[required] == 0 {
			issues = append(issues, StructuralIssue{
				Type:       "missing",
				Location:   fmt.Sprintf("## %s", toTitle(required)),
				Message:    fmt.Sprintf("Required section '## %s' is missing", toTitle(required)),
				Suggestion: "Add the missing section with appropriate content",
			})
		}
	}

	// Check for duplicate sections
	for section, count := range foundSections {
		if count > 1 {
			issues = append(issues, StructuralIssue{
				Type:       "duplicate",
				Location:   fmt.Sprintf("## %s", section),
				Message:    fmt.Sprintf("Section '## %s' appears %d times", section, count),
				Suggestion: "Remove duplicate sections, keeping the first occurrence",
			})
		}
	}

	// Check section order
	expectedOrder := []string{}
	for _, req := range RequiredSections {
		for _, found := range sectionOrder {
			if found == req {
				expectedOrder = append(expectedOrder, req)
				break
			}
		}
	}

	// Compare actual order with expected
	actualIdx := 0
	for _, expected := range expectedOrder {
		found := false
		for i := actualIdx; i < len(sectionOrder); i++ {
			if sectionOrder[i] == expected {
				actualIdx = i + 1
				found = true
				break
			}
		}
		if !found {
			// Section exists but is out of order
			for _, s := range sectionOrder {
				if s == expected {
					issues = append(issues, StructuralIssue{
						Type:       "order",
						Location:   fmt.Sprintf("## %s", expected),
						Message:    fmt.Sprintf("Section '## %s' is out of order", expected),
						Suggestion: "Reorder sections to match the required structure",
					})
					break
				}
			}
		}
	}

	return issues
}

// FormatStructuralIssues converts issues to a readable string
func FormatStructuralIssues(issues []StructuralIssue) string {
	if len(issues) == 0 {
		return "No structural issues found."
	}

	var sb strings.Builder
	for i, issue := range issues {
		sb.WriteString(fmt.Sprintf("%d. [%s] %s\n", i+1, issue.Type, issue.Message))
		if issue.Suggestion != "" {
			sb.WriteString(fmt.Sprintf("   Suggestion: %s\n", issue.Suggestion))
		}
	}
	return sb.String()
}

// EnsureDocumentStructure ensures the document has a valid structure.
// This fixes title format programmatically and returns remaining issues.
func EnsureDocumentStructure(content, packageTitle string, pkgCtx *validators.PackageContext) (string, []StructuralIssue) {
	if pkgCtx != nil && pkgCtx.Manifest != nil {
		packageTitle = pkgCtx.Manifest.Title
	}

	// First, ensure the title is correct
	content = parsing.EnsureDocumentTitle(content, packageTitle)

	// Validate structure
	issues := ValidateDocumentStructure(content, packageTitle, pkgCtx)

	// Filter out title issues since we already fixed them
	var remainingIssues []StructuralIssue
	for _, issue := range issues {
		if issue.Type != "title" {
			remainingIssues = append(remainingIssues, issue)
		}
	}

	return content, remainingIssues
}

// toTitle capitalizes the first letter of each word (simple title case)
func toTitle(s string) string {
	words := strings.Fields(s)
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}
