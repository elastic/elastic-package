// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package requiresupdates

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/stretchr/testify/require"

	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/registry"
)

func TestResolve(t *testing.T) {
	tests := []struct {
		name       string
		revisions  []packages.PackageManifest // nil = no registry server
		manifest   string
		prerelease bool
		check      func(t *testing.T, result *Result, err error, manifest string)
	}{
		{
			name: "bumps compatible dependency",
			revisions: []packages.PackageManifest{
				manifestRevision("0.2.0", "^9.4.0"),
				manifestRevision("0.3.0", "^9.4.0"),
			},
			manifest: `name: test_pkg
version: 1.0.0
type: integration
owner:
  github: elastic/integrations
conditions:
  kibana:
    version: "^9.4.0"
requires:
  input:
    - package: sql_input
      version: "0.2.0"
`,
			check: func(t *testing.T, result *Result, err error, _ string) {
				require.NoError(t, err)
				require.Equal(t, "test_pkg", result.Package)
				require.Equal(t, "elastic/integrations", result.CodeOwner)
				require.Len(t, result.Proposals, 1)
				require.Equal(t, "0.2.0", result.Proposals[0].Current)
				require.Equal(t, "0.3.0", result.Proposals[0].Proposed)
				require.Empty(t, result.Proposals[0].Warning)
			},
		},
		{
			name: "warns when newer dependency requires higher kibana",
			revisions: []packages.PackageManifest{
				manifestRevision("0.2.0", "^9.4.0"),
				manifestRevision("0.3.0", "^9.4.0"),
				manifestRevision("0.5.0", "^9.6.0"),
			},
			manifest: `name: test_pkg
version: 1.0.0
type: integration
conditions:
  kibana:
    version: ">=9.4.0,<9.6.0"
requires:
  input:
    - package: sql_input
      version: "0.2.0"
`,
			check: func(t *testing.T, result *Result, err error, _ string) {
				require.NoError(t, err)
				require.Len(t, result.Proposals, 1)
				require.Equal(t, "0.3.0", result.Proposals[0].Proposed)
				require.Contains(t, result.Proposals[0].Warning, "0.5.0")
				require.Contains(t, result.Proposals[0].Warning, "^9.6.0")
			},
		},
		{
			name: "apply round-trip updates manifest version",
			revisions: []packages.PackageManifest{
				manifestRevision("0.2.0", "^9.4.0"),
				manifestRevision("0.4.0", "^9.4.0"),
			},
			manifest: `name: test_pkg
version: 1.0.0
type: integration
conditions:
  kibana:
    version: "^9.4.0"
requires:
  input:
    - package: sql_input
      version: "0.2.0"
`,
			check: func(t *testing.T, result *Result, err error, manifest string) {
				require.NoError(t, err)
				require.Len(t, result.Proposals, 1)
				require.Equal(t, "0.4.0", result.Proposals[0].Proposed)

				updated, err := Apply([]byte(manifest), result.Proposals)
				require.NoError(t, err)
				dir := t.TempDir()
				require.NoError(t, os.WriteFile(filepath.Join(dir, "manifest.yml"), updated, 0o644))
				pkg, err := packages.ReadPackageManifestFromPackageRoot(dir)
				require.NoError(t, err)
				require.Equal(t, "0.4.0", pkg.Requires.Input[0].Version)
			},
		},
		{
			name: "skips non-integration package type",
			manifest: `name: test_input
version: 1.0.0
type: input
`,
			check: func(t *testing.T, result *Result, err error, _ string) {
				require.NoError(t, err)
				require.NotEmpty(t, result.SkipReason)
				require.Contains(t, result.SkipReason, "integration")
			},
		},
		{
			name: "skips integration without requires section",
			manifest: `name: test_integration
version: 1.0.0
type: integration
`,
			check: func(t *testing.T, result *Result, err error, _ string) {
				require.NoError(t, err)
				require.NotEmpty(t, result.SkipReason)
				require.Contains(t, result.SkipReason, "requires")
			},
		},
		{
			name: "warning only when all revisions require higher kibana",
			revisions: []packages.PackageManifest{
				manifestRevision("0.3.0", "^9.6.0"),
			},
			manifest: `name: test_pkg
version: 1.0.0
type: integration
conditions:
  kibana:
    version: ">=9.4.0,<9.6.0"
requires:
  input:
    - package: sql_input
      version: "0.2.0"
`,
			check: func(t *testing.T, result *Result, err error, _ string) {
				require.NoError(t, err)
				require.Len(t, result.Proposals, 1)
				p := result.Proposals[0]
				require.Equal(t, "0.2.0", p.Current)
				require.Empty(t, p.Proposed, "expected no proposed version when no compatible revision exists")
				require.NotEmpty(t, p.Warning, "expected a warning when only a higher-Kibana revision is available")
				require.Contains(t, p.Warning, "0.3.0")
				require.Contains(t, p.Warning, "^9.6.0")
			},
		},
		{
			name: "content dep exact pin bumped",
			revisions: []packages.PackageManifest{
				manifestRevision("0.2.0", "^9.4.0"),
				manifestRevision("0.3.0", "^9.4.0"),
			},
			manifest: `name: test_pkg
version: 1.0.0
type: integration
conditions:
  kibana:
    version: "^9.4.0"
requires:
  content:
    - package: sql_input
      version: "0.2.0"
`,
			check: func(t *testing.T, result *Result, err error, _ string) {
				require.NoError(t, err)
				require.Len(t, result.Proposals, 1)
				p := result.Proposals[0]
				require.Equal(t, ContentDependency, p.Kind)
				require.Equal(t, "0.2.0", p.Current)
				require.Equal(t, "0.3.0", p.Proposed)
				require.Empty(t, p.Warning)
			},
		},
		{
			name: "content dep constraint style bumps beyond current range",
			revisions: []packages.PackageManifest{
				manifestRevision("0.3.5", "^9.4.0"),
				manifestRevision("0.4.0", "^9.4.0"),
			},
			manifest: `name: test_pkg
version: 1.0.0
type: integration
conditions:
  kibana:
    version: "^9.4.0"
requires:
  content:
    - package: sql_input
      version: "^0.3.0"
`,
			check: func(t *testing.T, result *Result, err error, _ string) {
				require.NoError(t, err)
				require.Len(t, result.Proposals, 1)
				p := result.Proposals[0]
				require.Equal(t, ContentDependency, p.Kind)
				require.Equal(t, "^0.3.0", p.Current)
				require.Equal(t, "0.4.0", p.Proposed)
				require.Empty(t, p.Warning)
			},
		},
		{
			name: "content dep constraint style no update when all versions satisfy",
			revisions: []packages.PackageManifest{
				manifestRevision("0.3.5", "^9.4.0"),
			},
			manifest: `name: test_pkg
version: 1.0.0
type: integration
conditions:
  kibana:
    version: "^9.4.0"
requires:
  content:
    - package: sql_input
      version: "^0.3.0"
`,
			check: func(t *testing.T, result *Result, err error, _ string) {
				require.NoError(t, err)
				require.Empty(t, result.Proposals)
			},
		},
		{
			name: "errors on constraint style input pin",
			revisions: []packages.PackageManifest{
				manifestRevision("0.2.0", "^9.4.0"),
				manifestRevision("0.3.0", "^9.4.0"),
			},
			manifest: `name: test_pkg
version: 1.0.0
type: integration
conditions:
  kibana:
    version: "^9.4.0"
requires:
  input:
    - package: sql_input
      version: "^0.2.0"
`,
			check: func(t *testing.T, _ *Result, err error, _ string) {
				require.Error(t, err)
				require.Contains(t, err.Error(), "not a constraint")
			},
		},
		{
			name: "prerelease only fallback when no stable versions exist",
			revisions: []packages.PackageManifest{
				manifestRevision("0.1.0-beta.1", "^9.4.0"),
				manifestRevision("0.2.0-beta.1", "^9.4.0"),
			},
			manifest: `name: test_pkg
version: 1.0.0
type: integration
conditions:
  kibana:
    version: "^9.4.0"
requires:
  input:
    - package: sql_input
      version: "0.1.0-beta.1"
`,
			prerelease: false,
			check: func(t *testing.T, result *Result, err error, _ string) {
				require.NoError(t, err)
				require.Len(t, result.Proposals, 1)
				require.Equal(t, "0.1.0-beta.1", result.Proposals[0].Current)
				require.Equal(t, "0.2.0-beta.1", result.Proposals[0].Proposed)
			},
		},
		{
			name: "prerelease excluded when stable versions exist",
			revisions: []packages.PackageManifest{
				manifestRevision("0.2.0", "^9.4.0"),
				manifestRevision("0.3.0-beta.1", "^9.4.0"),
			},
			manifest: `name: test_pkg
version: 1.0.0
type: integration
conditions:
  kibana:
    version: "^9.4.0"
requires:
  input:
    - package: sql_input
      version: "0.1.0"
`,
			prerelease: false,
			check: func(t *testing.T, result *Result, err error, _ string) {
				require.NoError(t, err)
				require.Len(t, result.Proposals, 1)
				require.Equal(t, "0.2.0", result.Proposals[0].Proposed)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			opts := Options{
				PackageRoot: writeIntegrationPackage(t, tc.manifest),
				Prerelease:  tc.prerelease,
			}
			if tc.revisions != nil {
				srv, client := testRegistryServer(t, tc.revisions)
				t.Cleanup(srv.Close)
				opts.RegistryClient = client
			}
			result, err := Resolve(opts)
			tc.check(t, result, err, tc.manifest)
		})
	}
}

func TestApply(t *testing.T) {
	tests := []struct {
		name      string
		manifest  string
		proposals []UpdateProposal
		check     func(t *testing.T, updated []byte, err error)
	}{
		{
			name:     "multi-proposal updates both input and content",
			manifest: sampleManifest,
			proposals: []UpdateProposal{
				{Kind: InputDependency, Package: "sql_input", Current: "0.2.0", Proposed: "0.3.0"},
				{Kind: ContentDependency, Package: "dashboards", Current: "^0.1.0", Proposed: "0.2.0"},
			},
			check: func(t *testing.T, updated []byte, err error) {
				require.NoError(t, err)
				dir := t.TempDir()
				require.NoError(t, os.WriteFile(filepath.Join(dir, "manifest.yml"), updated, 0o644))
				pkg, err := packages.ReadPackageManifestFromPackageRoot(dir)
				require.NoError(t, err)
				require.Equal(t, "0.3.0", pkg.Requires.Input[0].Version)
				require.Equal(t, "0.2.0", pkg.Requires.Content[0].Version)
			},
		},
		{
			name:     "skips proposal with empty proposed version",
			manifest: sampleManifest,
			proposals: []UpdateProposal{
				{Kind: InputDependency, Package: "sql_input", Current: "0.2.0", Proposed: "", Warning: "needs kibana bump"},
			},
			check: func(t *testing.T, updated []byte, err error) {
				require.NoError(t, err)
				require.Equal(t, []byte(sampleManifest), updated)
			},
		},
		{
			name:     "unknown package returns error",
			manifest: sampleManifest,
			proposals: []UpdateProposal{
				{Kind: InputDependency, Package: "nonexistent", Current: "0.1.0", Proposed: "0.2.0"},
			},
			check: func(t *testing.T, _ []byte, err error) {
				require.Error(t, err)
				require.Contains(t, err.Error(), "nonexistent")
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			updated, err := Apply([]byte(tc.manifest), tc.proposals)
			tc.check(t, updated, err)
		})
	}
}

func TestLatestRevision(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		require.Nil(t, latestRevision(nil))
	})

	t.Run("single", func(t *testing.T) {
		revisions := []packages.PackageManifest{manifestRevision("1.0.0", "^9.0.0")}
		require.Equal(t, "1.0.0", latestRevision(revisions).Version)
	})

	t.Run("unsorted picks max", func(t *testing.T) {
		// Input order is intentionally non-ascending to confirm no sorted assumption.
		revisions := []packages.PackageManifest{
			manifestRevision("0.3.0", "^9.0.0"),
			manifestRevision("0.1.0", "^9.0.0"),
			manifestRevision("0.5.0", "^9.0.0"),
			manifestRevision("0.2.0", "^9.0.0"),
		}
		require.Equal(t, "0.5.0", latestRevision(revisions).Version)
	})

	t.Run("skips unparseable versions", func(t *testing.T) {
		revisions := []packages.PackageManifest{
			manifestRevision("not-a-version", "^9.0.0"),
			manifestRevision("0.2.0", "^9.0.0"),
			manifestRevision("bad", "^9.0.0"),
		}
		require.Equal(t, "0.2.0", latestRevision(revisions).Version)
	})

	t.Run("all unparseable returns nil", func(t *testing.T) {
		revisions := []packages.PackageManifest{
			manifestRevision("not-semver", "^9.0.0"),
		}
		require.Nil(t, latestRevision(revisions))
	})
}

func manifestRevision(version, kibana string) packages.PackageManifest {
	return packages.PackageManifest{
		Name:    "sql_input",
		Version: version,
		Type:    "input",
		Conditions: packages.Conditions{
			Kibana: packages.KibanaConditions{Version: kibana},
		},
	}
}

func testRegistryServer(t *testing.T, revisions []packages.PackageManifest) (*httptest.Server, *registry.Client) {
	t.Helper()
	byPackage := make(map[string][]packages.PackageManifest)
	for _, rev := range revisions {
		byPackage[rev.Name] = append(byPackage[rev.Name], rev)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search" {
			http.NotFound(w, r)
			return
		}
		pkg := r.URL.Query().Get("package")
		all := byPackage[pkg]
		includePrerelease := r.URL.Query().Get("prerelease") == "true"
		if !includePrerelease {
			var stable []packages.PackageManifest
			for _, rev := range all {
				v, err := semver.NewVersion(rev.Version)
				if err != nil || v.Prerelease() == "" {
					stable = append(stable, rev)
				}
			}
			all = stable
		}
		body, err := json.Marshal(all)
		require.NoError(t, err)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	client, err := registry.NewClient(srv.URL)
	require.NoError(t, err)
	return srv, client
}

func writeIntegrationPackage(t *testing.T, manifest string) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "manifest.yml"), []byte(manifest), 0o644))
	return dir
}

// requestCapturingServer creates an httptest.Server that records all received
// requests (protected by a mutex) and delegates to handler. Read *reqs only
// after the fetchAllRevisions call returns.
func requestCapturingServer(t *testing.T, handler http.HandlerFunc) (*registry.Client, *[]http.Request) {
	t.Helper()
	var mu sync.Mutex
	var reqs []http.Request
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		reqs = append(reqs, *r)
		mu.Unlock()
		handler(w, r)
	}))
	t.Cleanup(srv.Close)
	client, err := registry.NewClient(srv.URL)
	require.NoError(t, err)
	return client, &reqs
}

func TestFetchAllRevisions(t *testing.T) {
	jsonBody := func(revisions []packages.PackageManifest) []byte {
		b, _ := json.Marshal(revisions)
		return b
	}

	tests := []struct {
		name       string
		prerelease bool
		handler    http.HandlerFunc
		check      func(t *testing.T, revisions []packages.PackageManifest, err error, reqs []http.Request)
	}{
		{
			name:       "correct query params sent",
			prerelease: false,
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte("[]"))
			},
			check: func(t *testing.T, _ []packages.PackageManifest, _ error, reqs []http.Request) {
				require.GreaterOrEqual(t, len(reqs), 1)
				q := reqs[0].URL.Query()
				require.Equal(t, "true", q.Get("all"))
				require.Equal(t, "true", q.Get("experimental"))
				require.Equal(t, "sql_input", q.Get("package"))
				require.Equal(t, "false", q.Get("prerelease"))
			},
		},
		{
			name:       "prerelease param forwarded",
			prerelease: true,
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write(jsonBody([]packages.PackageManifest{manifestRevision("0.1.0-beta.1", "^9.4.0")}))
			},
			check: func(t *testing.T, _ []packages.PackageManifest, _ error, reqs []http.Request) {
				require.GreaterOrEqual(t, len(reqs), 1)
				require.Equal(t, "true", reqs[0].URL.Query().Get("prerelease"))
			},
		},
		{
			name:       "fallback to prerelease when stable list is empty",
			prerelease: false,
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.URL.Query().Get("prerelease") == "true" {
					_, _ = w.Write(jsonBody([]packages.PackageManifest{manifestRevision("0.1.0-beta.1", "^9.4.0")}))
					return
				}
				_, _ = w.Write([]byte("[]"))
			},
			check: func(t *testing.T, revisions []packages.PackageManifest, err error, reqs []http.Request) {
				require.NoError(t, err)
				require.Len(t, reqs, 2)
				require.Equal(t, "false", reqs[0].URL.Query().Get("prerelease"))
				require.Equal(t, "true", reqs[1].URL.Query().Get("prerelease"))
				require.Len(t, revisions, 1)
				require.Equal(t, "0.1.0-beta.1", revisions[0].Version)
			},
		},
		{
			name:       "no fallback when stable versions exist",
			prerelease: false,
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write(jsonBody([]packages.PackageManifest{manifestRevision("0.2.0", "^9.4.0")}))
			},
			check: func(t *testing.T, revisions []packages.PackageManifest, err error, reqs []http.Request) {
				require.NoError(t, err)
				require.Len(t, reqs, 1)
				require.Len(t, revisions, 1)
			},
		},
		{
			name:       "server error propagated",
			prerelease: false,
			handler: func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "internal error", http.StatusInternalServerError)
			},
			check: func(t *testing.T, _ []packages.PackageManifest, err error, _ []http.Request) {
				require.Error(t, err)
			},
		},
		{
			name:       "both calls fire when all responses empty",
			prerelease: false,
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte("[]"))
			},
			check: func(t *testing.T, revisions []packages.PackageManifest, err error, reqs []http.Request) {
				require.NoError(t, err)
				require.Empty(t, revisions)
				require.Len(t, reqs, 2, "stable call returns empty so prerelease fallback should fire")
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client, reqs := requestCapturingServer(t, tc.handler)
			revisions, err := fetchAllRevisions(client, "sql_input", tc.prerelease)
			tc.check(t, revisions, err, *reqs)
		})
	}
}
