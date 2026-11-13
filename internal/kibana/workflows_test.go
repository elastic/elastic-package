// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package kibana

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newWorkflowsTestClient(t *testing.T, version string, handler http.HandlerFunc) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	client, err := NewClient(
		Address(server.URL),
		RetryMax(0),
		func(c *Client) {
			c.versionInfo = VersionInfo{Number: version}
			c.semver = semver.MustParse(version)
		},
	)
	require.NoError(t, err)
	return client
}

// TestListWorkflowsPagination uses a stub server because a real stack doesn't have enough workflows
// to span several pages. The API responses are covered by recorded tests in workflows_export_test.go.
func TestListWorkflowsPagination(t *testing.T) {
	const total = 150
	var requests []*http.Request
	client := newWorkflowsTestClient(t, "9.7.0-SNAPSHOT", func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r)
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		size, _ := strconv.Atoi(r.URL.Query().Get("size"))

		resp := listWorkflowsResponse{Page: page, Size: size, Total: total}
		for i := (page - 1) * size; i < min(page*size, total); i++ {
			resp.Results = append(resp.Results, Workflow{ID: fmt.Sprintf("workflow-%d", i), Name: fmt.Sprintf("Workflow %d", i)})
		}
		require.NoError(t, json.NewEncoder(w).Encode(resp))
	})

	workflows, err := client.ListWorkflows(t.Context(), ListWorkflowsOptions{Managed: WorkflowsUnmanagedOnly, Query: "alert"})
	require.NoError(t, err)

	assert.Len(t, workflows, total)
	assert.Equal(t, "workflow-0", workflows[0].ID)
	assert.Equal(t, "workflow-149", workflows[total-1].ID)

	require.Len(t, requests, 2)
	for i, r := range requests {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/api/workflows", r.URL.Path)
		assert.Equal(t, workflowsAPIVersion, r.Header.Get("elastic-api-version"))
		assert.Equal(t, strconv.Itoa(i+1), r.URL.Query().Get("page"))
		assert.Equal(t, "100", r.URL.Query().Get("size"))
		assert.Equal(t, "unmanaged", r.URL.Query().Get("managed"))
		assert.Equal(t, "alert", r.URL.Query().Get("query"))
	}
}

func TestListWorkflowsError(t *testing.T) {
	client := newWorkflowsTestClient(t, "9.7.0", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprint(w, `{"message":"forbidden"}`)
	})

	_, err := client.ListWorkflows(t.Context(), ListWorkflowsOptions{})
	require.ErrorContains(t, err, "API status code = 403")
}

func TestWorkflowsUnsupportedKibanaVersion(t *testing.T) {
	client := newWorkflowsTestClient(t, "9.4.6", func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request to %s", r.URL)
	})

	_, err := client.ListWorkflows(t.Context(), ListWorkflowsOptions{})
	require.ErrorContains(t, err, "minimum version is 9.5.0")

	_, err = client.ExportWorkflows(t.Context(), []string{"a"})
	require.ErrorContains(t, err, "minimum version is 9.5.0")
}

func TestGetWorkflow(t *testing.T) {
	t.Run("found", func(t *testing.T) {
		client := newWorkflowsTestClient(t, "9.7.0", func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method)
			assert.Equal(t, "/api/workflows/workflow/my-pkg-alert-triage", r.URL.Path)
			assert.Equal(t, workflowsAPIVersion, r.Header.Get("elastic-api-version"))
			fmt.Fprint(w, `{"id":"my-pkg-alert-triage","name":"Alert Triage","managed":false,"yaml":"name: Alert Triage\n"}`)
		})

		workflow, found, err := client.GetWorkflow(t.Context(), "my-pkg-alert-triage")
		require.NoError(t, err)
		assert.True(t, found)
		assert.Equal(t, Workflow{ID: "my-pkg-alert-triage", Name: "Alert Triage"}, workflow)
	})

	t.Run("not found", func(t *testing.T) {
		client := newWorkflowsTestClient(t, "9.7.0", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprint(w, `{"message":"Workflow not found"}`)
		})

		_, found, err := client.GetWorkflow(t.Context(), "missing")
		require.NoError(t, err)
		assert.False(t, found)
	})

	t.Run("error", func(t *testing.T) {
		client := newWorkflowsTestClient(t, "9.7.0", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			fmt.Fprint(w, `{"message":"forbidden"}`)
		})

		_, _, err := client.GetWorkflow(t.Context(), "my-pkg-alert-triage")
		require.ErrorContains(t, err, "API status code = 403")
	})
}

func TestCreateWorkflow(t *testing.T) {
	t.Run("created", func(t *testing.T) {
		client := newWorkflowsTestClient(t, "9.7.0", func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method)
			assert.Equal(t, "/api/workflows/workflow", r.URL.Path)
			assert.Empty(t, r.URL.RawQuery)
			assert.Equal(t, workflowsAPIVersion, r.Header.Get("elastic-api-version"))

			var req createWorkflowRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, createWorkflowRequest{ID: "my-pkg-alert-triage", YAML: "name: Alert Triage\n"}, req)
			fmt.Fprint(w, `{"id":"my-pkg-alert-triage","name":"Alert Triage","managed":false}`)
		})

		workflow, err := client.CreateWorkflow(t.Context(), "my-pkg-alert-triage", "name: Alert Triage\n")
		require.NoError(t, err)
		assert.Equal(t, Workflow{ID: "my-pkg-alert-triage", Name: "Alert Triage"}, workflow)
	})

	t.Run("conflict", func(t *testing.T) {
		client := newWorkflowsTestClient(t, "9.7.0", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusConflict)
			fmt.Fprint(w, `{"message":"Workflow with id 'my-pkg-alert-triage' already exists"}`)
		})

		_, err := client.CreateWorkflow(t.Context(), "my-pkg-alert-triage", "name: Alert Triage\n")
		require.ErrorContains(t, err, "already exists")
	})
}
