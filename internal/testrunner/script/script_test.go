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

func TestCheckStackConstraint(t *testing.T) {
	tests := []struct {
		name       string
		version    string
		constraint string
		want       bool
		wantErr    string
	}{
		{name: "empty version returns false", version: "", constraint: ">=9.5.0", want: false},
		{name: "release satisfies >=", version: "9.5.0", constraint: ">=9.5.0", want: true},
		{name: "snapshot satisfies >=", version: "9.5.0-SNAPSHOT", constraint: ">=9.5.0", want: true},
		{name: "older version fails >=", version: "9.4.3", constraint: ">=9.5.0", want: false},
		{name: "older snapshot fails >=", version: "9.4.3-SNAPSHOT", constraint: ">=9.5.0", want: false},
		{name: "range constraint", version: "9.5.1", constraint: ">=9.5.0,<10.0.0", want: true},
		{name: "outside range", version: "10.0.0", constraint: ">=9.5.0,<10.0.0", want: false},
		{name: "invalid constraint", version: "9.5.0", constraint: "not-valid", wantErr: "invalid stack_constraint"},
		{name: "invalid version", version: "bad", constraint: ">=9.5.0", wantErr: "parsing stack version"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := checkStackConstraint(tt.version, tt.constraint)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got, "checkStackConstraint(%q, %q)", tt.version, tt.constraint)
		})
	}
}
