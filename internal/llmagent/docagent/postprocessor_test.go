// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package docagent

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHasFieldsTemplate(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		dsName   string
		expected bool
	}{
		{
			name:     "exact match no spaces",
			content:  `{{fields "audit"}}`,
			dsName:   "audit",
			expected: true,
		},
		{
			name:     "match with spaces",
			content:  `{{ fields "audit" }}`,
			dsName:   "audit",
			expected: true,
		},
		{
			name:     "match with mixed spaces",
			content:  `{{  fields  "audit"  }}`,
			dsName:   "audit",
			expected: true,
		},
		{
			name:     "no match wrong name",
			content:  `{{fields "log"}}`,
			dsName:   "audit",
			expected: false,
		},
		{
			name:     "no match no template",
			content:  `Some random content`,
			dsName:   "audit",
			expected: false,
		},
		{
			name:     "match in larger document",
			content:  "## Reference\n\n### audit\n\n{{fields \"audit\"}}\n\n### log\n\n{{fields \"log\"}}",
			dsName:   "audit",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasFieldsTemplate(tt.content, tt.dsName)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestHasEventTemplate(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		dsName   string
		expected bool
	}{
		{
			name:     "exact match no spaces",
			content:  `{{event "audit"}}`,
			dsName:   "audit",
			expected: true,
		},
		{
			name:     "match with spaces",
			content:  `{{ event "audit" }}`,
			dsName:   "audit",
			expected: true,
		},
		{
			name:     "no match wrong name",
			content:  `{{event "log"}}`,
			dsName:   "audit",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasEventTemplate(tt.content, tt.dsName)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestHasDataStreamSubsection(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		dsName   string
		expected bool
	}{
		{
			name:     "exact match",
			content:  "## Reference\n\n### audit\n\nSome content",
			dsName:   "audit",
			expected: true,
		},
		{
			name:     "no match",
			content:  "## Reference\n\n### log\n\nSome content",
			dsName:   "audit",
			expected: false,
		},
		{
			name:     "case insensitive match",
			content:  "## Reference\n\n### Audit\n\nSome content",
			dsName:   "audit",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasDataStreamSubsection(tt.content, tt.dsName)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestInsertFieldsTemplate(t *testing.T) {
	tests := []struct {
		name           string
		content        string
		dsName         string
		expectContains string
	}{
		{
			name: "insert into existing subsection",
			content: `## Reference

### audit

The audit data stream.

### log

The log data stream.
`,
			dsName:         "audit",
			expectContains: `{{fields "audit"}}`,
		},
		{
			name: "create new subsection when missing",
			content: `## Reference

### log

The log data stream.
`,
			dsName:         "audit",
			expectContains: `### audit`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := insertFieldsTemplate(tt.content, tt.dsName)
			assert.Contains(t, result, tt.expectContains)
			// Verify the template is now present
			assert.True(t, hasFieldsTemplate(result, tt.dsName))
		})
	}
}

func TestInsertEventTemplate(t *testing.T) {
	tests := []struct {
		name           string
		content        string
		dsName         string
		expectContains string
	}{
		{
			name: "insert before existing fields template",
			content: `## Reference

### audit

The audit data stream.

{{fields "audit"}}
`,
			dsName:         "audit",
			expectContains: `{{event "audit"}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := insertEventTemplate(tt.content, tt.dsName)
			assert.Contains(t, result, tt.expectContains)
			// Verify event comes before fields
			eventIdx := strings.Index(result, `{{event "audit"}}`)
			fieldsIdx := strings.Index(result, `{{fields "audit"}}`)
			if eventIdx >= 0 && fieldsIdx >= 0 {
				assert.Less(t, eventIdx, fieldsIdx, "event template should come before fields template")
			}
		})
	}
}

func TestAppendDataStreamSubsection(t *testing.T) {
	content := `## Reference

### log

{{fields "log"}}

## Troubleshooting
`

	result := appendDataStreamSubsection(content, "audit", true, true)

	// Should contain the new subsection
	assert.Contains(t, result, "### audit")
	assert.Contains(t, result, `{{event "audit"}}`)
	assert.Contains(t, result, `{{fields "audit"}}`)

	// Should still contain the original content
	assert.Contains(t, result, "### log")
	assert.Contains(t, result, "## Troubleshooting")

	// The new subsection should be in the Reference section (before Troubleshooting)
	auditIdx := strings.Index(result, "### audit")
	troubleshootingIdx := strings.Index(result, "## Troubleshooting")
	assert.Less(t, auditIdx, troubleshootingIdx, "audit subsection should be before Troubleshooting")
}
