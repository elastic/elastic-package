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

func TestRenderAlertRuleTemplates(t *testing.T) {
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
				templatesDir := filepath.Join(tmpDir, "kibana", "alerting_rule_template")
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
				templatesDir := filepath.Join(tmpDir, "kibana", "alerting_rule_template")
				require.NoError(t, os.MkdirAll(templatesDir, 0o755))

				template := `{
					"attributes": {
						"name": "Test Alert Rule",
						"description": "This is a test alert rule description"
					}
				}`
				templateFile := filepath.Join(templatesDir, "test_rule.json")
				require.NoError(t, os.WriteFile(templateFile, []byte(template), 0o644))
				return tmpDir
			},
			expectError: false,
			expectEmpty: false,
			validateFunc: func(t *testing.T, result string) {
				assert.Contains(t, result, "Alert rule templates provide pre-defined configurations")
				assert.Contains(t, result, "**Test Alert Rule**")
				assert.Contains(t, result, "This is a test alert rule description")
			},
		},
		{
			name: "multiple valid templates",
			setupFunc: func(t *testing.T) string {
				tmpDir := t.TempDir()
				templatesDir := filepath.Join(tmpDir, "kibana", "alerting_rule_template")
				require.NoError(t, os.MkdirAll(templatesDir, 0o755))

				template1 := `{
					"attributes": {
						"name": "First Rule",
						"description": "First description"
					}
				}`
				template2 := `{
					"attributes": {
						"name": "Second Rule",
						"description": "Second description"
					}
				}`
				require.NoError(t, os.WriteFile(filepath.Join(templatesDir, "rule1.json"), []byte(template1), 0o644))
				require.NoError(t, os.WriteFile(filepath.Join(templatesDir, "rule2.json"), []byte(template2), 0o644))
				return tmpDir
			},
			expectError: false,
			expectEmpty: false,
			validateFunc: func(t *testing.T, result string) {
				assert.Contains(t, result, "**First Rule**")
				assert.Contains(t, result, "First description")
				assert.Contains(t, result, "**Second Rule**")
				assert.Contains(t, result, "Second description")
			},
		},
		{
			name: "skip non-json files",
			setupFunc: func(t *testing.T) string {
				tmpDir := t.TempDir()
				templatesDir := filepath.Join(tmpDir, "kibana", "alerting_rule_template")
				require.NoError(t, os.MkdirAll(templatesDir, 0o755))

				template := `{
					"attributes": {
						"name": "Valid Rule",
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
				assert.Contains(t, result, "**Valid Rule**")
				assert.NotContains(t, result, "ignored")
				assert.NotContains(t, result, "readme")
			},
		},
		{
			name: "skip subdirectories",
			setupFunc: func(t *testing.T) string {
				tmpDir := t.TempDir()
				templatesDir := filepath.Join(tmpDir, "kibana", "alerting_rule_template")
				require.NoError(t, os.MkdirAll(templatesDir, 0o755))

				template := `{
					"attributes": {
						"name": "Root Rule",
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
				// Should only have one rule mentioned
				assert.Equal(t, 1, strings.Count(result, "**Root Rule**"))
			},
		},
		{
			name: "unreadable file",
			setupFunc: func(t *testing.T) string {
				tmpDir := t.TempDir()
				templatesDir := filepath.Join(tmpDir, "kibana", "alerting_rule_template")
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
				templatesDir := filepath.Join(tmpDir, "kibana", "alerting_rule_template")
				require.NoError(t, os.MkdirAll(templatesDir, 0o755))

				invalidJSON := `{ "attributes": { "name": "Invalid" }`
				require.NoError(t, os.WriteFile(filepath.Join(templatesDir, "invalid.json"), []byte(invalidJSON), 0o644))
				return tmpDir
			},
			expectError: true,
			expectEmpty: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			packageRoot := tc.setupFunc(t)

			result, err := renderAlertRuleTemplates(packageRoot, newEmptyLinkMap())

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
