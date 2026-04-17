// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package script

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/elastic/elastic-package/internal/stack"
)

func TestRevisionsFromRegistry_searchOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search" {
			http.NotFound(w, r)
			return
		}
		_, err := w.Write([]byte("[]"))
		require.NoError(t, err)
	}))
	t.Cleanup(srv.Close)

	revs, err := revisionsFromRegistry(srv.URL, nil, "acme")
	require.NoError(t, err)
	require.Empty(t, revs)
}

func TestRevisionsFromRegistry_propagatesRegistryClientError(t *testing.T) {
	badCA := filepath.Join(t.TempDir(), "invalid-ca.pem")
	require.NoError(t, os.WriteFile(badCA, []byte("not a PEM certificate"), 0o600))
	t.Setenv(stack.CACertificateEnv, badCA)

	_, err := revisionsFromRegistry("https://epr.example", nil, "acme")
	require.Error(t, err)
	require.ErrorContains(t, err, "creating package registry client")
}
