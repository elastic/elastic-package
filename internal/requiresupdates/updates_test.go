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
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/registry"
)

func TestUpdate_bumpsCompatibleDependency(t *testing.T) {
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

	result, err := Update(Options{
		PackageRoot:    packageRoot,
		RegistryClient: client,
		DryRun:         true,
	})
	require.NoError(t, err)
	require.Equal(t, "test_pkg", result.Package)
	require.Equal(t, "elastic/integrations", result.CodeOwner)
	require.Len(t, result.Proposals, 1)
	require.Equal(t, "0.2.0", result.Proposals[0].Current)
	require.Equal(t, "0.3.0", result.Proposals[0].Proposed)
	require.Empty(t, result.Proposals[0].Warning)
}

func TestUpdate_warnsWhenNewerRequiresHigherKibana(t *testing.T) {
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

	result, err := Update(Options{
		PackageRoot:    packageRoot,
		RegistryClient: client,
		DryRun:         true,
	})
	require.NoError(t, err)
	require.Len(t, result.Proposals, 1)
	require.Equal(t, "0.3.0", result.Proposals[0].Proposed)
	require.Contains(t, result.Proposals[0].Warning, "0.5.0")
	require.Contains(t, result.Proposals[0].Warning, "^9.6.0")
}

func TestUpdate_appliesManifestChange(t *testing.T) {
	revisions := []packages.PackageManifest{
		manifestRevision("0.2.0", "^9.4.0"),
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
  input:
    - package: sql_input
      version: "0.2.0"
`)

	result, err := Update(Options{
		PackageRoot:    packageRoot,
		RegistryClient: client,
	})
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.Equal(t, "0.4.0", result.Proposals[0].Proposed)

	manifest, err := packages.ReadPackageManifestFromPackageRoot(packageRoot)
	require.NoError(t, err)
	require.Equal(t, "0.4.0", manifest.Requires.Input[0].Version)
}

func TestUpdate_skipsNonIntegration(t *testing.T) {
	packageRoot := writeIntegrationPackage(t, `name: test_input
version: 1.0.0
type: input
`)
	result, err := Update(Options{PackageRoot: packageRoot})
	require.NoError(t, err)
	require.NotEmpty(t, result.SkipReason)
	require.Contains(t, result.SkipReason, "integration")
}

func TestUpdate_skipsIntegrationWithoutRequires(t *testing.T) {
	packageRoot := writeIntegrationPackage(t, `name: test_integration
version: 1.0.0
type: integration
`)
	result, err := Update(Options{PackageRoot: packageRoot})
	require.NoError(t, err)
	require.NotEmpty(t, result.SkipReason)
	require.Contains(t, result.SkipReason, "requires")
}

func TestUpdate_warningOnlyWhenAllRevisionsRequireHigherKibana(t *testing.T) {
	// All available revisions require ^9.6.0; the integration is capped at <9.6.0.
	// latestCompatible == nil, but a newer unfiltered revision exists, so the result
	// should be a proposal with Proposed=="" and a non-empty Warning (lines 183-202 in updates.go).
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

	result, err := Update(Options{
		PackageRoot:    packageRoot,
		RegistryClient: client,
		DryRun:         true,
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

func TestUpdate_errorsOnConstraintStylePin(t *testing.T) {
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

	_, err := Update(Options{
		PackageRoot:    packageRoot,
		RegistryClient: client,
		DryRun:         true,
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
		body, err := json.Marshal(byPackage[pkg])
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
