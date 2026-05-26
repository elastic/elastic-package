// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package requiresupdates

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
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

	var node yaml.Node
	require.NoError(t, yaml.Unmarshal(updated, &node))
	root := node.Content[0]
	requires := findMapValueNode(root, "requires")
	input := findMapValueNode(requires, "input")
	item := input.Content[0]
	version := findMapValueNode(item, "version")
	require.Equal(t, "0.3.0", version.Value)
}

func TestSetRequiresDependencyVersion_preservesFormatting(t *testing.T) {
	manifest := `format_version: 3.6.2
name: test_integration
version: 1.0.0
type: integration
requires:
  input:
    - package: sql_input
      version: '0.2.0'   # pinned input dep
  content:
    - package: dashboards
      version: "^0.1.0"
policy_templates: []
`

	updated, err := setRequiresDependencyVersion([]byte(manifest), "input", "sql_input", "0.3.0")
	require.NoError(t, err)

	linesBefore := strings.Split(manifest, "\n")
	linesAfter := strings.Split(string(updated), "\n")
	require.Len(t, linesAfter, len(linesBefore))

	for i, before := range linesBefore {
		if strings.Contains(before, "version: '0.2.0'") {
			require.Equal(t, "      version: '0.3.0'   # pinned input dep", linesAfter[i])
			continue
		}
		require.Equal(t, before, linesAfter[i], "line %d should be unchanged", i+1)
	}
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
	require.Contains(t, string(rewritten), `"0.2.0"`)
}

func Test_replaceVersionOnLine(t *testing.T) {
	t.Run("double quoted with comment", func(t *testing.T) {
		got, err := replaceVersionOnLine(`      version: "0.2.0"   # note`, "0.3.0")
		require.NoError(t, err)
		require.Equal(t, `      version: "0.3.0"   # note`, got)
	})

	t.Run("single quoted", func(t *testing.T) {
		got, err := replaceVersionOnLine(`      version: '0.2.0'`, "0.3.0")
		require.NoError(t, err)
		require.Equal(t, `      version: '0.3.0'`, got)
	})

	t.Run("unquoted constraint", func(t *testing.T) {
		got, err := replaceVersionOnLine(`      version: ^0.1.0`, "^0.2.0")
		require.NoError(t, err)
		require.Equal(t, `      version: ^0.2.0`, got)
	})

	t.Run("unquoted constraint bumped to unquoted exact", func(t *testing.T) {
		// Constraint operator is dropped; unquoted style is preserved.
		got, err := replaceVersionOnLine(`      version: ^0.3.0`, "0.4.0")
		require.NoError(t, err)
		require.Equal(t, `      version: 0.4.0`, got)
	})
}

func TestSetRequiresDependencyVersion_contentPreservesQuoteStyle(t *testing.T) {
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
		require.Contains(t, string(updated), `version: 0.4.0`)
		require.NotContains(t, string(updated), `"0.4.0"`)
	})

	t.Run("double-quoted constraint becomes double-quoted exact", func(t *testing.T) {
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
		require.Contains(t, string(updated), `version: "0.4.0"`)
	})
}
