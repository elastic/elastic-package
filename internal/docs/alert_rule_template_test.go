// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package docs

import (
	"encoding/json"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/parser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/elastic-package/internal/fields"
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
				assert.Contains(t, result, "**First Rule Name**\n\nFirst Rule Description\n\n")
				assert.Contains(t, result, "**Second Rule Name**\n\nSecond Rule Description\n\n")
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
			name: "classic single-engine table matches main escaping",
			setupFunc: func(t *testing.T) string {
				tmpDir := t.TempDir()
				createPackageManifestFile(t, tmpDir, "3.6.0")
				templatesDir := filepath.Join(tmpDir, "kibana", "alerting_rule_template")
				require.NoError(t, os.MkdirAll(templatesDir, 0o755))
				template := `{
					"attributes": {
						"name": "[AWS EC2 OTel] Not ES|QL",
						"description": "line one\nline | two"
					}
				}`
				require.NoError(t, os.WriteFile(filepath.Join(templatesDir, "pipe.json"), []byte(template), 0o644))
				return tmpDir
			},
			expectError: false,
			expectEmpty: false,
			validateFunc: func(t *testing.T, result string) {
				// Classic single-engine tables mirror main: brackets stay
				// unescaped and pipes remain literal, so existing READMEs do not
				// churn.
				assert.Contains(t, result, `| [AWS EC2 OTel] Not ES|QL | line one line | two |`)
				assert.NotContains(t, result, `\[AWS EC2 OTel\]`)
				assert.NotContains(t, result, `\|`)
			},
		},
		{
			name: "v2 single-engine table escapes pipes",
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
								"name": "[AWS EC2 OTel] ES|QL",
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
				// Alerting v2 is new output with no byte-identity constraint, so
				// pipes are escaped and the cell is not split by "ES|QL".
				assert.Contains(t, result, `| \[AWS EC2 OTel\] ES\|QL | line one line \| two |`)
				assert.Contains(t, result, `\|`)
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
			name: "list retains classic blank descriptions",
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
				assert.Contains(t, result, "**[PostgreSQL] Latency high**\n\n   \n\n")
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
				assert.Contains(t, result, "| Name | Description | ES\\|QL | Standard |")
				assert.Contains(t, result, "| Same name | V2 description | ✓ |  |")
				assert.Contains(t, result, "| Same name | Classic description |  | ✓ |")
				v2Row := strings.Index(result, "| Same name | V2 description |")
				v1Row := strings.Index(result, "| Same name | Classic description |")
				assert.Greater(t, v1Row, v2Row)
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
				assert.Contains(t, result, "| Same name | V2 description | ✓ |  |")
				assert.Contains(t, result, "| Same name | Classic description |  | ✓ |")
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
		require.Contains(t, result, "| Same name | V2 description | ✓ |  |")
		require.Contains(t, result, "| Same name | Classic description |  | ✓ |")
		require.NotContains(t, result, "Wrong location")
		require.Contains(t, result, "version 9.6.0 or later")
	}
}

func TestAlertRuleReadmeDescriptionColumns(t *testing.T) {
	layouts := []struct {
		name    string
		engines []string
		headers []string
		rows    int
	}{
		{"v1", []string{"v1", "v1"}, []string{"Name"}, 2},
		{"v2", []string{"v2", "v2"}, []string{"Name"}, 2},
		{"paired", []string{"v1", "v2"}, []string{"Name", "ES|QL", "Standard"}, 1},
		{"duplicate names", []string{"v1", "v1", "v2"}, []string{"Name", "ES|QL", "Standard"}, 3},
	}
	descriptions := []struct {
		name    string
		value   string
		omit    bool
		mixed   bool
		visible bool
	}{
		{name: "missing", omit: true},
		{name: "empty"},
		{name: "whitespace", value: " \t\r\n "},
		{name: "mixed", value: "Investigate latency.", mixed: true, visible: true},
		{name: "nonempty", value: "Investigate latency.", visible: true},
	}
	htmlTags := regexp.MustCompile(`<[^>]+>`)
	for _, version := range []string{"3.5.0", "3.6.0"} {
		for _, layout := range layouts {
			for _, description := range descriptions {
				t.Run(version+"/"+layout.name+"/"+description.name, func(t *testing.T) {
					root := t.TempDir()
					require.NoError(t, os.CopyFS(root, os.DirFS("../../test/packages/other/alert_rule_templates")))
					manifestPath := filepath.Join(root, packages.PackageManifestFile)
					manifest, err := os.ReadFile(manifestPath)
					require.NoError(t, err)
					require.NoError(t, os.WriteFile(manifestPath, []byte(strings.ReplaceAll(string(manifest), "format_version: 3.6.0", "format_version: "+version)), 0o644))
					dir := filepath.Join(root, "kibana", "alerting_rule_template")
					require.NoError(t, os.RemoveAll(dir))
					require.NoError(t, os.MkdirAll(dir, 0o755))
					for i, engine := range layout.engines {
						name := "Latency high"
						if i == 1 && (layout.name == "v1" || layout.name == "v2") {
							name = "Error rate high"
						}
						metadata := map[string]any{"name": name}
						if !description.omit {
							value := description.value
							if description.mixed && i == 0 {
								value = ""
							}
							metadata["description"] = value
						}
						writeAlertRuleReadmeTemplate(t, dir, fmt.Sprintf("%d.json", i), engine, metadata)
					}
					repository, err := os.OpenRoot(root)
					require.NoError(t, err)
					defer repository.Close()
					rendered, found, err := GenerateReadme(repository, "README.md", "", root, fields.SchemaURLs{})
					require.NoError(t, err)
					require.True(t, found)
					body := strings.NewReplacer("<details>", "", "</details>", "", "<summary>View the alert rule templates</summary>", "").Replace(string(rendered))
					renderedHTML := string(markdown.ToHTML([]byte(body), parser.NewWithExtensions(parser.CommonExtensions), nil))
					if description.visible {
						require.Contains(t, renderedHTML, description.value, "nonempty descriptions must be preserved")
					}
					if version == "3.5.0" && (layout.name == "v1" || layout.name == "v2") {
						require.NotContains(t, renderedHTML, "<table>")
						require.Equal(t, layout.rows, strings.Count(renderedHTML, "<strong>"))
						return
					}
					headers := append([]string(nil), layout.headers...)
					if description.visible || layout.name == "v1" || layout.name == "v2" {
						headers = append([]string{"Name", "Description"}, headers[1:]...)
					}
					var actualHeaders []string
					for _, cell := range regexp.MustCompile(`<th[^>]*>(.*?)</th>`).FindAllStringSubmatch(renderedHTML, -1) {
						actualHeaders = append(actualHeaders, html.UnescapeString(htmlTags.ReplaceAllString(cell[1], "")))
					}
					require.Equal(t, headers, actualHeaders)
					rows := regexp.MustCompile(`<tr>\s*((?:<td[\s\S]*?</td>\s*)+)</tr>`).FindAllStringSubmatch(renderedHTML, -1)
					require.Len(t, rows, layout.rows)
					for _, row := range rows {
						require.Len(t, regexp.MustCompile(`<td[^>]*>`).FindAllString(row[1], -1), len(headers), "every row must match the header columns")
					}
				})
			}
		}
	}
}

func writeAlertRuleReadmeTemplate(t *testing.T, dir, filename, engine string, metadata map[string]any) {
	t.Helper()
	attributes := metadata
	if engine == "v2" {
		attributes = map[string]any{"engine": "v2", "rule": map[string]any{"metadata": metadata}}
	}
	raw, err := json.Marshal(map[string]any{"attributes": attributes})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, filename), raw, 0o644))
}

func TestAlertRuleReadmeFormatting(t *testing.T) {
	htmlTags := regexp.MustCompile(`<[^>]+>`)
	texts := []struct {
		name, description, want string
	}{
		{`Rule \| value`, `Path C:\|archive`, `Rule \| value`},
		{"Rule\r\nname", "First\r\nSecond", "Rule name"},
		{"Rule\rname", "First\rSecond", "Rule name"},
		{"[Service] *bold* _under_ `code` ~~strike~~ [link](https://example.com)", "Description", "[Service] *bold* _under_ `code` ~~strike~~ [link](https://example.com)"},
		{"Rule <img src=x> &amp; value", "Check <angle>", "Rule <img src=x> &amp; value"},
	}
	for _, version := range []string{"3.5.0", "3.6.0"} {
		for _, layout := range []string{"paired", "fallback"} {
			for i, text := range texts {
				t.Run(fmt.Sprintf("%s/%s/%d", version, layout, i), func(t *testing.T) {
					root := t.TempDir()
					require.NoError(t, os.CopyFS(root, os.DirFS("../../test/packages/other/alert_rule_templates")))
					manifestPath := filepath.Join(root, packages.PackageManifestFile)
					manifest, err := os.ReadFile(manifestPath)
					require.NoError(t, err)
					require.NoError(t, os.WriteFile(manifestPath, []byte(strings.ReplaceAll(string(manifest), "format_version: 3.6.0", "format_version: "+version)), 0o644))
					dir := filepath.Join(root, "kibana", "alerting_rule_template")
					require.NoError(t, os.RemoveAll(dir))
					require.NoError(t, os.MkdirAll(dir, 0o755))
					for _, engine := range []string{"v1", "v2"} {
						description := text.description + " [Dashboard](https://example.com)"
						if layout == "fallback" && engine == "v2" {
							description += " extra"
						}
						metadata := map[string]any{"name": text.name, "description": description}
						writeAlertRuleReadmeTemplate(t, dir, engine+".json", engine, metadata)
					}
					repository, err := os.OpenRoot(root)
					require.NoError(t, err)
					defer repository.Close()
					rendered, found, err := GenerateReadme(repository, "README.md", "", root, fields.SchemaURLs{})
					require.NoError(t, err)
					require.True(t, found)
					// Parse the generated Markdown independently of the renderer. Strip
					// the disclosure tags because this parser treats HTML blocks as raw.
					body := strings.NewReplacer("<details>", "", "</details>", "", "<summary>View the alert rule templates</summary>", "").Replace(string(rendered))
					renderedHTML := string(markdown.ToHTML([]byte(body), parser.NewWithExtensions(parser.CommonExtensions), nil))
					require.Contains(t, renderedHTML, `href="https://example.com"`, "description links must remain usable")
					// Both layouts use the ES|QL and Standard columns. "paired"
					// has matching descriptions, so it collapses to one row with
					// both marks. "fallback" gives the v2 template a different
					// description, so the shared name expands to one row per
					// engine (ES|QL first), each with a single mark.
					columns := 4
					rows := 1
					if layout == "fallback" {
						rows = 2
					}
					cells := regexp.MustCompile(`<td[^>]*>(.*?)</td>`).FindAllStringSubmatch(renderedHTML, -1)
					require.Len(t, cells, columns*rows, "table cells must not be split or dropped")
					for row := range rows {
						require.Equal(t, text.want, html.UnescapeString(htmlTags.ReplaceAllString(cells[row*columns][1], "")))
					}
					if layout == "paired" {
						require.Equal(t, "✓", cells[2][1])
						require.Equal(t, "✓", cells[3][1])
					} else {
						// v2 row first: ES|QL mark only; v1 row: Standard mark only.
						require.Equal(t, "✓", cells[2][1])
						require.Equal(t, "", cells[3][1])
						require.Equal(t, "", cells[6][1])
						require.Equal(t, "✓", cells[7][1])
					}
				})
			}
		}
	}
}

func TestSingleEngineReadmeMatchesMain(t *testing.T) {
	htmlTags := regexp.MustCompile(`<[^>]+>`)
	// Classic single-engine 3.6.0 tables must stay byte-compatible with main's
	// escaping: emphasis and angle brackets are escaped, while labels in square
	// brackets and inline links are left intact. A regression here churns every
	// published README that uses {{alertRuleTemplates}}. Alerting v2 is new
	// output with no byte-identity constraint, so its names are fully escaped.
	for _, engine := range []string{"v1", "v2"} {
		t.Run(engine, func(t *testing.T) {
			root := t.TempDir()
			require.NoError(t, os.CopyFS(root, os.DirFS("../../test/packages/other/alert_rule_templates")))
			dir := filepath.Join(root, "kibana", "alerting_rule_template")
			require.NoError(t, os.RemoveAll(dir))
			require.NoError(t, os.MkdirAll(dir, 0o755))
			metadata := map[string]any{
				"name":        "[AWS EC2 OTel] *CPU* <high>",
				"description": "Investigate [the runbook](https://example.com) for <details>.",
			}
			writeAlertRuleReadmeTemplate(t, dir, engine+".json", engine, metadata)

			repository, err := os.OpenRoot(root)
			require.NoError(t, err)
			defer repository.Close()
			rendered, found, err := GenerateReadme(repository, "README.md", "", root, fields.SchemaURLs{})
			require.NoError(t, err)
			require.True(t, found)

			if engine == "v2" {
				// Alerting v2 names are fully escaped, including square brackets.
				require.Contains(t, string(rendered), `\[AWS EC2 OTel\]`)
			} else {
				// Classic labels in square brackets are not escaped, matching main.
				require.Contains(t, string(rendered), "[AWS EC2 OTel]")
				require.NotContains(t, string(rendered), `\[AWS EC2 OTel\]`)
			}
			// Emphasis and angle brackets are escaped in both paths.
			require.Contains(t, string(rendered), `\*CPU\*`)
			require.Contains(t, string(rendered), `\<high\>`)

			body := strings.NewReplacer("<details>", "", "</details>", "", "<summary>View the alert rule templates</summary>", "").Replace(string(rendered))
			renderedHTML := string(markdown.ToHTML([]byte(body), parser.NewWithExtensions(parser.CommonExtensions), nil))
			require.Contains(t, renderedHTML, `href="https://example.com"`, "description links must remain usable")
			cells := regexp.MustCompile(`<td[^>]*>(.*?)</td>`).FindAllStringSubmatch(renderedHTML, -1)
			require.Len(t, cells, 2, "single-engine table must have exactly Name and Description cells")
			// After Markdown rendering the backslash escapes resolve, so both
			// paths display the same name text.
			require.Equal(t, "[AWS EC2 OTel] *CPU* <high>", html.UnescapeString(htmlTags.ReplaceAllString(cells[0][1], "")))
		})
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
