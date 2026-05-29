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

func TestResolve_bumpsCompatibleDependency(t *testing.T) {
	revisions := []packages.PackageManifest{
		manifestRevision("0.2.0", "^9.4.0"),
		manifestRevision("0.3.0", "^9.4.0"),
	}
	srv, client := testRegistryServer(t, revisions)
	t.Cleanup(srv.Close)

	packageRoot := writeIntegrationPackage(t, `name: test_pkg
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
`)

	result, err := Resolve(Options{
		PackageRoot:    packageRoot,
		RegistryClient: client,
	})
	require.NoError(t, err)
	require.Equal(t, "test_pkg", result.Package)
	require.Equal(t, "elastic/integrations", result.CodeOwner)
	require.Len(t, result.Proposals, 1)
	require.Equal(t, "0.2.0", result.Proposals[0].Current)
	require.Equal(t, "0.3.0", result.Proposals[0].Proposed)
	require.Empty(t, result.Proposals[0].Warning)
}

func TestResolve_warnsWhenNewerRequiresHigherKibana(t *testing.T) {
	revisions := []packages.PackageManifest{
		manifestRevision("0.2.0", "^9.4.0"),
		manifestRevision("0.3.0", "^9.4.0"),
		manifestRevision("0.5.0", "^9.6.0"),
	}
	srv, client := testRegistryServer(t, revisions)
	t.Cleanup(srv.Close)

	packageRoot := writeIntegrationPackage(t, `name: test_pkg
version: 1.0.0
type: integration
conditions:
  kibana:
    version: ">=9.4.0,<9.6.0"
requires:
  input:
    - package: sql_input
      version: "0.2.0"
`)

	result, err := Resolve(Options{
		PackageRoot:    packageRoot,
		RegistryClient: client,
	})
	require.NoError(t, err)
	require.Len(t, result.Proposals, 1)
	require.Equal(t, "0.3.0", result.Proposals[0].Proposed)
	require.Contains(t, result.Proposals[0].Warning, "0.5.0")
	require.Contains(t, result.Proposals[0].Warning, "^9.6.0")
}

func TestResolve_appliesManifestChangeViaApply(t *testing.T) {
	revisions := []packages.PackageManifest{
		manifestRevision("0.2.0", "^9.4.0"),
		manifestRevision("0.4.0", "^9.4.0"),
	}
	srv, client := testRegistryServer(t, revisions)
	t.Cleanup(srv.Close)

	manifestContent := `name: test_pkg
version: 1.0.0
type: integration
conditions:
  kibana:
    version: "^9.4.0"
requires:
  input:
    - package: sql_input
      version: "0.2.0"
`
	packageRoot := writeIntegrationPackage(t, manifestContent)

	result, err := Resolve(Options{
		PackageRoot:    packageRoot,
		RegistryClient: client,
	})
	require.NoError(t, err)
	require.Len(t, result.Proposals, 1)
	require.Equal(t, "0.4.0", result.Proposals[0].Proposed)

	updated, err := Apply([]byte(manifestContent), result.Proposals)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(packageRoot, "manifest.yml"), updated, 0o644))

	manifest, err := packages.ReadPackageManifestFromPackageRoot(packageRoot)
	require.NoError(t, err)
	require.Equal(t, "0.4.0", manifest.Requires.Input[0].Version)
}

func TestResolve_skipsNonIntegration(t *testing.T) {
	packageRoot := writeIntegrationPackage(t, `name: test_input
version: 1.0.0
type: input
`)
	result, err := Resolve(Options{PackageRoot: packageRoot})
	require.NoError(t, err)
	require.NotEmpty(t, result.SkipReason)
	require.Contains(t, result.SkipReason, "integration")
}

func TestResolve_skipsIntegrationWithoutRequires(t *testing.T) {
	packageRoot := writeIntegrationPackage(t, `name: test_integration
version: 1.0.0
type: integration
`)
	result, err := Resolve(Options{PackageRoot: packageRoot})
	require.NoError(t, err)
	require.NotEmpty(t, result.SkipReason)
	require.Contains(t, result.SkipReason, "requires")
}

func TestResolve_warningOnlyWhenAllRevisionsRequireHigherKibana(t *testing.T) {
	// All available revisions require ^9.6.0; the integration is capped at <9.6.0.
	// latestCompatible == nil but a newer unfiltered revision exists: expect a
	// proposal with Proposed=="" and a non-empty Warning.
	revisions := []packages.PackageManifest{
		manifestRevision("0.3.0", "^9.6.0"),
	}
	srv, client := testRegistryServer(t, revisions)
	t.Cleanup(srv.Close)

	packageRoot := writeIntegrationPackage(t, `name: test_pkg
version: 1.0.0
type: integration
conditions:
  kibana:
    version: ">=9.4.0,<9.6.0"
requires:
  input:
    - package: sql_input
      version: "0.2.0"
`)

	result, err := Resolve(Options{
		PackageRoot:    packageRoot,
		RegistryClient: client,
	})
	require.NoError(t, err)
	require.Len(t, result.Proposals, 1)
	p := result.Proposals[0]
	require.Equal(t, "0.2.0", p.Current)
	require.Empty(t, p.Proposed, "expected no proposed version when no compatible revision exists")
	require.NotEmpty(t, p.Warning, "expected a warning when only a higher-Kibana revision is available")
	require.Contains(t, p.Warning, "0.3.0")
	require.Contains(t, p.Warning, "^9.6.0")
}

func TestResolve_contentDep_exactPin_bumps(t *testing.T) {
	revisions := []packages.PackageManifest{
		manifestRevision("0.2.0", "^9.4.0"),
		manifestRevision("0.3.0", "^9.4.0"),
	}
	srv, client := testRegistryServer(t, revisions)
	t.Cleanup(srv.Close)

	packageRoot := writeIntegrationPackage(t, `name: test_pkg
version: 1.0.0
type: integration
conditions:
  kibana:
    version: "^9.4.0"
requires:
  content:
    - package: sql_input
      version: "0.2.0"
`)

	result, err := Resolve(Options{
		PackageRoot:    packageRoot,
		RegistryClient: client,
	})
	require.NoError(t, err)
	require.Len(t, result.Proposals, 1)
	p := result.Proposals[0]
	require.Equal(t, ContentDependency, p.Kind)
	require.Equal(t, "0.2.0", p.Current)
	require.Equal(t, "0.3.0", p.Proposed)
	require.Empty(t, p.Warning)
}

func TestResolve_contentDep_constraintStyle_bumps(t *testing.T) {
	// ^0.3.0 covers >=0.3.0,<0.4.0; 0.3.5 satisfies it, 0.4.0 does not → propose 0.4.0.
	revisions := []packages.PackageManifest{
		manifestRevision("0.3.5", "^9.4.0"),
		manifestRevision("0.4.0", "^9.4.0"),
	}
	srv, client := testRegistryServer(t, revisions)
	t.Cleanup(srv.Close)

	packageRoot := writeIntegrationPackage(t, `name: test_pkg
version: 1.0.0
type: integration
conditions:
  kibana:
    version: "^9.4.0"
requires:
  content:
    - package: sql_input
      version: "^0.3.0"
`)

	result, err := Resolve(Options{
		PackageRoot:    packageRoot,
		RegistryClient: client,
	})
	require.NoError(t, err)
	require.Len(t, result.Proposals, 1)
	p := result.Proposals[0]
	require.Equal(t, ContentDependency, p.Kind)
	require.Equal(t, "^0.3.0", p.Current)
	require.Equal(t, "0.4.0", p.Proposed)
	require.Empty(t, p.Warning)
}

func TestResolve_contentDep_constraintStyle_noUpdate(t *testing.T) {
	// All registry versions satisfy ^0.3.0 → no update needed.
	revisions := []packages.PackageManifest{
		manifestRevision("0.3.5", "^9.4.0"),
	}
	srv, client := testRegistryServer(t, revisions)
	t.Cleanup(srv.Close)

	packageRoot := writeIntegrationPackage(t, `name: test_pkg
version: 1.0.0
type: integration
conditions:
  kibana:
    version: "^9.4.0"
requires:
  content:
    - package: sql_input
      version: "^0.3.0"
`)

	result, err := Resolve(Options{
		PackageRoot:    packageRoot,
		RegistryClient: client,
	})
	require.NoError(t, err)
	require.Empty(t, result.Proposals)
}

func TestResolve_errorsOnConstraintStylePin(t *testing.T) {
	revisions := []packages.PackageManifest{
		manifestRevision("0.2.0", "^9.4.0"),
		manifestRevision("0.3.0", "^9.4.0"),
	}
	srv, client := testRegistryServer(t, revisions)
	t.Cleanup(srv.Close)

	packageRoot := writeIntegrationPackage(t, `name: test_pkg
version: 1.0.0
type: integration
conditions:
  kibana:
    version: "^9.4.0"
requires:
  input:
    - package: sql_input
      version: "^0.2.0"
`)

	_, err := Resolve(Options{
		PackageRoot:    packageRoot,
		RegistryClient: client,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "not a constraint")
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

func TestResolve_prereleaseOnlyFallback(t *testing.T) {
	// All available versions are pre-releases. Without --prerelease, the fallback
	// should still include them so the dependency can be bumped.
	revisions := []packages.PackageManifest{
		manifestRevision("0.1.0-beta.1", "^9.4.0"),
		manifestRevision("0.2.0-beta.1", "^9.4.0"),
	}
	srv, client := testRegistryServer(t, revisions)
	t.Cleanup(srv.Close)

	packageRoot := writeIntegrationPackage(t, `name: test_pkg
version: 1.0.0
type: integration
conditions:
  kibana:
    version: "^9.4.0"
requires:
  input:
    - package: sql_input
      version: "0.1.0-beta.1"
`)

	result, err := Resolve(Options{
		PackageRoot:    packageRoot,
		RegistryClient: client,
		Prerelease:     false,
	})
	require.NoError(t, err)
	require.Len(t, result.Proposals, 1)
	require.Equal(t, "0.1.0-beta.1", result.Proposals[0].Current)
	require.Equal(t, "0.2.0-beta.1", result.Proposals[0].Proposed)
}

func TestResolve_prereleaseExcludedWhenStableExists(t *testing.T) {
	// Mix of stable and pre-release versions. Without --prerelease, only stable
	// versions should be considered; the pre-release must not be proposed.
	revisions := []packages.PackageManifest{
		manifestRevision("0.2.0", "^9.4.0"),
		manifestRevision("0.3.0-beta.1", "^9.4.0"),
	}
	srv, client := testRegistryServer(t, revisions)
	t.Cleanup(srv.Close)

	packageRoot := writeIntegrationPackage(t, `name: test_pkg
version: 1.0.0
type: integration
conditions:
  kibana:
    version: "^9.4.0"
requires:
  input:
    - package: sql_input
      version: "0.1.0"
`)

	result, err := Resolve(Options{
		PackageRoot:    packageRoot,
		RegistryClient: client,
		Prerelease:     false,
	})
	require.NoError(t, err)
	require.Len(t, result.Proposals, 1)
	require.Equal(t, "0.2.0", result.Proposals[0].Proposed)
}

func TestApply_multiProposal(t *testing.T) {
	manifest := []byte(sampleManifest)
	proposals := []UpdateProposal{
		{Kind: InputDependency, Package: "sql_input", Current: "0.2.0", Proposed: "0.3.0"},
		{Kind: ContentDependency, Package: "dashboards", Current: "^0.1.0", Proposed: "0.2.0"},
	}
	updated, err := Apply(manifest, proposals)
	require.NoError(t, err)

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "manifest.yml"), updated, 0o644))
	pkg, err := packages.ReadPackageManifestFromPackageRoot(dir)
	require.NoError(t, err)
	require.Equal(t, "0.3.0", pkg.Requires.Input[0].Version)
	require.Equal(t, "0.2.0", pkg.Requires.Content[0].Version)
}

func TestApply_skipsEmptyProposed(t *testing.T) {
	manifest := []byte(sampleManifest)
	proposals := []UpdateProposal{
		{Kind: InputDependency, Package: "sql_input", Current: "0.2.0", Proposed: "", Warning: "needs kibana bump"},
	}
	updated, err := Apply(manifest, proposals)
	require.NoError(t, err)
	require.Equal(t, manifest, updated)
}

func TestApply_unknownPackageReturnsError(t *testing.T) {
	manifest := []byte(sampleManifest)
	proposals := []UpdateProposal{
		{Kind: InputDependency, Package: "nonexistent", Current: "0.1.0", Proposed: "0.2.0"},
	}
	_, err := Apply(manifest, proposals)
	require.Error(t, err)
	require.Contains(t, err.Error(), "nonexistent")
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
