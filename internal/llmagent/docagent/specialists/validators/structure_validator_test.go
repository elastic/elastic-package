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

	if misplacedCount != 2 {
		t.Errorf("Expected 2 misplaced section issues, got %d", misplacedCount)
		for _, issue := range result.Issues {
			t.Logf("Issue: [%s] %s - %s", issue.Severity, issue.Message, issue.Location)
		}
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
	issues := v.checkSectionOrder(content)

	// Should catch that Overview is out of order
	orderIssuesCount := 0
	for _, issue := range issues {
		if issue.Category == CategoryStructure && issue.Severity == SeverityMajor {
			orderIssuesCount++
		}
	}

	if orderIssuesCount == 0 {
		t.Error("Expected section order issues, got none")
	}
}

func TestValidDocumentPasses(t *testing.T) {
	// A properly structured document should pass
	content := `# Test Integration

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

