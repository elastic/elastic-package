// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/elastic/elastic-package/internal/elasticsearch"
)

// ElasticsearchClient returns a client for a testing http server that uses prerecorded
// responses. If responses are not found, it forwards the query to the server started by
// elastic-package stack, and records the response.
// Responses are recorded in the directory indicated by serverDataDir.
func ElasticsearchClient(t *testing.T, serverDataDir string) *elasticsearch.API {
	server := testElasticsearchServerForExport(t, serverDataDir)
	t.Cleanup(func() { server.Close() })

	clientOptions := elasticsearch.DefaultClientOptionsFromEnv()
	clientOptions.Address = server.URL
	client, err := elasticsearch.ClientWithOptions(clientOptions)
	require.NoError(t, err)

	return client.API
}

func testElasticsearchServerForExport(t *testing.T, mockServerDir string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Log(r.Method, r.URL.String())
		f := filepath.Join(mockServerDir, pathForURL(r.URL.String()))
		if _, err := os.Stat(f); err != nil {
			recordRequest(t, r, f)
		}
		http.ServeFile(w, r, f)
	}))
}

func pathForURL(url string) string {
	clean := strings.Trim(url, "/")
	if len(clean) == 0 {
		return "root.json"
	}
	parts := strings.Split(clean, "/")
	return strings.Join(parts, "-") + ".json"
}

func recordRequest(t *testing.T, r *http.Request, path string) {
	options := elasticsearch.DefaultClientOptionsFromEnv()
	t.Logf("Recording %s in %s", options.Address+r.URL.Path, path)
	req, err := http.NewRequest(r.Method, options.Address+r.URL.Path, nil)
	require.NoError(t, err)

	if options.Username != "" && options.Password != "" {
		req.SetBasicAuth(options.Username, options.Password)
	}
	req.Host = options.Address
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	os.MkdirAll(filepath.Dir(path), 0755)
	f, err := os.Create(path)
	require.NoError(t, err)
	defer f.Close()

	_, err = io.Copy(f, resp.Body)
	require.NoError(t, err)
}
