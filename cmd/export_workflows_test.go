// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/elastic-package/internal/kibana"
)

func testWorkflowYAML(name string) string {
	return "name: " + name + "\nenabled: true\ntriggers:\n  - type: manual\nsteps:\n  - name: log\n    type: console\n"
}

func TestCheckWorkflowsExportable(t *testing.T) {
	unmanaged := []kibana.Workflow{{ID: "authored", Name: "Authored"}, {ID: "other", Name: "Other"}}

	t.Run("unmanaged workflows", func(t *testing.T) {
		require.NoError(t, checkWorkflowsExportable([]string{"authored", "other"}, unmanaged))
	})

	t.Run("managed workflow", func(t *testing.T) {
		err := checkWorkflowsExportable([]string{"authored", "system-security-alert-analysis"}, unmanaged)
		require.ErrorContains(t, err, "workflow system-security-alert-analysis not found among the workflows authored by users")
	})
}

func TestTransformExportedWorkflows(t *testing.T) {
	t.Run("file names", func(t *testing.T) {
		files, err := transformExportedWorkflows("my_pkg", []kibana.ExportedWorkflow{
			{ID: "authored", YAML: testWorkflowYAML("Authored Workflow")},
		})
		require.NoError(t, err)
		require.Len(t, files, 1)
		assert.Equal(t, "my_pkg-authored-workflow.yml", files[0].Name)
	})

	t.Run("same file name", func(t *testing.T) {
		_, err := transformExportedWorkflows("my_pkg", []kibana.ExportedWorkflow{
			{ID: "first", YAML: testWorkflowYAML("Alert Triage")},
			{ID: "second", YAML: testWorkflowYAML("alert triage")},
		})
		require.ErrorContains(t, err, "workflows first and second would be exported to the same file my_pkg-alert-triage.yml")
	})

	t.Run("invalid workflow", func(t *testing.T) {
		_, err := transformExportedWorkflows("my_pkg", []kibana.ExportedWorkflow{
			{ID: "invalid", YAML: "name: [unclosed\n"},
		})
		require.ErrorContains(t, err, "transforming workflow invalid failed")
	})
}
