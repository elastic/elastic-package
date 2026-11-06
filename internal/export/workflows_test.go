// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package export

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/elastic-package/internal/kibana"
)

func TestWorkflowFileName(t *testing.T) {
	cases := []struct {
		workflowName string
		expected     string
		expectedErr  string
	}{
		{workflowName: "alert-triage", expected: "my_pkg-alert-triage.yml"},
		{workflowName: "Alert Triage (v2)", expected: "my_pkg-alert-triage-v2.yml"},
		{workflowName: "  Daily_Report: ÉTÉ!! ", expected: "my_pkg-daily-report-t.yml"},
		{workflowName: "!!!", expectedErr: "cannot build a file name"},
		{workflowName: "", expectedErr: "cannot build a file name"},
	}

	for _, c := range cases {
		t.Run(c.workflowName, func(t *testing.T) {
			fileName, err := workflowFileName("my_pkg", c.workflowName)
			if c.expectedErr != "" {
				require.ErrorContains(t, err, c.expectedErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, c.expected, fileName)
		})
	}
}

func TestTransformWorkflow(t *testing.T) {
	t.Run("exported from Kibana", func(t *testing.T) {
		content, err := os.ReadFile(filepath.Join("testdata", "workflows", "alert_triage.kibana.yml"))
		require.NoError(t, err)

		file, err := TransformWorkflow("my_pkg", kibana.ExportedWorkflow{ID: "alert-triage-v2", YAML: string(content)})
		require.NoError(t, err)

		expected, err := os.ReadFile(filepath.Join("testdata", "workflows", file.Name))
		require.NoError(t, err)
		assert.Equal(t, "my_pkg-alert-triage-v2.yml", file.Name)
		assert.Equal(t, string(expected), string(file.Content))
	})

	cases := []struct {
		title       string
		content     string
		expected    string
		expectedErr string
	}{
		{
			title: "enabled missing",
			content: `name: Test
triggers:
  - type: manual
steps:
  - name: log
    type: console
`,
			expected: `name: Test
triggers:
  - type: manual
steps:
  - name: log
    type: console
enabled: false
`,
		},
		{
			title: "already disabled",
			content: `name: Test
enabled: false
triggers:
  - type: manual
steps:
  - name: log
    type: console
`,
			expected: `name: Test
enabled: false
triggers:
  - type: manual
steps:
  - name: log
    type: console
`,
		},
		{
			title:       "missing required fields",
			content:     "description: nothing else\n",
			expectedErr: "invalid workflow: missing name\nmissing triggers\nmissing steps",
		},
		{
			title:       "empty steps",
			content:     "name: Test\ntriggers:\n  - type: manual\nsteps: []\n",
			expectedErr: "missing steps",
		},
		{
			title:       "invalid yaml",
			content:     "name: [unclosed\n",
			expectedErr: "could not parse workflow",
		},
	}

	for _, c := range cases {
		t.Run(c.title, func(t *testing.T) {
			file, err := TransformWorkflow("my_pkg", kibana.ExportedWorkflow{ID: "test", YAML: c.content})
			if c.expectedErr != "" {
				require.ErrorContains(t, err, c.expectedErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, "my_pkg-test.yml", file.Name)
			assert.Equal(t, c.expected, string(file.Content))
		})
	}
}

func TestTransformFleetWorkflow(t *testing.T) {
	const content = `name: Scheduled ES|QL report
enabled: false
triggers:
  - type: manual
steps:
  - name: log
    type: console
`
	cases := []struct {
		title       string
		pkgName     string
		id          string
		expected    string
		expectedErr string
	}{
		{
			title:    "keeps the installed file name",
			pkgName:  "good_workflow_integration",
			id:       "system-fleet-package-workflow-37a8eec1ce19687d-8ed92b542831a71c-good-workflow-integration-scheduled-esql-report",
			expected: "good_workflow_integration-scheduled-esql-report.yml",
		},
		{
			title:    "package name without underscores",
			pkgName:  "nginx",
			id:       "system-fleet-package-workflow-37a8eec1ce19687d-8ed92b542831a71c-nginx-scheduled-esql-report",
			expected: "nginx-scheduled-esql-report.yml",
		},
		{
			title:       "installed from another package",
			pkgName:     "nginx",
			id:          "system-fleet-package-workflow-37a8eec1ce19687d-8ed92b542831a71c-apache-scheduled-esql-report",
			expectedErr: `was not installed from package "nginx"`,
		},
		{
			title:    "not installed by Fleet",
			pkgName:  "nginx",
			id:       "scheduled-esql-report",
			expected: "nginx-scheduled-es-ql-report.yml",
		},
	}

	for _, c := range cases {
		t.Run(c.title, func(t *testing.T) {
			file, err := TransformWorkflow(c.pkgName, kibana.ExportedWorkflow{ID: c.id, YAML: content})
			if c.expectedErr != "" {
				require.ErrorContains(t, err, c.expectedErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, c.expected, file.Name)
			assert.Equal(t, content, string(file.Content))
		})
	}
}
