// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package validation

import (
	_ "embed"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateDocsStructureFromPath(t *testing.T) {
	tests := []struct {
		name           string
		rootPath       string
		expectedResult error
	}{
		{
			name:           "Missing header",
			rootPath:       "../../test/packages/other/readme_structure",
			expectedResult: DocsValidationError{fmt.Errorf("missing required section 'Overview' in file 'README.md'")},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actualResult := ValidateDocsStructureFromPath(tt.rootPath)
			assert.Equal(t, tt.expectedResult, actualResult, "Result does not match expected")
		})
	}
}

func TestLoadSectionsFromConfig(t *testing.T) {
	tests := []struct {
		name           string
		version        string
		expectedResult []string
		expectedError  error
	}{
		{
			name:           "Valid version",
			version:        "1",
			expectedResult: []string{"Overview", "What data does this integration collect?", "What do I need to use this integration?", "How do I deploy this integration?", "Troubleshooting", "Performance and scaling"},
			expectedError:  nil,
		},
		{
			name:           "Invalid version",
			version:        "999",
			expectedResult: nil,
			expectedError:  fmt.Errorf("unsupported format_version: 999"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actualResult, err := loadSectionsFromConfig(tt.version)
			assert.Equal(t, tt.expectedResult, actualResult, "Result does not match expected")
			assert.Equal(t, tt.expectedError, err, "Error does not match expected")
		})
	}
}
