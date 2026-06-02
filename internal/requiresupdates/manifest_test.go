// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package requiresupdates

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

const sampleManifest = `name: test_integration
version: 1.0.0
type: integration
requires:
  input:
    - package: sql_input
      version: "0.2.0"
  content:
    - package: dashboards
      version: "^0.1.0"
`

func TestSetRequiresDependencyVersion(t *testing.T) {
	updated, err := setRequiresDependencyVersion([]byte(sampleManifest), "input", "sql_input", "0.3.0")
	require.NoError(t, err)
	require.Contains(t, string(updated), "0.3.0")
	require.Contains(t, string(updated), "dashboards")
	require.Contains(t, string(updated), "^0.1.0")
}

func TestSetRequiresDependencyVersion_roundTripFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.yml")
	require.NoError(t, os.WriteFile(path, []byte(sampleManifest), 0o644))

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	data, err = setRequiresDependencyVersion(data, "content", "dashboards", "0.2.0")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, data, 0o644))

	rewritten, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Contains(t, string(rewritten), `0.2.0`)
}

func TestSetRequiresDependencyVersion_versionUpdated(t *testing.T) {
	t.Run("unquoted constraint becomes unquoted exact", func(t *testing.T) {
		manifest := `name: test_integration
version: 1.0.0
type: integration
requires:
  content:
    - package: dashboards
      version: ^0.3.0
`
		updated, err := setRequiresDependencyVersion([]byte(manifest), "content", "dashboards", "0.4.0")
		require.NoError(t, err)
		require.Contains(t, string(updated), `0.4.0`)
	})

	t.Run("quoted constraint updated to new version", func(t *testing.T) {
		manifest := `name: test_integration
version: 1.0.0
type: integration
requires:
  content:
    - package: dashboards
      version: "^0.3.0"
`
		updated, err := setRequiresDependencyVersion([]byte(manifest), "content", "dashboards", "0.4.0")
		require.NoError(t, err)
		require.Contains(t, string(updated), `0.4.0`)
	})
}
