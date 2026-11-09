// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/elastic-package/internal/export"
	"github.com/elastic/elastic-package/internal/kibana"
)

const (
	testFleetWorkflowID      = "system-fleet-package-workflow-37a8eec1ce19687d-8ed92b542831a71c-my-pkg-alert-triage"
	testOtherFleetWorkflowID = "system-fleet-package-workflow-37a8eec1ce19687d-0123456789abcdef-other-alert-triage"
)

func testWorkflowYAML(name string) string {
	return "name: " + name + "\nenabled: true\ntriggers:\n  - type: manual\nsteps:\n  - name: log\n    type: console\n"
}

func TestSelectableWorkflows(t *testing.T) {
	workflows := []kibana.Workflow{
		{ID: "authored", Name: "Authored"},
		{ID: testFleetWorkflowID, Name: "Alert triage", Managed: true},
		{ID: testOtherFleetWorkflowID, Name: "Other alert triage", Managed: true},
		{ID: "system-security-alert-analysis", Name: "Security alert analysis", Managed: true},
	}

	t.Run("without installed workflows", func(t *testing.T) {
		selectable := selectableWorkflows(workflows, nil)
		assert.Equal(t, []kibana.Workflow{workflows[0]}, selectable)
	})

	t.Run("with workflows installed for the package", func(t *testing.T) {
		selectable := selectableWorkflows(workflows, map[string]bool{testFleetWorkflowID: true})
		assert.Equal(t, []kibana.Workflow{workflows[0], workflows[1]}, selectable)
	})
}

func TestNoWorkflowsFoundMessage(t *testing.T) {
	assert.Equal(t, "No workflows were found in Kibana.", noWorkflowsFoundMessage(true))
	assert.Contains(t, noWorkflowsFoundMessage(false), "--managed")
}

func TestCheckWorkflowsExportable(t *testing.T) {
	unmanaged := []kibana.Workflow{{ID: "authored", Name: "Authored"}}
	installed := map[string]bool{testFleetWorkflowID: true}

	cases := []struct {
		title       string
		ids         []string
		installed   map[string]bool
		expectedErr string
	}{
		{title: "unmanaged workflows", ids: []string{"authored"}},
		{title: "installed for the package", ids: []string{"authored", testFleetWorkflowID}, installed: installed},
		{
			title:       "installed for another package",
			ids:         []string{testFleetWorkflowID, testOtherFleetWorkflowID},
			installed:   installed,
			expectedErr: "workflow " + testOtherFleetWorkflowID + " not found among the workflows authored by users, nor among the ones installed by Fleet for package my_pkg",
		},
		{
			title:       "package not installed",
			ids:         []string{testFleetWorkflowID},
			expectedErr: "workflow " + testFleetWorkflowID + " not found",
		},
		{
			title:       "system managed workflow",
			ids:         []string{"authored", "system-security-alert-analysis"},
			installed:   installed,
			expectedErr: "workflow system-security-alert-analysis not found",
		},
	}

	for _, c := range cases {
		t.Run(c.title, func(t *testing.T) {
			err := checkWorkflowsExportable(c.ids, unmanaged, c.installed, "my_pkg")
			if c.expectedErr != "" {
				require.ErrorContains(t, err, c.expectedErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestTransformExportedWorkflows(t *testing.T) {
	t.Run("file names", func(t *testing.T) {
		files, err := transformExportedWorkflows("my_pkg", []kibana.ExportedWorkflow{
			{ID: "authored", YAML: testWorkflowYAML("Authored Workflow")},
			{ID: testFleetWorkflowID, YAML: testWorkflowYAML("Alert Triage (renamed)")},
		})
		require.NoError(t, err)
		require.Len(t, files, 2)
		assert.Equal(t, "my_pkg-authored-workflow.yml", files[0].Name)
		assert.Equal(t, "my_pkg-alert-triage.yml", files[1].Name)
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
			{ID: "invalid", YAML: "name: Invalid\n"},
		})
		require.ErrorContains(t, err, "transforming workflow invalid failed")
	})
}

func TestCheckWorkflowFilesNotExist(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "my_pkg-existing.yml"), []byte("name: existing\n"), 0644))

	t.Run("new files", func(t *testing.T) {
		err := checkWorkflowFilesNotExist(dir, []*export.WorkflowFile{{Name: "my_pkg-new.yml"}})
		require.NoError(t, err)
	})

	t.Run("directory does not exist", func(t *testing.T) {
		err := checkWorkflowFilesNotExist(filepath.Join(dir, "missing"), []*export.WorkflowFile{{Name: "my_pkg-new.yml"}})
		require.NoError(t, err)
	})

	t.Run("existing file", func(t *testing.T) {
		err := checkWorkflowFilesNotExist(dir, []*export.WorkflowFile{{Name: "my_pkg-new.yml"}, {Name: "my_pkg-existing.yml"}})
		require.ErrorContains(t, err, "my_pkg-existing.yml already exists, use --force to overwrite it")
	})
}
