// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package tools

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidatePathInRoot(t *testing.T) {
	tmpDir := t.TempDir()

	// Create some test directories so EvalSymlinks works consistently
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "data_stream", "logs"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "foo"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "bar"), 0o755))

	tests := []struct {
		name        string
		userPath    string
		shouldError bool
	}{
		{
			name:        "valid relative path",
			userPath:    "data_stream/logs",
			shouldError: false,
		},
		{
			name:        "root path",
			userPath:    "",
			shouldError: false,
		},
		{
			name:        "path traversal attack",
			userPath:    "../../../etc/passwd",
			shouldError: true,
		},
		{
			name:        "path with dot dot escape",
			userPath:    "foo/../../../etc/passwd",
			shouldError: true,
		},
		{
			name:        "valid path with dot dot inside",
			userPath:    "foo/../bar",
			shouldError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := validatePathInRoot(tmpDir, tt.userPath)
			if tt.shouldError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, result)
			}
		})
	}
}

func TestPackageTools(t *testing.T) {
	tmpDir := t.TempDir()
	tools := PackageTools(tmpDir, nil) // nil service info provider for basic tool list test

	// Verify all expected tools are present
	expectedTools := []string{
		"list_directory",
		"read_file",
		"write_file",
		"get_readme_template",
		"list_examples",
		"get_example",
		"get_service_info",
	}

	assert.Len(t, tools, len(expectedTools))

	for i, name := range expectedTools {
		assert.Equal(t, name, tools[i].Name())
		assert.NotEmpty(t, tools[i].Description())
	}
}

func TestGetServiceInfoMappingForSection(t *testing.T) {
	tests := []struct {
		name           string
		sectionTitle   string
		expectedLength int
		shouldContain  []string
		shouldBeEmpty  bool
	}{
		{
			name:           "Overview section",
			sectionTitle:   "Overview",
			expectedLength: 3,
			shouldContain:  []string{"Common use cases", "Data types collected"},
		},
		{
			name:           "Troubleshooting section",
			sectionTitle:   "Troubleshooting",
			expectedLength: 1,
			shouldContain:  []string{"Troubleshooting"},
		},
		{
			name:           "Case insensitive",
			sectionTitle:   "OVERVIEW",
			expectedLength: 3,
		},
		{
			name:          "Unknown section",
			sectionTitle:  "Unknown Section",
			shouldBeEmpty: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetServiceInfoMappingForSection(tt.sectionTitle)

			if tt.shouldBeEmpty {
				assert.Empty(t, result)
			} else {
				assert.Len(t, result, tt.expectedLength)
				for _, expected := range tt.shouldContain {
					assert.Contains(t, result, expected)
				}
			}
		})
	}
}
