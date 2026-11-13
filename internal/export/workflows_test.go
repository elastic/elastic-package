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

		file, err := TransformWorkflow("my_pkg", kibana.ExportedWorkflow{ID: "alert-triage-v2", YAML: string(content)}, nil)
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
			title:       "missing name",
			content:     "description: nothing else\n",
			expectedErr: `cannot build a file name from workflow name ""`,
		},
		{
			title:       "invalid yaml",
			content:     "name: [unclosed\n",
			expectedErr: "could not parse workflow",
		},
	}

	for _, c := range cases {
		t.Run(c.title, func(t *testing.T) {
			file, err := TransformWorkflow("my_pkg", kibana.ExportedWorkflow{ID: "test", YAML: c.content}, nil)
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

func TestTransformWorkflowExistingFile(t *testing.T) {
	content := "name: Alert Triage Renamed\nenabled: true\ntriggers:\n  - type: manual\nsteps:\n  - name: log\n    type: console\n"
	existingFiles := []string{"my_pkg-other.yml", "my_pkg-alert-triage.yml", "README.md"}

	cases := []struct {
		title    string
		id       string
		expected string
	}{
		{title: "ID matches an existing file", id: "my-pkg-alert-triage", expected: "my_pkg-alert-triage.yml"},
		{title: "no matching file", id: "8f3c0f4e-2b8a-4d2e-9d47-0f8f5f5b6c1a", expected: "my_pkg-alert-triage-renamed.yml"},
		{title: "ID matches a non-workflow file", id: "readme", expected: "my_pkg-alert-triage-renamed.yml"},
	}

	for _, c := range cases {
		t.Run(c.title, func(t *testing.T) {
			file, err := TransformWorkflow("my_pkg", kibana.ExportedWorkflow{ID: c.id, YAML: content}, existingFiles)
			require.NoError(t, err)
			assert.Equal(t, c.expected, file.Name)
		})
	}
}
