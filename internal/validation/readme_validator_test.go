// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package validation

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateContent(t *testing.T) {
	tests := []struct {
		name             string
		filename         string
		content          []byte
		enforcedSections []string
		expectedResult   error
	}{
		{
			name:             "Valid content",
			filename:         "test.md",
			content:          []byte("# Overview\n\nThis is a valid overview section."),
			enforcedSections: []string{"Overview"},
			expectedResult:   nil,
		},
		{
			name:             "Missing header",
			filename:         "test.md",
			content:          []byte("# Overview\n\nThis is a valid overview section."),
			enforcedSections: []string{"Overview", "Setup"},
			expectedResult:   DocsValidationError{fmt.Errorf("missing required section 'Setup' in file 'test.md'")},
		}, {
			name:             "Empty content",
			filename:         "test.md",
			content:          []byte(""),
			enforcedSections: []string{"Overview", "Setup"},
			expectedResult:   DocsValidationError{fmt.Errorf("missing required section 'Overview' in file 'test.md'"), fmt.Errorf("missing required section 'Setup' in file 'test.md'")},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actualResult := validateContent(tt.filename, tt.content, tt.enforcedSections)
			assert.Equal(t, tt.expectedResult, actualResult, "Result does not match expected")
		})
	}
}

func TestValidateReadmeStructure(t *testing.T) {
	tests := []struct {
		name             string
		packageRoot      string
		enforcedSections []string
		expectedResult   string
	}{
		{
			name:             "Valid test",
			packageRoot:      "testdata",
			enforcedSections: []string{"Overview", "Setup"},
			expectedResult:   "\n\nmissing required section 'Setup' in file 'missing_headers.md'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actualResult := ValidateReadmeStructure(tt.packageRoot, tt.enforcedSections)

			assert.Equal(t, tt.expectedResult, actualResult.Error(), "Result does not match expected")
		})
	}
}
