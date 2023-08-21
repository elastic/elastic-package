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
	"github.com/elastic/elastic-package/internal/stack"
)

// NewClient returns a client for a testing http server that uses prerecorded
// responses. If responses are not found, it forwards the query to the server started by
// elastic-package stack, and records the response.
// Responses are recorded in the directory indicated by serverDataDir.
func NewClient(t *testing.T, serverDataDir string) *elasticsearch.Client {
	server := testElasticsearchServer(t, serverDataDir)
	t.Cleanup(func() { server.Close() })

	client, err := stack.NewElasticsearchClient(
		elasticsearch.OptionWithAddress(server.URL),
	)
	require.NoError(t, err)

	return client
}

func testElasticsearchServer(t *testing.T, mockServerDir string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Log(r.Method, r.URL.String())
		f := filepath.Join(mockServerDir, pathForURL(r.URL.String()))
		if _, err := os.Stat(f); err != nil {
			recordRequest(t, r, f)
		}
		http.ServeFile(w, r, f)
	}))
}

var pathReplacer = strings.NewReplacer("/", "-", "*", "_")

func pathForURL(url string) string {
	clean := strings.Trim(url, "/")
	if len(clean) == 0 {
		return "root.json"
	}
	return pathReplacer.Replace(clean) + ".json"
}

func recordRequest(t *testing.T, r *http.Request, path string) {
	client, err := stack.NewElasticsearchClient()
	require.NoError(t, err)

	t.Logf("Recording %s in %s", r.URL.Path, path)
	req, err := http.NewRequest(r.Method, r.URL.Path, nil)
	require.NoError(t, err)

	resp, err := client.Perform(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	os.MkdirAll(filepath.Dir(path), 0755)
	f, err := os.Create(path)
	require.NoError(t, err)
	defer f.Close()

	_, err = io.Copy(f, resp.Body)
	require.NoError(t, err)
}
