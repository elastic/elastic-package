// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package docs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderSloTemplates(t *testing.T) {
	cases := []struct {
		name         string
		setupFunc    func(t *testing.T) string
		expectError  bool
		expectEmpty  bool
		validateFunc func(t *testing.T, result string)
	}{
		{
			name: "no templates directory",
			setupFunc: func(t *testing.T) string {
				tmpDir := t.TempDir()
				return tmpDir
			},
			expectError: false,
			expectEmpty: true,
		},
		{
			name: "empty templates directory",
			setupFunc: func(t *testing.T) string {
				tmpDir := t.TempDir()
				templatesDir := filepath.Join(tmpDir, "kibana", "slo_template")
				require.NoError(t, os.MkdirAll(templatesDir, 0o755))
				return tmpDir
			},
			expectError: false,
			expectEmpty: true,
		},
		{
			name: "single valid template",
			setupFunc: func(t *testing.T) string {
				tmpDir := t.TempDir()
				templatesDir := filepath.Join(tmpDir, "kibana", "slo_template")
				require.NoError(t, os.MkdirAll(templatesDir, 0o755))

				template := `{
					"attributes": {
						"name": "Test SLO Template",
						"description": "This is a test SLO template description"
					}
				}`
				templateFile := filepath.Join(templatesDir, "test_slo.json")
				require.NoError(t, os.WriteFile(templateFile, []byte(template), 0o644))
				return tmpDir
			},
			expectError: false,
			expectEmpty: false,
			validateFunc: func(t *testing.T, result string) {
				assert.Contains(t, result, "SLO templates provide pre-defined configurations")
				assert.Contains(t, result, "| Name | Description |")
				assert.Contains(t, result, "Test SLO Template")
				assert.Contains(t, result, "This is a test SLO template description")
			},
		},
		{
			name: "multiple valid templates",
			setupFunc: func(t *testing.T) string {
				tmpDir := t.TempDir()
				templatesDir := filepath.Join(tmpDir, "kibana", "slo_template")
				require.NoError(t, os.MkdirAll(templatesDir, 0o755))

				template1 := `{
					"attributes": {
						"name": "First SLO",
						"description": "First description"
					}
				}`
				template2 := `{
					"attributes": {
						"name": "Second SLO",
						"description": "Second description"
					}
				}`
				require.NoError(t, os.WriteFile(filepath.Join(templatesDir, "slo1.json"), []byte(template1), 0o644))
				require.NoError(t, os.WriteFile(filepath.Join(templatesDir, "slo2.json"), []byte(template2), 0o644))
				return tmpDir
			},
			expectError: false,
			expectEmpty: false,
			validateFunc: func(t *testing.T, result string) {
				assert.Contains(t, result, "First SLO")
				assert.Contains(t, result, "First description")
				assert.Contains(t, result, "Second SLO")
				assert.Contains(t, result, "Second description")
			},
		},
		{
			name: "skip non-json files",
			setupFunc: func(t *testing.T) string {
				tmpDir := t.TempDir()
				templatesDir := filepath.Join(tmpDir, "kibana", "slo_template")
				require.NoError(t, os.MkdirAll(templatesDir, 0o755))

				template := `{
					"attributes": {
						"name": "Valid SLO",
						"description": "Valid description"
					}
				}`
				require.NoError(t, os.WriteFile(filepath.Join(templatesDir, "valid.json"), []byte(template), 0o644))
				require.NoError(t, os.WriteFile(filepath.Join(templatesDir, "ignore.txt"), []byte("ignored"), 0o644))
				require.NoError(t, os.WriteFile(filepath.Join(templatesDir, "README.md"), []byte("# readme"), 0o644))
				return tmpDir
			},
			expectError: false,
			expectEmpty: false,
			validateFunc: func(t *testing.T, result string) {
				assert.Contains(t, result, "Valid SLO")
				assert.NotContains(t, result, "ignored")
				assert.NotContains(t, result, "readme")
			},
		},
		{
			name: "skip subdirectories",
			setupFunc: func(t *testing.T) string {
				tmpDir := t.TempDir()
				templatesDir := filepath.Join(tmpDir, "kibana", "slo_template")
				require.NoError(t, os.MkdirAll(templatesDir, 0o755))

				template := `{
					"attributes": {
						"name": "Root SLO",
						"description": "Root description"
					}
				}`
				require.NoError(t, os.WriteFile(filepath.Join(templatesDir, "root.json"), []byte(template), 0o644))

				// Create subdirectory with a file that should be skipped
				subDir := filepath.Join(templatesDir, "subdir")
				require.NoError(t, os.MkdirAll(subDir, 0o755))
				require.NoError(t, os.WriteFile(filepath.Join(subDir, "nested.json"), []byte(template), 0o644))
				return tmpDir
			},
			expectError: false,
			expectEmpty: false,
			validateFunc: func(t *testing.T, result string) {
				// Should only have one SLO mentioned
				assert.Equal(t, 1, strings.Count(result, "Root SLO"))
			},
		},
		{
			name: "unreadable file",
			setupFunc: func(t *testing.T) string {
				tmpDir := t.TempDir()
				templatesDir := filepath.Join(tmpDir, "kibana", "slo_template")
				require.NoError(t, os.MkdirAll(templatesDir, 0o755))

				unreadableFile := filepath.Join(templatesDir, "unreadable.json")
				require.NoError(t, os.WriteFile(unreadableFile, []byte("content"), 0o000))
				return tmpDir
			},
			expectError: true,
			expectEmpty: false,
		},
		{
			name: "invalid json file",
			setupFunc: func(t *testing.T) string {
				tmpDir := t.TempDir()
				templatesDir := filepath.Join(tmpDir, "kibana", "slo_template")
				require.NoError(t, os.MkdirAll(templatesDir, 0o755))

				invalidJSON := `{ "attributes": { "name": "Invalid" }`
				require.NoError(t, os.WriteFile(filepath.Join(templatesDir, "invalid.json"), []byte(invalidJSON), 0o644))
				return tmpDir
			},
			expectError: true,
			expectEmpty: false,
		},
		{
			name: "table structure is correct",
			setupFunc: func(t *testing.T) string {
				tmpDir := t.TempDir()
				templatesDir := filepath.Join(tmpDir, "kibana", "slo_template")
				require.NoError(t, os.MkdirAll(templatesDir, 0o755))

				template1 := `{
					"attributes": {
						"name": "First SLO Name",
						"description": "First SLO Description"
					}
				}`
				template2 := `{
					"attributes": {
						"name": "Second SLO Name",
						"description": "Second SLO Description"
					}
				}`
				require.NoError(t, os.WriteFile(filepath.Join(templatesDir, "slo1.json"), []byte(template1), 0o644))
				require.NoError(t, os.WriteFile(filepath.Join(templatesDir, "slo2.json"), []byte(template2), 0o644))
				return tmpDir
			},
			expectError: false,
			expectEmpty: false,
			validateFunc: func(t *testing.T, result string) {
				// Verify table header
				assert.Contains(t, result, "| Name | Description |")
				// Verify table separator
				assert.Contains(t, result, "|---|---|")
				// Verify table rows format
				assert.Contains(t, result, "| First SLO Name | First SLO Description |")
				assert.Contains(t, result, "| Second SLO Name | Second SLO Description |")
			},
		},
		{
			name: "special characters are escaped",
			setupFunc: func(t *testing.T) string {
				tmpDir := t.TempDir()
				templatesDir := filepath.Join(tmpDir, "kibana", "slo_template")
				require.NoError(t, os.MkdirAll(templatesDir, 0o755))

				template := `{
					"attributes": {
						"name": "SLO with *bold* and {braces}",
						"description": "Description with <angle> and {curly} brackets"
					}
				}`
				require.NoError(t, os.WriteFile(filepath.Join(templatesDir, "special.json"), []byte(template), 0o644))
				return tmpDir
			},
			expectError: false,
			expectEmpty: false,
			validateFunc: func(t *testing.T, result string) {
				// Verify special characters are escaped
				assert.Contains(t, result, `\*bold\*`)
				assert.Contains(t, result, `\{braces\}`)
				assert.Contains(t, result, `\<angle\>`)
				assert.Contains(t, result, `\{curly\}`)
				// Verify unescaped versions are not present
				assert.NotContains(t, result, "*bold*")
				assert.NotContains(t, result, "{braces}")
				assert.NotContains(t, result, "<angle>")
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			packageRoot := tc.setupFunc(t)

			result, err := renderSloTemplates(packageRoot, newEmptyLinkMap())

			if tc.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)

			if tc.expectEmpty {
				assert.Empty(t, result)
			} else {
				assert.NotEmpty(t, result)
				if tc.validateFunc != nil {
					tc.validateFunc(t, result)
				}
			}
		})
	}
}
