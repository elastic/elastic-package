// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package kibana

import (
	"encoding/json"
	"fmt"
	"io"
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

func TestListWorkflows(t *testing.T) {
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

	workflows, err := client.ListWorkflows(t.Context(), ListWorkflowsOptions{Managed: WorkflowsManagedAll, Query: "alert"})
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
		assert.Equal(t, "all", r.URL.Query().Get("managed"))
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

func TestExportWorkflows(t *testing.T) {
	client := newWorkflowsTestClient(t, "9.7.0", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/workflows/export", r.URL.Path)
		assert.Equal(t, workflowsAPIVersion, r.Header.Get("elastic-api-version"))

		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		var req exportWorkflowsRequest
		require.NoError(t, json.Unmarshal(body, &req))

		var resp exportWorkflowsResponse
		for _, id := range req.IDs {
			if id == "missing" {
				continue
			}
			resp.Entries = append(resp.Entries, ExportedWorkflow{ID: id, YAML: "name: " + id + "\n"})
		}
		require.NoError(t, json.NewEncoder(w).Encode(resp))
	})

	t.Run("all found", func(t *testing.T) {
		exported, err := client.ExportWorkflows(t.Context(), []string{"a", "b"})
		require.NoError(t, err)
		assert.Equal(t, []ExportedWorkflow{
			{ID: "a", YAML: "name: a\n"},
			{ID: "b", YAML: "name: b\n"},
		}, exported)
	})

	t.Run("missing workflow", func(t *testing.T) {
		_, err := client.ExportWorkflows(t.Context(), []string{"a", "missing"})
		require.ErrorContains(t, err, `workflow "missing" not found`)
	})
}

func TestWorkflowsUnsupportedKibanaVersion(t *testing.T) {
	client := newWorkflowsTestClient(t, "9.3.0", func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request to %s", r.URL)
	})

	_, err := client.ListWorkflows(t.Context(), ListWorkflowsOptions{})
	require.ErrorContains(t, err, "minimum version is 9.4.0")

	_, err = client.ExportWorkflows(t.Context(), []string{"a"})
	require.ErrorContains(t, err, "minimum version is 9.4.0")
}

func TestFleetPackageWorkflowFileBase(t *testing.T) {
	cases := []struct {
		id       string
		expected string
		ok       bool
	}{
		{
			id:       "system-fleet-package-workflow-37a8eec1ce19687d-8ed92b542831a71c-good-workflow-integration-alert-triage",
			expected: "good-workflow-integration-alert-triage",
			ok:       true,
		},
		{id: "system-fleet-package-workflow-37a8eec1ce19687d-8ed92b542831a71c-", ok: false},
		{id: "system-fleet-package-workflow-37a8eec1ce19687d", ok: false},
		{id: "system-security-alert-analysis", ok: false},
		{id: "alert-triage", ok: false},
	}

	for _, c := range cases {
		t.Run(c.id, func(t *testing.T) {
			fileBase, ok := FleetPackageWorkflowFileBase(c.id)
			assert.Equal(t, c.ok, ok)
			assert.Equal(t, c.expected, fileBase)
		})
	}
}

func TestInstalledPackageWorkflowIDs(t *testing.T) {
	cases := []struct {
		title    string
		status   int
		body     string
		expected map[string]bool
	}{
		{
			title:  "installed",
			status: http.StatusOK,
			body: `{"item":{"name":"my_pkg","status":"installed","installationInfo":{"installed_kibana":[
				{"id":"system-fleet-package-workflow-a-b-my-pkg-alert-triage","type":"workflow"},
				{"id":"my_pkg-dashboard","type":"dashboard"}
			]}}}`,
			expected: map[string]bool{"system-fleet-package-workflow-a-b-my-pkg-alert-triage": true},
		},
		{
			title:    "not installed",
			status:   http.StatusOK,
			body:     `{"item":{"name":"my_pkg","status":"not_installed"}}`,
			expected: map[string]bool{},
		},
		{
			title:    "not found",
			status:   http.StatusNotFound,
			body:     `{"message":"not found"}`,
			expected: map[string]bool{},
		},
	}

	for _, c := range cases {
		t.Run(c.title, func(t *testing.T) {
			client := newWorkflowsTestClient(t, "9.7.0", func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/api/fleet/epm/packages/my_pkg", r.URL.Path)
				w.WriteHeader(c.status)
				fmt.Fprint(w, c.body)
			})

			ids, err := client.InstalledPackageWorkflowIDs(t.Context(), "my_pkg")
			require.NoError(t, err)
			assert.Equal(t, c.expected, ids)
		})
	}
}
