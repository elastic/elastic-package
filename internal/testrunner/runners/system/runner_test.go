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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/elastic-package/internal/elasticsearch"
)

func TestBuildLogsDBColumnarTemplatePayload(t *testing.T) {
	t.Run("creates payload when template is missing", func(t *testing.T) {
		payload, err := buildLogsDBColumnarTemplatePayload(nil, false, nil, nil)
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
		payload, err := buildLogsDBColumnarTemplatePayload(current, true, nil, nil)
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

	t.Run("merges doc_values property overrides and dynamic templates", func(t *testing.T) {
		current := []byte(`{
			"template": {
				"settings": {
					"index": {
						"mode": "logsdb_columnar"
					}
				},
				"mappings": {
					"dynamic_templates": [
						{"existing": {"path_match": "foo", "mapping": {"type": "keyword"}}}
					]
				}
			}
		}`)
		overrides := map[string]map[string]any{
			"event.original": {
				"type":       "keyword",
				"index":      false,
				"doc_values": true,
			},
			"doppel.darkweb.cred_leaks_password": {
				"type":       "keyword",
				"index":      false,
				"doc_values": true,
			},
		}
		payload, err := buildLogsDBColumnarTemplatePayload(current, true, overrides, logsDBColumnarDocValuesDynamicTemplates)
		require.NoError(t, err)

		decoded := map[string]any{}
		require.NoError(t, json.Unmarshal(payload, &decoded))
		mappings := decoded["template"].(map[string]any)["mappings"].(map[string]any)

		properties := mappings["properties"].(map[string]any)
		event := properties["event"].(map[string]any)["properties"].(map[string]any)
		original := event["original"].(map[string]any)
		assert.Equal(t, true, original["doc_values"])
		assert.Equal(t, false, original["index"])

		doppel := properties["doppel"].(map[string]any)["properties"].(map[string]any)
		darkweb := doppel["darkweb"].(map[string]any)["properties"].(map[string]any)
		password := darkweb["cred_leaks_password"].(map[string]any)
		assert.Equal(t, true, password["doc_values"])

		dynamicTemplates := mappings["dynamic_templates"].([]any)
		require.GreaterOrEqual(t, len(dynamicTemplates), 3)
		first := dynamicTemplates[0].(map[string]any)
		_, hasWorkaround := first["event_original_logsdb_columnar_workaround"]
		assert.True(t, hasWorkaround, "workaround dynamic template should be prepended")
		last := dynamicTemplates[len(dynamicTemplates)-1].(map[string]any)
		_, hasExisting := last["existing"]
		assert.True(t, hasExisting, "existing dynamic templates should be preserved after workarounds")
	})
}

func TestCollectDocValuesDisabledFieldOverrides(t *testing.T) {
	packageTemplate := []byte(`{
		"template": {
			"mappings": {
				"properties": {
					"event": {
						"properties": {
							"original": {
								"type": "keyword",
								"index": false,
								"doc_values": false
							},
							"category": {
								"type": "keyword"
							}
						}
					},
					"doppel": {
						"properties": {
							"darkweb": {
								"properties": {
									"cred_leaks_password": {
										"type": "keyword",
										"index": false,
										"doc_values": false
									}
								}
							}
						}
					}
				}
			}
		}
	}`)

	overrides := collectDocValuesDisabledFieldOverrides(packageTemplate)
	require.Len(t, overrides, 2)
	assert.Equal(t, true, overrides["event.original"]["doc_values"])
	assert.Equal(t, false, overrides["event.original"]["index"])
	assert.Equal(t, true, overrides["doppel.darkweb.cred_leaks_password"]["doc_values"])
	_, hasCategory := overrides["event.category"]
	assert.False(t, hasCategory, "fields without doc_values:false should not be overridden")
}

func TestPackageComponentTemplateName(t *testing.T) {
	assert.Equal(t, "logs-doppel.alerts@package", packageComponentTemplateName("logs-doppel.alerts@custom"))
	assert.Equal(t, "logs@package", packageComponentTemplateName("logs@custom"))
	assert.Equal(t, "", packageComponentTemplateName("logs-foo"))
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
		case req.Method == http.MethodGet && req.URL.Path == "/_component_template/logs@package":
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
	assert.Equal(t, true, eventOriginalDocValuesFromPayload(t, putBodies[0]))
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
		case req.Method == http.MethodGet && req.URL.Path == "/_component_template/logs@package":
			w.WriteHeader(http.StatusNotFound)
			_, _ = io.WriteString(w, `{"status":404}`)
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
	assert.Equal(t, true, eventOriginalDocValuesFromPayload(t, putBodies[0]))

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

func TestEnsureLogsDBColumnarTemplateWithPackageDocValuesOverrides(t *testing.T) {
	var putBodies [][]byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("X-Elastic-Product", "Elasticsearch")
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/":
			_, _ = io.WriteString(w, `{"version":{"number":"9.5.0-SNAPSHOT"}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/_capabilities":
			_, _ = io.WriteString(w, `{"supported":true}`)
		case req.Method == http.MethodGet && req.URL.Path == "/_component_template/logs-doppel.alerts@custom":
			w.WriteHeader(http.StatusNotFound)
			_, _ = io.WriteString(w, `{"status":404}`)
		case req.Method == http.MethodGet && req.URL.Path == "/_component_template/logs-doppel.alerts@package":
			_, _ = io.WriteString(w, `{
				"component_templates":[{
					"name":"logs-doppel.alerts@package",
					"component_template":{
						"template":{
							"mappings":{
								"properties":{
									"doppel":{
										"properties":{
											"darkweb":{
												"properties":{
													"cred_leaks_password":{
														"type":"keyword",
														"index":false,
														"doc_values":false
													}
												}
											}
										}
									}
								}
							}
						}
					}
				}]
			}`)
		case req.Method == http.MethodPut && req.URL.Path == "/_component_template/logs-doppel.alerts@custom":
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

	packageRoot := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(packageRoot, "manifest.yml"), []byte(`
format_version: 3.3.2
name: doppel
title: Doppel
version: 0.0.1
description: test
type: integration
conditions:
  kibana:
    version: "^8.0.0"
owner:
  github: elastic/security-service-integrations
`), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(packageRoot, "data_stream", "alerts"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(packageRoot, "data_stream", "alerts", "manifest.yml"), []byte(`
title: Alerts
type: logs
`), 0o644))

	r := &runner{
		esAPI:                   client.API,
		esClient:                client,
		packageRoot:             packageRoot,
		dataStreams:             []string{"alerts"},
		logsDBColumnarStatePath: filepath.Join(t.TempDir(), "logsdb-columnar-state.json"),
	}

	err = r.ensureLogsDBColumnarTemplate(t.Context())
	require.NoError(t, err)
	require.Len(t, putBodies, 1)

	decoded := map[string]any{}
	require.NoError(t, json.Unmarshal(putBodies[0], &decoded))
	assert.Equal(t, logsDBColumnarIndexMode, indexModeFromPayload(t, putBodies[0]))
	assert.Equal(t, true, eventOriginalDocValuesFromPayload(t, putBodies[0]))

	properties := decoded["template"].(map[string]any)["mappings"].(map[string]any)["properties"].(map[string]any)
	password := properties["doppel"].(map[string]any)["properties"].(map[string]any)["darkweb"].(map[string]any)["properties"].(map[string]any)["cred_leaks_password"].(map[string]any)
	assert.Equal(t, true, password["doc_values"])

	dynamicTemplates := decoded["template"].(map[string]any)["mappings"].(map[string]any)["dynamic_templates"].([]any)
	require.NotEmpty(t, dynamicTemplates)
	first := dynamicTemplates[0].(map[string]any)
	_, ok := first["event_original_logsdb_columnar_workaround"]
	assert.True(t, ok)
}

func TestSanitizeComponentTemplateForPut(t *testing.T) {
	original := []byte(`{
		"template": {"mappings": {"subobjects": false}},
		"_meta": {"managed": true},
		"version": 1,
		"created_date_millis": 1784647677960,
		"modified_date_millis": 1784647677960,
		"created_date": "2026-01-01T00:00:00.000Z",
		"modified_date": "2026-01-01T00:00:00.000Z"
	}`)

	sanitized, err := sanitizeComponentTemplateForPut(original)
	require.NoError(t, err)

	decoded := map[string]any{}
	require.NoError(t, json.Unmarshal(sanitized, &decoded))
	assert.Equal(t, false, decoded["template"].(map[string]any)["mappings"].(map[string]any)["subobjects"])
	assert.Equal(t, map[string]any{"managed": true}, decoded["_meta"])
	assert.EqualValues(t, 1, decoded["version"])
	_, hasCreatedMillis := decoded["created_date_millis"]
	_, hasModifiedMillis := decoded["modified_date_millis"]
	_, hasCreated := decoded["created_date"]
	_, hasModified := decoded["modified_date"]
	assert.False(t, hasCreatedMillis)
	assert.False(t, hasModifiedMillis)
	assert.False(t, hasCreated)
	assert.False(t, hasModified)
}

func TestStripSubobjectsFromComponentTemplate(t *testing.T) {
	original := []byte(`{
		"template": {
			"mappings": {
				"subobjects": false,
				"properties": {
					"host": {
						"type": "object",
						"subobjects": false,
						"properties": {
							"name": {"type": "keyword"}
						}
					},
					"message": {"type": "match_only_text"}
				}
			}
		}
	}`)

	stripped, changed, err := stripSubobjectsFromComponentTemplate(original)
	require.NoError(t, err)
	assert.True(t, changed)

	decoded := map[string]any{}
	require.NoError(t, json.Unmarshal(stripped, &decoded))
	mappings := decoded["template"].(map[string]any)["mappings"].(map[string]any)
	_, hasRootSubobjects := mappings["subobjects"]
	assert.False(t, hasRootSubobjects)
	host := mappings["properties"].(map[string]any)["host"].(map[string]any)
	_, hasHostSubobjects := host["subobjects"]
	assert.False(t, hasHostSubobjects)
	assert.Equal(t, "keyword", host["properties"].(map[string]any)["name"].(map[string]any)["type"])

	unchanged, changedAgain, err := stripSubobjectsFromComponentTemplate(stripped)
	require.NoError(t, err)
	assert.False(t, changedAgain)
	assert.JSONEq(t, string(stripped), string(unchanged))
}

func TestEnsureLogsDBColumnarTemplateStripsPackageSubobjects(t *testing.T) {
	originalPackage := `{
		"template":{"mappings":{"subobjects":false,"properties":{"message":{"type":"keyword"}}}},
		"created_date_millis":1784647677960,
		"modified_date_millis":1784647677960
	}`
	packagePuts := map[string][][]byte{}
	var customPuts [][]byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("X-Elastic-Product", "Elasticsearch")
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/":
			_, _ = io.WriteString(w, `{"version":{"number":"9.5.0-SNAPSHOT"}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/_capabilities":
			_, _ = io.WriteString(w, `{"supported":true}`)
		case req.Method == http.MethodGet && req.URL.Path == "/_component_template/logs-doppler.activity@custom":
			w.WriteHeader(http.StatusNotFound)
			_, _ = io.WriteString(w, `{"status":404}`)
		case req.Method == http.MethodGet && (req.URL.Path == "/_component_template/logs-doppler.activity@package" ||
			req.URL.Path == "/_component_template/logs-doppler.secret_read@package"):
			name := strings.TrimPrefix(req.URL.Path, "/_component_template/")
			_, _ = io.WriteString(w, fmt.Sprintf(`{
				"component_templates":[{
					"name":%q,
					"component_template":%s
				}]
			}`, name, originalPackage))
		case req.Method == http.MethodPut && (req.URL.Path == "/_component_template/logs-doppler.activity@package" ||
			req.URL.Path == "/_component_template/logs-doppler.secret_read@package"):
			name := strings.TrimPrefix(req.URL.Path, "/_component_template/")
			body, err := io.ReadAll(req.Body)
			require.NoError(t, err)
			packagePuts[name] = append(packagePuts[name], body)
			_, _ = io.WriteString(w, `{"acknowledged":true}`)
		case req.Method == http.MethodPut && req.URL.Path == "/_component_template/logs-doppler.activity@custom":
			body, err := io.ReadAll(req.Body)
			require.NoError(t, err)
			customPuts = append(customPuts, body)
			_, _ = io.WriteString(w, `{"acknowledged":true}`)
		case req.Method == http.MethodDelete && req.URL.Path == "/_component_template/logs-doppler.activity@custom":
			_, _ = io.WriteString(w, `{"acknowledged":true}`)
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
	}))
	defer server.Close()

	client, err := elasticsearch.NewClient(elasticsearch.OptionWithAddress(server.URL))
	require.NoError(t, err)

	packageRoot := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(packageRoot, "manifest.yml"), []byte(`
format_version: 3.3.2
name: doppler
title: Doppler
version: 0.0.1
description: test
type: integration
conditions:
  kibana:
    version: "^8.0.0"
owner:
  github: elastic/security-service-integrations
`), 0o644))
	for _, stream := range []string{"activity", "secret_read"} {
		require.NoError(t, os.MkdirAll(filepath.Join(packageRoot, "data_stream", stream), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(packageRoot, "data_stream", stream, "manifest.yml"), []byte(fmt.Sprintf(`
title: %s
type: logs
`, stream)), 0o644))
	}

	r := &runner{
		esAPI:                   client.API,
		esClient:                client,
		packageRoot:             packageRoot,
		dataStreams:             []string{"activity"},
		logsDBColumnarStatePath: filepath.Join(t.TempDir(), "logsdb-columnar-state.json"),
	}

	err = r.ensureLogsDBColumnarTemplate(t.Context())
	require.NoError(t, err)
	require.Len(t, packagePuts["logs-doppler.activity@package"], 1, "selected stream package template should be rewritten")
	require.Len(t, packagePuts["logs-doppler.secret_read@package"], 1, "sibling stream package template should also be stripped")
	require.Len(t, customPuts, 1, "columnar @custom should only be configured for the selected stream")

	for name, puts := range packagePuts {
		strippedPackage := map[string]any{}
		require.NoError(t, json.Unmarshal(puts[0], &strippedPackage), name)
		mappings := strippedPackage["template"].(map[string]any)["mappings"].(map[string]any)
		_, hasSubobjects := mappings["subobjects"]
		assert.False(t, hasSubobjects, "%s: subobjects should be stripped before enabling columnar mode", name)
		_, hasCreatedMillis := strippedPackage["created_date_millis"]
		_, hasModifiedMillis := strippedPackage["modified_date_millis"]
		assert.False(t, hasCreatedMillis, "%s: system-managed created_date_millis must not be sent on put", name)
		assert.False(t, hasModifiedMillis, "%s: system-managed modified_date_millis must not be sent on put", name)
	}

	err = r.restoreLogsDBColumnarTemplate(t.Context())
	require.NoError(t, err)
	require.Len(t, packagePuts["logs-doppler.activity@package"], 2, "original activity package template should be restored after custom")
	require.Len(t, packagePuts["logs-doppler.secret_read@package"], 2, "original sibling package template should be restored")
	for name, puts := range packagePuts {
		restoredPackage := map[string]any{}
		require.NoError(t, json.Unmarshal(puts[1], &restoredPackage), name)
		assert.Equal(t, false, restoredPackage["template"].(map[string]any)["mappings"].(map[string]any)["subobjects"], name)
		_, hasCreatedMillis := restoredPackage["created_date_millis"]
		_, hasModifiedMillis := restoredPackage["modified_date_millis"]
		assert.False(t, hasCreatedMillis, name)
		assert.False(t, hasModifiedMillis, name)
	}
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

func eventOriginalDocValuesFromPayload(t *testing.T, payload []byte) bool {
	t.Helper()
	decoded := map[string]any{}
	require.NoError(t, json.Unmarshal(payload, &decoded))
	properties := decoded["template"].(map[string]any)["mappings"].(map[string]any)["properties"].(map[string]any)
	return properties["event"].(map[string]any)["properties"].(map[string]any)["original"].(map[string]any)["doc_values"].(bool)
}
