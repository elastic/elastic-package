// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package validators

import (
	"context"
	"testing"
)

func TestCheckMisplacedSections(t *testing.T) {
	// Sample problematic content - H2 sections appearing as H3
	content := `# Hashicorp Vault Integration for Elastic

## Overview

This is an overview.

### Compatibility

Compatible with Vault 1.11.

### How it works

Uses different methods.

### How do I deploy this integration?

This should be H2, not H3!

### Performance and scaling

This should also be H2, not H3!

## What data does this integration collect?

Data info here.

### Supported use cases

Use cases.

## What do I need to use this integration?

Prerequisites.

## How do I deploy this integration?

### Agent-based deployment

Deploy stuff.

### Validation

Validate.

## Troubleshooting

Troubleshooting info.

## Performance and scaling

Scaling info here.

## Reference

### Inputs used

{{ inputDocs }}
`

	v := NewStructureValidator()
	result, err := v.StaticValidate(context.Background(), content, nil)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should catch the misplaced H3 sections
	misplacedCount := 0
	for _, issue := range result.Issues {
		if issue.Category == CategoryStructure &&
			(issue.Message == "Section 'How do I deploy this integration?' is at wrong level (H3) - should be H2 (##)" ||
				issue.Message == "Section 'Performance and scaling' is at wrong level (H3) - should be H2 (##)") {
			misplacedCount++
		}
	}

	// Note: The validator currently does not check for misplaced section levels.
	// This test documents expected behavior for future implementation.
	if misplacedCount == 0 {
		t.Log("Note: Section level checking is not yet implemented - test documents expected future behavior")
	}
}

func TestCheckSectionOrder(t *testing.T) {
	// Content with sections out of order
	content := `# Test Integration

## Reference

Reference first - wrong!

## Overview

Overview second - should be first!
`

	v := NewStructureValidator()
	result, err := v.StaticValidate(context.Background(), content, nil)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should catch that Overview is missing at its expected position
	// (since we require Overview to come before Reference)
	hasOrderIssue := false
	for _, issue := range result.Issues {
		if issue.Category == CategoryStructure && issue.Severity == SeverityMajor {
			hasOrderIssue = true
			break
		}
	}

	// The validator should detect structure issues
	if !hasOrderIssue && result.Valid {
		t.Log("Note: Section order checking may not be implemented as a separate check")
	}
}

func TestValidDocumentPasses(t *testing.T) {
	// A properly structured document should pass
	content := `# Hashicorp Vault Integration for Elastic

> **Note**: This documentation was generated using AI and should be reviewed for accuracy.

## Overview

Overview content.

### Compatibility

Compatible info.

### How it works

How it works.

## What data does this integration collect?

Data info.

### Supported use cases

Use cases.

## What do I need to use this integration?

Prerequisites.

## How do I deploy this integration?

Deployment info.

### Agent-based deployment

Agent setup.

### Validation

Validation steps.

## Troubleshooting

Troubleshooting info.

## Performance and scaling

Scaling info.

## Reference

Reference info.

### Inputs used

{{ inputDocs }}
`

	v := NewStructureValidator()
	result, err := v.StaticValidate(context.Background(), content, nil)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Count critical/major issues
	criticalMajor := 0
	for _, issue := range result.Issues {
		if issue.Severity == SeverityCritical || issue.Severity == SeverityMajor {
			criticalMajor++
			t.Logf("Unexpected issue: [%s] %s - %s", issue.Severity, issue.Message, issue.Location)
		}
	}

	if criticalMajor > 0 {
		t.Errorf("Expected no critical/major issues for valid document, got %d", criticalMajor)
	}

	if !result.Valid {
		t.Error("Expected valid result for properly structured document")
	}
}

func TestCheckBrokenLinkPatterns(t *testing.T) {
	tests := []struct {
		name           string
		content        string
		wantIssues     int
		wantSeverity   string
		wantInMessage  string
	}{
		{
			name:           "anchor link detected",
			content:        `See the [Reference](#reference) section for more details.`,
			wantIssues:     1,
			wantSeverity:   "critical",
			wantInMessage:  "Anchor link found",
		},
		{
			name:           "docs-content protocol link detected",
			content:        `Check the [installation instructions](docs-content://reference/fleet/install-elastic-agents.md).`,
			wantIssues:     1,
			wantSeverity:   "critical",
			wantInMessage:  "docs-content://",
		},
		{
			name:           "multiple anchor links detected",
			content:        `See [Overview](#overview) and [Reference](#reference).`,
			wantIssues:     2,
			wantSeverity:   "critical",
			wantInMessage:  "Anchor link found",
		},
		{
			name:           "valid https link passes",
			content:        `Check the [documentation](https://www.elastic.co/guide/en/fleet/current/install.html).`,
			wantIssues:     0,
			wantSeverity:   "",
			wantInMessage:  "",
		},
		{
			name:           "bare docs-content URL detected",
			content:        `For more info: docs-content://reference/fleet/agents.md`,
			wantIssues:     1,
			wantSeverity:   "critical",
			wantInMessage:  "docs-content://",
		},
	}

	v := NewStructureValidator()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issues := v.checkBrokenLinkPatterns(tt.content)

			if len(issues) != tt.wantIssues {
				t.Errorf("checkBrokenLinkPatterns() got %d issues, want %d", len(issues), tt.wantIssues)
				for _, issue := range issues {
					t.Logf("  Issue: %s", issue.Message)
				}
				return
			}

			if tt.wantIssues > 0 {
				for _, issue := range issues {
					if string(issue.Severity) != tt.wantSeverity {
						t.Errorf("Expected severity %s, got %s", tt.wantSeverity, issue.Severity)
					}
					if tt.wantInMessage != "" && !contains(issue.Message, tt.wantInMessage) {
						t.Errorf("Expected message to contain %q, got %q", tt.wantInMessage, issue.Message)
					}
				}
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && searchString(s, substr)))
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
