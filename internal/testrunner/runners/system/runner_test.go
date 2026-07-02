// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package system

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/elastic-package/internal/elasticsearch"
	"github.com/elastic/elastic-package/internal/testrunner"
)

func TestBuildLogsDBColumnarTemplatePayload(t *testing.T) {
	t.Run("creates payload when template is missing", func(t *testing.T) {
		payload, err := buildLogsDBColumnarTemplatePayload(nil, false)
		require.NoError(t, err)

		mode := indexModeFromPayload(t, payload)
		assert.Equal(t, logsDBColumnarIndexMode, mode)
	})

	t.Run("merges with existing template settings", func(t *testing.T) {
		current := []byte(`{
			"template": {
				"settings": {
					"index": {
						"number_of_shards": "2"
					}
				}
			},
			"_meta": {
				"managed_by": "test"
			}
		}`)
		payload, err := buildLogsDBColumnarTemplatePayload(current, true)
		require.NoError(t, err)

		decoded := map[string]any{}
		require.NoError(t, json.Unmarshal(payload, &decoded))
		template := decoded["template"].(map[string]any)
		settings := template["settings"].(map[string]any)
		index := settings["index"].(map[string]any)
		assert.Equal(t, logsDBColumnarIndexMode, index["mode"])
		assert.Equal(t, "2", index["number_of_shards"])
		assert.Equal(t, "test", decoded["_meta"].(map[string]any)["managed_by"])
	})
}

func TestEnsureAndRestoreLogsDBColumnarTemplateWithoutExistingTemplate(t *testing.T) {
	var putBodies [][]byte
	deleteCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("X-Elastic-Product", "Elasticsearch")
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/":
			_, _ = io.WriteString(w, `{"version":{"number":"9.5.0-SNAPSHOT"}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/_capabilities":
			assert.Equal(t, http.MethodPut, req.URL.Query().Get("method"))
			assert.Equal(t, "/{index}", req.URL.Query().Get("path"))
			assert.Equal(t, "columnar_index_modes", req.URL.Query().Get("capabilities"))
			_, _ = io.WriteString(w, `{"supported":true}`)
		case req.Method == http.MethodGet && req.URL.Path == "/_component_template/logs@custom":
			w.WriteHeader(http.StatusNotFound)
			_, _ = io.WriteString(w, `{"status":404}`)
		case req.Method == http.MethodPut && req.URL.Path == "/_component_template/logs@custom":
			body, err := io.ReadAll(req.Body)
			require.NoError(t, err)
			putBodies = append(putBodies, body)
			_, _ = io.WriteString(w, `{"acknowledged":true}`)
		case req.Method == http.MethodDelete && req.URL.Path == "/_component_template/logs@custom":
			deleteCount++
			_, _ = io.WriteString(w, `{"acknowledged":true}`)
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
	}))
	defer server.Close()

	client, err := elasticsearch.NewClient(elasticsearch.OptionWithAddress(server.URL))
	require.NoError(t, err)

	statePath := filepath.Join(t.TempDir(), "logsdb-columnar-state.json")
	r := &runner{
		esAPI:                   client.API,
		esClient:                client,
		logsDBColumnarStatePath: statePath,
	}

	err = r.ensureLogsDBColumnarTemplate(t.Context())
	require.NoError(t, err)
	require.Len(t, putBodies, 1)
	assert.Equal(t, logsDBColumnarIndexMode, indexModeFromPayload(t, putBodies[0]))
	_, err = os.Stat(statePath)
	require.NoError(t, err)

	err = r.restoreLogsDBColumnarTemplate(t.Context())
	require.NoError(t, err)
	assert.Equal(t, 1, deleteCount)
	_, err = os.Stat(statePath)
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestEnsureAndRestoreLogsDBColumnarTemplateWithExistingTemplate(t *testing.T) {
	originalTemplate := `{"template":{"settings":{"index":{"number_of_shards":"2"}}}}`
	var putBodies [][]byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("X-Elastic-Product", "Elasticsearch")
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/":
			_, _ = io.WriteString(w, `{"version":{"number":"9.5.0-SNAPSHOT"}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/_capabilities":
			_, _ = io.WriteString(w, `{"supported":true}`)
		case req.Method == http.MethodGet && req.URL.Path == "/_component_template/logs@custom":
			_, _ = io.WriteString(w, fmt.Sprintf(`{
				"component_templates":[
					{"name":"logs@custom","component_template":%s}
				]
			}`, originalTemplate))
		case req.Method == http.MethodPut && req.URL.Path == "/_component_template/logs@custom":
			body, err := io.ReadAll(req.Body)
			require.NoError(t, err)
			putBodies = append(putBodies, body)
			_, _ = io.WriteString(w, `{"acknowledged":true}`)
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
	}))
	defer server.Close()

	client, err := elasticsearch.NewClient(elasticsearch.OptionWithAddress(server.URL))
	require.NoError(t, err)

	r := &runner{
		esAPI:                   client.API,
		esClient:                client,
		logsDBColumnarStatePath: filepath.Join(t.TempDir(), "logsdb-columnar-state.json"),
	}

	err = r.ensureLogsDBColumnarTemplate(context.Background())
	require.NoError(t, err)
	require.Len(t, putBodies, 1)
	assert.Equal(t, logsDBColumnarIndexMode, indexModeFromPayload(t, putBodies[0]))

	err = r.restoreLogsDBColumnarTemplate(context.Background())
	require.NoError(t, err)
	require.Len(t, putBodies, 2)

	restored := map[string]any{}
	require.NoError(t, json.Unmarshal(putBodies[1], &restored))
	settings := restored["template"].(map[string]any)["settings"].(map[string]any)
	index := settings["index"].(map[string]any)
	assert.Equal(t, "2", index["number_of_shards"])
	_, hasMode := index["mode"]
	assert.False(t, hasMode)
}

func TestHasNestedFieldsInDataStream(t *testing.T) {
	packageRoot := t.TempDir()

	err := writeFile(filepath.Join(packageRoot, "data_stream", "with_nested", "fields", "fields.yml"), `
- name: dns.answers
  type: nested
`)
	require.NoError(t, err)

	err = writeFile(filepath.Join(packageRoot, "data_stream", "without_nested", "fields", "fields.yml"), `
- name: message
  type: keyword
`)
	require.NoError(t, err)

	nested, err := hasNestedFieldsInDataStream(packageRoot, "with_nested")
	require.NoError(t, err)
	assert.True(t, nested)

	nested, err = hasNestedFieldsInDataStream(packageRoot, "without_nested")
	require.NoError(t, err)
	assert.False(t, nested)

	nested, err = hasNestedFieldsInDataStream(packageRoot, "missing")
	require.NoError(t, err)
	assert.False(t, nested)
}

func TestSkipReasonsForLogsDBColumnar(t *testing.T) {
	packageRoot := t.TempDir()
	require.NoError(t, writeFile(filepath.Join(packageRoot, "data_stream", "logs_stream", "manifest.yml"), "title: Logs\ntype: logs\n"))
	require.NoError(t, writeFile(filepath.Join(packageRoot, "data_stream", "logs_stream", "fields", "fields.yml"), `
- name: dns.answers
  type: nested
`))
	require.NoError(t, writeFile(filepath.Join(packageRoot, "data_stream", "metrics_stream", "manifest.yml"), "title: Metrics\ntype: metrics\n"))
	require.NoError(t, writeFile(filepath.Join(packageRoot, "data_stream", "metrics_stream", "fields", "fields.yml"), `
- name: host.name
  type: keyword
`))

	r := &runner{packageRoot: packageRoot}
	folders := []testrunner.TestFolder{
		{Path: "/tmp/logs", Package: "demo", DataStream: "logs_stream"},
		{Path: "/tmp/metrics", Package: "demo", DataStream: "metrics_stream"},
	}

	reasons, err := r.skipReasonsForLogsDBColumnar(folders)
	require.NoError(t, err)
	require.Len(t, reasons, 1)
	assert.Contains(t, reasons["/tmp/logs"], "does not support nested mappings")
}

func indexModeFromPayload(t *testing.T, payload []byte) string {
	t.Helper()
	decoded := map[string]any{}
	require.NoError(t, json.Unmarshal(payload, &decoded))
	template := decoded["template"].(map[string]any)
	settings := template["settings"].(map[string]any)
	index := settings["index"].(map[string]any)
	return index["mode"].(string)
}

func writeFile(path string, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0644)
}
