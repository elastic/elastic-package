// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package kibana_test

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/elastic-package/internal/kibana"
	kibanatest "github.com/elastic/elastic-package/internal/kibana/test"
)

const testWorkflowName = "elastic-package test workflow"

func TestListWorkflows(t *testing.T) {
	client := kibanatest.NewClient(t, filepath.Join("testdata", "kibana-9-list-workflows"))
	id := preloadWorkflow(t, client)

	t.Run("unmanaged", func(t *testing.T) {
		workflows, err := client.ListWorkflows(t.Context(), kibana.ListWorkflowsOptions{Managed: kibana.WorkflowsUnmanagedOnly})
		require.NoError(t, err)

		assert.Contains(t, workflowIDs(workflows), id)
		for _, w := range workflows {
			assert.False(t, w.Managed, "unexpected managed workflow %s", w.ID)
		}
	})

	t.Run("all", func(t *testing.T) {
		workflows, err := client.ListWorkflows(t.Context(), kibana.ListWorkflowsOptions{Managed: kibana.WorkflowsManagedAll})
		require.NoError(t, err)

		assert.Contains(t, workflowIDs(workflows), id)
		assert.True(t, containsManagedWorkflow(workflows), "expected managed workflows to be listed")
	})

	t.Run("query", func(t *testing.T) {
		workflows, err := client.ListWorkflows(t.Context(), kibana.ListWorkflowsOptions{Query: testWorkflowName})
		require.NoError(t, err)

		assert.Contains(t, workflowIDs(workflows), id)
	})
}

func TestExportWorkflows(t *testing.T) {
	client := kibanatest.NewClient(t, filepath.Join("testdata", "kibana-9-export-workflows"))
	id := preloadWorkflow(t, client)

	t.Run("found", func(t *testing.T) {
		exported, err := client.ExportWorkflows(t.Context(), []string{id})
		require.NoError(t, err)

		require.Len(t, exported, 1)
		assert.Equal(t, id, exported[0].ID)
		assert.Contains(t, exported[0].YAML, "name: "+testWorkflowName)
	})

	t.Run("some missing", func(t *testing.T) {
		_, err := client.ExportWorkflows(t.Context(), []string{id, "missing"})
		require.ErrorContains(t, err, `workflow "missing" not found`)
	})

	t.Run("all missing", func(t *testing.T) {
		_, err := client.ExportWorkflows(t.Context(), []string{"missing"})
		require.ErrorContains(t, err, "API status code = 404")
	})
}

// preloadWorkflow creates an unmanaged workflow and returns its ID. Kibana derives the ID from the
// workflow name, adding a suffix if it is already taken, so the returned ID must be used.
func preloadWorkflow(t *testing.T, client *kibana.Client) string {
	t.Helper()
	yaml := "name: " + testWorkflowName + `
enabled: false
triggers:
  - type: manual
steps:
  - name: hello
    type: console
    with:
      message: hello
`
	body, err := json.Marshal(map[string]string{"yaml": yaml})
	require.NoError(t, err)

	statusCode, respBody, err := client.SendRequest(t.Context(), http.MethodPost, "/api/workflows/workflow", body)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, statusCode, string(respBody))

	var created struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.Unmarshal(respBody, &created))
	require.NotEmpty(t, created.ID)

	t.Cleanup(func() {
		statusCode, _, err := client.SendRequest(context.Background(), http.MethodDelete, "/api/workflows/workflow/"+created.ID, nil)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, statusCode)
	})

	return created.ID
}

func workflowIDs(workflows []kibana.Workflow) []string {
	ids := make([]string, 0, len(workflows))
	for _, w := range workflows {
		ids = append(ids, w.ID)
	}
	return ids
}

func containsManagedWorkflow(workflows []kibana.Workflow) bool {
	for _, w := range workflows {
		if w.Managed {
			return true
		}
	}
	return false
}
