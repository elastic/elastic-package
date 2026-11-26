// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/elastic-package/internal/kibana"
)

const testInstalledWorkflowID = "system-fleet-package-workflow-37a8eec1ce19687d-8ed92b542831a71c-my-pkg-alert-triage"

func TestEditableWorkflowID(t *testing.T) {
	cases := []struct {
		title       string
		installedID string
		expected    string
		expectedErr bool
	}{
		{title: "installed by Fleet", installedID: testInstalledWorkflowID, expected: "my-pkg-alert-triage"},
		{title: "single segment file base", installedID: "system-fleet-package-workflow-37a8eec1ce19687d-8ed92b542831a71c-triage", expected: "triage"},
		{title: "missing hash segment, hexadecimal file base", installedID: "system-fleet-package-workflow-37a8eec1ce19687d-add-alert", expectedErr: true},
		{title: "non-hex hash", installedID: "system-fleet-package-workflow-default-8ed92b542831a71c-my-pkg-alert-triage", expectedErr: true},
		{title: "Kibana system workflow", installedID: "system-security-alert-analysis", expectedErr: true},
		{title: "workflow authored by users", installedID: "my-pkg-alert-triage", expectedErr: true},
		{title: "missing hash segment", installedID: "system-fleet-package-workflow-37a8eec1ce19687d-my-pkg-alert-triage", expectedErr: true},
		{title: "missing file base", installedID: "system-fleet-package-workflow-37a8eec1ce19687d-8ed92b542831a71c", expectedErr: true},
		{title: "empty file base", installedID: "system-fleet-package-workflow-37a8eec1ce19687d-8ed92b542831a71c-", expectedErr: true},
	}

	for _, c := range cases {
		t.Run(c.title, func(t *testing.T) {
			id, err := editableWorkflowID(c.installedID)
			if c.expectedErr {
				require.ErrorContains(t, err, "was not installed by Fleet from a package")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, c.expected, id)
		})
	}
}

type fakeWorkflowsKibana struct {
	t         *testing.T
	workflows map[string]string
	created   map[string]string
	createErr string
}

func (f *fakeWorkflowsKibana) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.URL.Path == kibana.StatusAPI:
		fmt.Fprint(w, `{"version":{"number":"9.7.0"}}`)
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/workflows/workflow/"):
		id := strings.TrimPrefix(r.URL.Path, "/api/workflows/workflow/")
		yaml, found := f.workflows[id]
		if !found {
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprint(w, `{"message":"Workflow not found"}`)
			return
		}
		require.NoError(f.t, json.NewEncoder(w).Encode(map[string]string{"id": id, "name": "Alert Triage", "yaml": yaml}))
	case r.Method == http.MethodPost && r.URL.Path == "/api/workflows/export":
		var req struct {
			IDs []string `json:"ids"`
		}
		require.NoError(f.t, json.NewDecoder(r.Body).Decode(&req))
		var entries []map[string]string
		for _, id := range req.IDs {
			if yaml, found := f.workflows[id]; found {
				entries = append(entries, map[string]string{"id": id, "yaml": yaml})
			}
		}
		require.NoError(f.t, json.NewEncoder(w).Encode(map[string]any{"entries": entries}))
	case r.Method == http.MethodPost && r.URL.Path == "/api/workflows/workflow":
		assert.Empty(f.t, r.URL.RawQuery, "overwrite must not be used")
		if f.createErr != "" {
			w.WriteHeader(http.StatusConflict)
			fmt.Fprint(w, f.createErr)
			return
		}
		var req struct {
			ID   string `json:"id"`
			YAML string `json:"yaml"`
		}
		require.NoError(f.t, json.NewDecoder(r.Body).Decode(&req))
		f.created[req.ID] = req.YAML
		require.NoError(f.t, json.NewEncoder(w).Encode(map[string]string{"id": req.ID, "name": "Alert Triage"}))
	default:
		f.t.Errorf("unexpected request %s %s", r.Method, r.URL)
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func newFakeWorkflowsKibana(t *testing.T, workflows map[string]string) (*fakeWorkflowsKibana, *kibana.Client) {
	t.Helper()
	fake := &fakeWorkflowsKibana{t: t, workflows: workflows, created: map[string]string{}}
	server := httptest.NewServer(fake)
	t.Cleanup(server.Close)

	client, err := kibana.NewClient(kibana.Address(server.URL), kibana.RetryMax(0))
	require.NoError(t, err)
	return fake, client
}

func TestMakeWorkflowEditable(t *testing.T) {
	const installedYAML = "name: Alert Triage\nenabled: false\ntriggers:\n  - type: manual\nsteps:\n  - name: log\n    type: console\n"

	t.Run("creates an editable copy", func(t *testing.T) {
		fake, client := newFakeWorkflowsKibana(t, map[string]string{testInstalledWorkflowID: installedYAML})

		id, created, err := makeWorkflowEditable(t.Context(), client, testInstalledWorkflowID)
		require.NoError(t, err)
		assert.Equal(t, "my-pkg-alert-triage", id)
		assert.True(t, created)
		assert.Equal(t, map[string]string{"my-pkg-alert-triage": installedYAML}, fake.created)
	})

	t.Run("leaves an existing copy untouched", func(t *testing.T) {
		fake, client := newFakeWorkflowsKibana(t, map[string]string{
			testInstalledWorkflowID: installedYAML,
			"my-pkg-alert-triage":   "name: Alert Triage\nenabled: true\n",
		})

		id, created, err := makeWorkflowEditable(t.Context(), client, testInstalledWorkflowID)
		require.NoError(t, err)
		assert.Equal(t, "my-pkg-alert-triage", id)
		assert.False(t, created)
		assert.Empty(t, fake.created)
	})

	t.Run("ID taken", func(t *testing.T) {
		fake, client := newFakeWorkflowsKibana(t, map[string]string{testInstalledWorkflowID: installedYAML})
		fake.createErr = `{"message":"Workflow with id 'my-pkg-alert-triage' already exists"}`

		_, _, err := makeWorkflowEditable(t.Context(), client, testInstalledWorkflowID)
		require.ErrorContains(t, err, "Workflow with id 'my-pkg-alert-triage' already exists")
	})

	t.Run("installed workflow not found", func(t *testing.T) {
		_, client := newFakeWorkflowsKibana(t, map[string]string{})

		_, _, err := makeWorkflowEditable(t.Context(), client, testInstalledWorkflowID)
		require.ErrorContains(t, err, "not found")
	})

	t.Run("not installed by Fleet", func(t *testing.T) {
		_, client := newFakeWorkflowsKibana(t, map[string]string{})

		_, _, err := makeWorkflowEditable(t.Context(), client, "system-security-alert-analysis")
		require.ErrorContains(t, err, "was not installed by Fleet from a package")
	})
}

func TestWorkflowURLs(t *testing.T) {
	urls, err := workflowURLs("http://localhost:5601/abc", []string{"my-pkg-alert-triage", "my-pkg-report"})
	require.NoError(t, err)
	assert.Equal(t, "\nhttp://localhost:5601/abc/app/workflows/my-pkg-alert-triage\nhttp://localhost:5601/abc/app/workflows/my-pkg-report", urls)
}
