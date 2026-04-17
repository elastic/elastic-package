// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package builder

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/elastic-package/internal/files"
)

func TestFleetPolicyPackageRoot(t *testing.T) {
	repoRoot, err := files.FindRepositoryRoot()
	require.NoError(t, err)
	t.Cleanup(func() { _ = repoRoot.Close() })
	repo := repoRoot.Name()

	t.Run("integration_without_requires_returns_source", func(t *testing.T) {
		src := filepath.Join(repo, "test", "packages", "parallel", "nginx")
		got, err := FleetPolicyPackageRoot(src)
		require.NoError(t, err)
		assert.Equal(t, src, got)
	})

	t.Run("input_package_returns_source", func(t *testing.T) {
		src := filepath.Join(repo, "test", "packages", "parallel", "sql_input")
		got, err := FleetPolicyPackageRoot(src)
		require.NoError(t, err)
		assert.Equal(t, src, got)
	})

	t.Run("composable_without_built_tree_errors", func(t *testing.T) {
		tmp := t.TempDir()
		// Unique name so the expected build/packages/<name>/<ver> path is unlikely to exist.
		manifest := `format_version: 3.0.0
name: fleet_policy_root_ephemeral
title: Ephemeral
version: 0.0.0-test
type: integration
policy_templates:
  - name: default
    title: Default
    inputs:
      - type: logs
        title: Logs
requires:
  input:
    - package: dummy
      version: "1.0.0"
`
		err := os.WriteFile(filepath.Join(tmp, "manifest.yml"), []byte(manifest), 0644)
		require.NoError(t, err)

		_, err = FleetPolicyPackageRoot(tmp)
		require.Error(t, err)
		assert.ErrorContains(t, err, "built package manifest not found")
	})

	t.Run("composable_with_built_tree_returns_build_dir", func(t *testing.T) {
		src := filepath.Join(repo, "test", "packages", "composable", "02_ci_composable_integration")
		built := filepath.Join(repo, "build", "packages", "ci_composable_integration", "0.1.0")
		if _, err := os.Stat(filepath.Join(built, "manifest.yml")); err != nil {
			t.Skip("built composable fixture not present; run elastic-package build on the package first")
		}
		got, err := FleetPolicyPackageRoot(src)
		require.NoError(t, err)
		assert.Equal(t, built, got)
	})
}
