// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package script

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/elastic/elastic-package/internal/install"
	"github.com/elastic/elastic-package/internal/profile"
	"github.com/elastic/elastic-package/internal/stack"
	"github.com/elastic/elastic-package/internal/testrunner"
)

func TestRun_scriptsUseRunnerProfile(t *testing.T) {
	const (
		current  = "current"
		selected = "selected"
	)
	tests := []struct {
		name    string
		profile string // Profile passed to the runner, as with -p.
		want    string
	}{
		{name: "selected profile", profile: selected, want: selected},
		{name: "current profile when none is selected", want: current},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("ELASTIC_PACKAGE_DATA_HOME", filepath.Join(t.TempDir(), "data"))
			t.Setenv("GOTMPDIR", t.TempDir())
			t.Setenv(install.ProfileNameEnvVar, current)
			require.NoError(t, install.EnsureInstalled())
			for _, name := range []string{current, selected} {
				require.NoError(t, profile.CreateProfile(profile.Options{Name: name}))
			}

			opt := Options{Dir: t.TempDir()}
			if tt.profile != "" {
				prof, err := profile.LoadProfile(tt.profile)
				require.NoError(t, err)
				opt.Profile = prof
			}
			// cmpenv expands environment variables in its second file only.
			script := `cmpenv want got
-- got --
${PROFILE}
-- want --
` + tt.want + "\n"
			require.NoError(t, os.WriteFile(filepath.Join(opt.Dir, "profile.txt"), []byte(script), 0o600))

			var results []testrunner.TestResult
			require.NoError(t, Run(&results, io.Discard, opt))
			require.Len(t, results, 1)
			require.Empty(t, results[0].FailureMsg, results[0].FailureDetails)
			require.Empty(t, results[0].ErrorMsg)
		})
	}
}

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
