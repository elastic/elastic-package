// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package docs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/elastic-package/internal/packages"
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
				createPackageManifestFile(t, tmpDir, "3.6.0")
				return tmpDir
			},
			expectError: false,
			expectEmpty: true,
		},
		{
			name: "empty templates directory",
			setupFunc: func(t *testing.T) string {
				tmpDir := t.TempDir()
				createPackageManifestFile(t, tmpDir, "3.6.0")
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
				createPackageManifestFile(t, tmpDir, "3.6.0")
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
				assert.Contains(t, result, "require Elastic Stack version 9.2.0 or later")
				assert.NotContains(t, result, "9.6.0")
				assert.Contains(t, result, "<details>")
				assert.Contains(t, result, "<summary>View the alert rule templates</summary>")
				assert.Contains(t, result, "</details>")
				assert.Contains(t, result, "| Name | Description |")
				assert.Contains(t, result, "| Test Alert Rule | This is a test alert rule description |")
			},
		},
		{
			name: "multiple valid templates",
			setupFunc: func(t *testing.T) string {
				tmpDir := t.TempDir()
				createPackageManifestFile(t, tmpDir, "3.6.0")
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
				assert.Contains(t, result, "<details>")
				assert.Contains(t, result, "</details>")
				assert.Contains(t, result, "| Name | Description |")
				assert.Contains(t, result, "| First Rule | First description |")
				assert.Contains(t, result, "| Second Rule | Second description |")
			},
		},
		{
			name: "skip non-json files",
			setupFunc: func(t *testing.T) string {
				tmpDir := t.TempDir()
				createPackageManifestFile(t, tmpDir, "3.6.0")
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
				assert.Contains(t, result, "| Valid Rule | Valid description |")
				assert.NotContains(t, result, "ignored")
				assert.NotContains(t, result, "readme")
			},
		},
		{
			name: "skip subdirectories",
			setupFunc: func(t *testing.T) string {
				tmpDir := t.TempDir()
				createPackageManifestFile(t, tmpDir, "3.6.0")
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
				assert.Equal(t, 1, strings.Count(result, "Root Rule"))
				assert.Contains(t, result, "| Root Rule | Root description |")
			},
		},
		{
			name: "table structure is correct",
			setupFunc: func(t *testing.T) string {
				tmpDir := t.TempDir()
				createPackageManifestFile(t, tmpDir, "3.6.0")
				templatesDir := filepath.Join(tmpDir, "kibana", "alerting_rule_template")
				require.NoError(t, os.MkdirAll(templatesDir, 0o755))

				template1 := `{
					"attributes": {
						"name": "First Rule Name",
						"description": "First Rule Description"
					}
				}`
				template2 := `{
					"attributes": {
						"name": "Second Rule Name",
						"description": "Second Rule Description"
					}
				}`
				require.NoError(t, os.WriteFile(filepath.Join(templatesDir, "rule1.json"), []byte(template1), 0o644))
				require.NoError(t, os.WriteFile(filepath.Join(templatesDir, "rule2.json"), []byte(template2), 0o644))
				return tmpDir
			},
			expectError: false,
			expectEmpty: false,
			validateFunc: func(t *testing.T, result string) {
				assert.Contains(t, result, "<details>")
				assert.Contains(t, result, "<summary>View the alert rule templates</summary>")
				assert.Contains(t, result, "</details>")
				assert.Contains(t, result, "| Name | Description |")
				assert.Contains(t, result, "|---|---|")
				assert.Contains(t, result, "| First Rule Name | First Rule Description |")
				assert.Contains(t, result, "| Second Rule Name | Second Rule Description |")
			},
		},
		{
			name: "list structure is correct",
			setupFunc: func(t *testing.T) string {
				tmpDir := t.TempDir()
				createPackageManifestFile(t, tmpDir, "3.5.0")
				templatesDir := filepath.Join(tmpDir, "kibana", "alerting_rule_template")
				require.NoError(t, os.MkdirAll(templatesDir, 0o755))

				template1 := `{
					"attributes": {
						"name": "First Rule Name",
						"description": "First Rule Description"
					}
				}`
				template2 := `{
					"attributes": {
						"name": "Second Rule Name",
						"description": "Second Rule Description"
					}
				}`
				require.NoError(t, os.WriteFile(filepath.Join(templatesDir, "rule1.json"), []byte(template1), 0o644))
				require.NoError(t, os.WriteFile(filepath.Join(templatesDir, "rule2.json"), []byte(template2), 0o644))
				return tmpDir
			},
			expectError: false,
			expectEmpty: false,
			validateFunc: func(t *testing.T, result string) {
				assert.NotContains(t, result, "<details>")
				assert.NotContains(t, result, "<summary>View the alert rule templates</summary>")
				assert.NotContains(t, result, "</details>")
				assert.Contains(t, result, "The following alert rule templates are available:")
				assert.Contains(t, result, "- **First Rule Name** — First Rule Description")
				assert.Contains(t, result, "- **Second Rule Name** — Second Rule Description")
			},
		},
		{
			name: "special characters are escaped",
			setupFunc: func(t *testing.T) string {
				tmpDir := t.TempDir()
				createPackageManifestFile(t, tmpDir, "3.6.0")
				templatesDir := filepath.Join(tmpDir, "kibana", "alerting_rule_template")
				require.NoError(t, os.MkdirAll(templatesDir, 0o755))

				template := `{
					"attributes": {
						"name": "Rule with *bold* and {braces}",
						"description": "Description with <angle> and {curly} brackets"
					}
				}`
				require.NoError(t, os.WriteFile(filepath.Join(templatesDir, "special.json"), []byte(template), 0o644))
				return tmpDir
			},
			expectError: false,
			expectEmpty: false,
			validateFunc: func(t *testing.T, result string) {
				assert.Contains(t, result, "<details>")
				assert.Contains(t, result, "</details>")
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
			name: "table pipes in names are escaped",
			setupFunc: func(t *testing.T) string {
				tmpDir := t.TempDir()
				createPackageManifestFile(t, tmpDir, "3.6.0")
				templatesDir := filepath.Join(tmpDir, "kibana", "alerting_rule_template")
				require.NoError(t, os.MkdirAll(templatesDir, 0o755))
				template := `{
					"attributes": {
						"engine": "v2",
						"rule": {
							"metadata": {
								"name": "Not ES|QL",
								"description": "line one\nline | two"
							}
						}
					}
				}`
				require.NoError(t, os.WriteFile(filepath.Join(templatesDir, "pipe.json"), []byte(template), 0o644))
				return tmpDir
			},
			expectError: false,
			expectEmpty: false,
			validateFunc: func(t *testing.T, result string) {
				assert.Contains(t, result, `| Not ES\|QL | line one line \| two |`)
			},
		},
		{
			name: "unreadable file",
			setupFunc: func(t *testing.T) string {
				tmpDir := t.TempDir()
				createPackageManifestFile(t, tmpDir, "3.6.0")
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
				createPackageManifestFile(t, tmpDir, "3.6.0")
				templatesDir := filepath.Join(tmpDir, "kibana", "alerting_rule_template")
				require.NoError(t, os.MkdirAll(templatesDir, 0o755))

				invalidJSON := `{ "attributes": { "name": "Invalid" }`
				require.NoError(t, os.WriteFile(filepath.Join(templatesDir, "invalid.json"), []byte(invalidJSON), 0o644))
				return tmpDir
			},
			expectError: true,
			expectEmpty: false,
		},
		{
			name: "list omits blank descriptions",
			setupFunc: func(t *testing.T) string {
				tmpDir := t.TempDir()
				createPackageManifestFile(t, tmpDir, "3.5.0")
				templatesDir := filepath.Join(tmpDir, "kibana", "alerting_rule_template")
				require.NoError(t, os.MkdirAll(templatesDir, 0o755))
				template := `{
					"attributes": {
						"name": "[PostgreSQL] Latency high",
						"description": "   "
					}
				}`
				require.NoError(t, os.WriteFile(filepath.Join(templatesDir, "rule.json"), []byte(template), 0o644))
				return tmpDir
			},
			expectError: false,
			expectEmpty: false,
			validateFunc: func(t *testing.T, result string) {
				assert.Contains(t, result, "- **[PostgreSQL] Latency high**\n")
				assert.NotContains(t, result, "—")
			},
		},
		{
			name: "v2 engine template uses metadata and 9.6",
			setupFunc: func(t *testing.T) string {
				tmpDir := t.TempDir()
				createPackageManifestFile(t, tmpDir, "3.6.0")
				templatesDir := filepath.Join(tmpDir, "kibana", "alerting_rule_template")
				require.NoError(t, os.MkdirAll(templatesDir, 0o755))
				template := `{
					"attributes": {
						"engine": "v2",
						"name": "Wrong location",
						"rule": {
							"metadata": {
								"name": "V2 Alert Rule",
								"description": "This is a v2 alert rule description"
							}
						}
					}
				}`
				require.NoError(t, os.WriteFile(filepath.Join(templatesDir, "v2_rule.json"), []byte(template), 0o644))
				return tmpDir
			},
			expectError: false,
			expectEmpty: false,
			validateFunc: func(t *testing.T, result string) {
				assert.Contains(t, result, "| V2 Alert Rule | This is a v2 alert rule description |")
				assert.NotContains(t, result, "Wrong location")
				assert.Contains(t, result, "require Elastic Stack version 9.6.0 or later")
				assert.NotContains(t, result, "9.2.0")
				assert.NotContains(t, result, "| Engine |")
			},
		},
		{
			name: "mixed table omits a blank description column",
			setupFunc: func(t *testing.T) string {
				tmpDir := t.TempDir()
				createPackageManifestFile(t, tmpDir, "3.4.0")
				templatesDir := filepath.Join(tmpDir, "kibana", "alerting_rule_template")
				require.NoError(t, os.MkdirAll(templatesDir, 0o755))
				require.NoError(t, os.WriteFile(filepath.Join(templatesDir, "a.json"), []byte(`{"attributes":{"name":"Latency high"}}`), 0o644))
				require.NoError(t, os.WriteFile(filepath.Join(templatesDir, "b.json"), []byte(`{"attributes":{"engine":"v2","rule":{"metadata":{"name":"Latency high"}}}}`), 0o644))
				return tmpDir
			},
			expectError: false,
			expectEmpty: false,
			validateFunc: func(t *testing.T, result string) {
				assert.Contains(t, result, "| Name | ES\\|QL | Standard |")
				assert.Contains(t, result, "|---|:---:|:---:|")
				assert.NotContains(t, result, "Description")
				assert.Contains(t, result, "| Latency high | ✓ | ✓ |")
			},
		},
		{
			name: "mixed engines are split into fleet sections",
			setupFunc: func(t *testing.T) string {
				return writeMixedAlertRuleTemplates(t, "3.6.0")
			},
			expectError: false,
			expectEmpty: false,
			validateFunc: func(t *testing.T, result string) {
				assert.Contains(t, result, "| Name | Description | Engine |")
				assert.Contains(t, result, "| Same name | V2 description | ES\\|QL |")
				assert.Contains(t, result, "| Same name | Classic description | Standard |")
				esql := strings.Index(result, "ES\\|QL")
				standard := strings.Index(result, "| Standard |")
				assert.Greater(t, standard, esql)
				assert.NotContains(t, result, "Wrong location")
				assert.Contains(t, result, "Standard alert rule templates require Elastic Stack version 9.2.0 or later.")
				assert.Contains(t, result, "ES|QL alert rule templates require Elastic Stack version 9.6.0 or later.")
			},
		},
		{
			name: "mixed engines use the same sections below spec 3.6.0",
			setupFunc: func(t *testing.T) string {
				return writeMixedAlertRuleTemplates(t, "3.5.0")
			},
			expectError: false,
			expectEmpty: false,
			validateFunc: func(t *testing.T, result string) {
				assert.Contains(t, result, "<summary>View the alert rule templates</summary>")
				assert.Contains(t, result, "| Same name | V2 description | ES\\|QL |")
				assert.Contains(t, result, "| Same name | Classic description | Standard |")
				assert.NotContains(t, result, "Wrong location")
			},
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

func writeMixedAlertRuleTemplates(t *testing.T, specVersion string) string {
	t.Helper()
	tmpDir := t.TempDir()
	createPackageManifestFile(t, tmpDir, specVersion)
	templatesDir := filepath.Join(tmpDir, "kibana", "alerting_rule_template")
	require.NoError(t, os.MkdirAll(templatesDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(templatesDir, "a.json"), []byte(`{"attributes":{"name":"Same name","description":"Classic description"}}`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(templatesDir, "z.json"), []byte(`{"attributes":{"engine":"v2","name":"Wrong location","rule":{"metadata":{"name":"Same name","description":"V2 description"}}}}`), 0o644))
	return tmpDir
}

func TestMixedAlertRuleTemplateFileOrder(t *testing.T) {
	for _, v2First := range []bool{false, true} {
		root := t.TempDir()
		createPackageManifestFile(t, root, "3.6.0")
		dir := filepath.Join(root, "kibana", "alerting_rule_template")
		require.NoError(t, os.MkdirAll(dir, 0o755))
		classicName, v2Name := "a.json", "z.json"
		if v2First {
			classicName, v2Name = v2Name, classicName
		}
		require.NoError(t, os.WriteFile(filepath.Join(dir, classicName), []byte(`{"attributes":{"name":"Same name","description":"Classic description"}}`), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(dir, v2Name), []byte(`{"attributes":{"engine":"v2","name":"Wrong location","rule":{"metadata":{"name":"Same name","description":"V2 description"}}}}`), 0o644))
		result, err := renderAlertRuleTemplates(root, newEmptyLinkMap())
		require.NoError(t, err)
		require.Contains(t, result, "| Same name | V2 description | ES\\|QL |")
		require.Contains(t, result, "| Same name | Classic description | Standard |")
		require.NotContains(t, result, "Wrong location")
		require.Contains(t, result, "version 9.6.0 or later")
	}
}

func createPackageManifestFile(t *testing.T, packageRoot, specVersion string) {
	t.Helper()
	manifest := fmt.Sprintf(`format_version: %s
type: integration
version: 1.0.0
`, specVersion)
	manifestFile := filepath.Join(packageRoot, packages.PackageManifestFile)
	require.NoError(t, os.WriteFile(manifestFile, []byte(manifest), 0o644))
}
