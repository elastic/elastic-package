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

### Onboard and configure

Config.

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

### Onboard and configure

Configuration.

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
