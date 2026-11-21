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

func TestRenderDashboards(t *testing.T) {
	cases := []struct {
		name         string
		setupFunc    func(t *testing.T) string
		expectError  bool
		expectEmpty  bool
		validateFunc func(t *testing.T, result string)
	}{
		{
			name: "no dashboards directory",
			setupFunc: func(t *testing.T) string {
				return t.TempDir()
			},
			expectError: false,
			expectEmpty: true,
		},
		{
			name: "empty dashboards directory",
			setupFunc: func(t *testing.T) string {
				tmpDir := t.TempDir()
				dashboardsDir := filepath.Join(tmpDir, "kibana", "dashboard")
				require.NoError(t, os.MkdirAll(dashboardsDir, 0o755))
				return tmpDir
			},
			expectError: false,
			expectEmpty: true,
		},
		{
			name: "single valid dashboard",
			setupFunc: func(t *testing.T) string {
				tmpDir := t.TempDir()
				dashboardsDir := filepath.Join(tmpDir, "kibana", "dashboard")
				require.NoError(t, os.MkdirAll(dashboardsDir, 0o755))

				dash := `{
					"attributes": {
						"title": "[PostgreSQL OTel Copy] Overview",
						"description": "Overview of PostgreSQL health and golden signals."
					}
				}`
				dashFile := filepath.Join(dashboardsDir, "overview.json")
				require.NoError(t, os.WriteFile(dashFile, []byte(dash), 0o644))
				return tmpDir
			},
			expectError: false,
			expectEmpty: false,
			validateFunc: func(t *testing.T, result string) {
				assert.Contains(t, result, "**The following dashboards are available:**")
				assert.Contains(t, result, "<details>")
				assert.Contains(t, result, "<summary>View the dashboards</summary>")
				assert.Contains(t, result, "</details>")
				assert.Contains(t, result, "| Dashboard | Description |")
				assert.Contains(t, result, "|---|---|")
				assert.Contains(t, result, "| **[PostgreSQL OTel Copy] Overview** | Overview of PostgreSQL health and golden signals. |")
			},
		},
		{
			name: "multiple valid dashboards",
			setupFunc: func(t *testing.T) string {
				tmpDir := t.TempDir()
				dashboardsDir := filepath.Join(tmpDir, "kibana", "dashboard")
				require.NoError(t, os.MkdirAll(dashboardsDir, 0o755))

				dash1 := `{
					"attributes": {
						"title": "First Dashboard",
						"description": "First description"
					}
				}`
				dash2 := `{
					"attributes": {
						"title": "Second Dashboard",
						"description": "Second description"
					}
				}`
				require.NoError(t, os.WriteFile(filepath.Join(dashboardsDir, "d1.json"), []byte(dash1), 0o644))
				require.NoError(t, os.WriteFile(filepath.Join(dashboardsDir, "d2.json"), []byte(dash2), 0o644))
				return tmpDir
			},
			expectError: false,
			expectEmpty: false,
			validateFunc: func(t *testing.T, result string) {
				assert.Contains(t, result, "<details>")
				assert.Contains(t, result, "<summary>View the dashboards</summary>")
				assert.Contains(t, result, "</details>")
				assert.Contains(t, result, "| Dashboard | Description |")
				assert.Contains(t, result, "|---|---|")
				assert.Contains(t, result, "| **First Dashboard** | First description |")
				assert.Contains(t, result, "| **Second Dashboard** | Second description |")
			},
		},
		{
			name: "skip non-json files",
			setupFunc: func(t *testing.T) string {
				tmpDir := t.TempDir()
				dashboardsDir := filepath.Join(tmpDir, "kibana", "dashboard")
				require.NoError(t, os.MkdirAll(dashboardsDir, 0o755))

				dash := `{
					"attributes": {
						"title": "Valid Dashboard",
						"description": "Valid description"
					}
				}`
				require.NoError(t, os.WriteFile(filepath.Join(dashboardsDir, "valid.json"), []byte(dash), 0o644))
				require.NoError(t, os.WriteFile(filepath.Join(dashboardsDir, "ignore.txt"), []byte("ignored"), 0o644))
				require.NoError(t, os.WriteFile(filepath.Join(dashboardsDir, "README.md"), []byte("# readme"), 0o644))
				return tmpDir
			},
			expectError: false,
			expectEmpty: false,
			validateFunc: func(t *testing.T, result string) {
				assert.Contains(t, result, "| **Valid Dashboard** | Valid description |")
				assert.NotContains(t, result, "ignored")
				assert.NotContains(t, result, "readme")
			},
		},
		{
			name: "skip subdirectories",
			setupFunc: func(t *testing.T) string {
				tmpDir := t.TempDir()
				dashboardsDir := filepath.Join(tmpDir, "kibana", "dashboard")
				require.NoError(t, os.MkdirAll(dashboardsDir, 0o755))

				dash := `{
					"attributes": {
						"title": "Root Dashboard",
						"description": "Root description"
					}
				}`
				require.NoError(t, os.WriteFile(filepath.Join(dashboardsDir, "root.json"), []byte(dash), 0o644))

				subDir := filepath.Join(dashboardsDir, "subdir")
				require.NoError(t, os.MkdirAll(subDir, 0o755))
				require.NoError(t, os.WriteFile(filepath.Join(subDir, "nested.json"), []byte(dash), 0o644))
				return tmpDir
			},
			expectError: false,
			expectEmpty: false,
			validateFunc: func(t *testing.T, result string) {
				assert.Equal(t, 1, strings.Count(result, "Root Dashboard"))
			},
		},
		{
			name: "unreadable file",
			setupFunc: func(t *testing.T) string {
				tmpDir := t.TempDir()
				dashboardsDir := filepath.Join(tmpDir, "kibana", "dashboard")
				require.NoError(t, os.MkdirAll(dashboardsDir, 0o755))

				unreadableFile := filepath.Join(dashboardsDir, "unreadable.json")
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
				dashboardsDir := filepath.Join(tmpDir, "kibana", "dashboard")
				require.NoError(t, os.MkdirAll(dashboardsDir, 0o755))

				invalidJSON := `{ "attributes": { "title": "Invalid" }`
				require.NoError(t, os.WriteFile(filepath.Join(dashboardsDir, "invalid.json"), []byte(invalidJSON), 0o644))
				return tmpDir
			},
			expectError: true,
			expectEmpty: false,
		},
		{
			name: "special characters are escaped",
			setupFunc: func(t *testing.T) string {
				tmpDir := t.TempDir()
				dashboardsDir := filepath.Join(tmpDir, "kibana", "dashboard")
				require.NoError(t, os.MkdirAll(dashboardsDir, 0o755))

				dash := `{
					"attributes": {
						"title": "Dashboard with *bold* and {braces}",
						"description": "Description with <angle> and {curly} brackets"
					}
				}`
				require.NoError(t, os.WriteFile(filepath.Join(dashboardsDir, "special.json"), []byte(dash), 0o644))
				return tmpDir
			},
			expectError: false,
			expectEmpty: false,
			validateFunc: func(t *testing.T, result string) {
				assert.Contains(t, result, `\*bold\*`)
				assert.Contains(t, result, `\{braces\}`)
				assert.Contains(t, result, `\<angle\>`)
				assert.Contains(t, result, `\{curly\}`)
				assert.NotContains(t, result, "*bold*")
				assert.NotContains(t, result, "{braces}")
				assert.NotContains(t, result, "<angle>")
			},
		},
		{
			name: "newlines in description are flattened",
			setupFunc: func(t *testing.T) string {
				tmpDir := t.TempDir()
				dashboardsDir := filepath.Join(tmpDir, "kibana", "dashboard")
				require.NoError(t, os.MkdirAll(dashboardsDir, 0o755))

				dash := `{
					"attributes": {
						"title": "Multiline",
						"description": "Line one.\nLine two."
					}
				}`
				require.NoError(t, os.WriteFile(filepath.Join(dashboardsDir, "ml.json"), []byte(dash), 0o644))
				return tmpDir
			},
			expectError: false,
			expectEmpty: false,
			validateFunc: func(t *testing.T, result string) {
				assert.Contains(t, result, "| **Multiline** | Line one. Line two. |")
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			packageRoot := tc.setupFunc(t)

			result, err := renderDashboards(packageRoot)

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
