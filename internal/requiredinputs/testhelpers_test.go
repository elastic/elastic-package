// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package requiredinputs

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func createFakeInputHelper(t *testing.T) string {
	t.Helper()
	// create fake input package with manifest and template file
	fakeDownloadedPkgDir := t.TempDir()
	inputPkgDir := filepath.Join(fakeDownloadedPkgDir, "sql")
	err := os.Mkdir(inputPkgDir, 0755)
	require.NoError(t, err)
	inputManifestBytes := []byte(`name: sql
version: 0.1.0
type: input
policy_templates:
  - input: sql
    template_path: input.yml.hbs
`)
	err = os.WriteFile(filepath.Join(inputPkgDir, "manifest.yml"), inputManifestBytes, 0644)
	require.NoError(t, err)
	err = os.MkdirAll(filepath.Join(inputPkgDir, "agent", "input"), 0755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(inputPkgDir, "agent", "input", "input.yml.hbs"), []byte("template content"), 0644)
	require.NoError(t, err)
	return inputPkgDir
}

func createFakeInputWithMultiplePolicyTemplates(t *testing.T) string {
	t.Helper()
	fakeDownloadedPkgDir := t.TempDir()
	inputPkgDir := filepath.Join(fakeDownloadedPkgDir, "sql")
	err := os.Mkdir(inputPkgDir, 0755)
	require.NoError(t, err)
	// Input package with two policy templates, each declaring a distinct template.
	inputManifestBytes := []byte(`name: sql
version: 0.1.0
type: input
policy_templates:
  - input: sql
    template_path: input.yml.hbs
  - input: sql/metrics
    template_paths:
      - metrics.yml.hbs
      - extra.yml.hbs
`)
	err = os.WriteFile(filepath.Join(inputPkgDir, "manifest.yml"), inputManifestBytes, 0644)
	require.NoError(t, err)
	err = os.MkdirAll(filepath.Join(inputPkgDir, "agent", "input"), 0755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(inputPkgDir, "agent", "input", "input.yml.hbs"), []byte("input template"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(inputPkgDir, "agent", "input", "metrics.yml.hbs"), []byte("metrics template"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(inputPkgDir, "agent", "input", "extra.yml.hbs"), []byte("extra template"), 0644)
	require.NoError(t, err)
	return inputPkgDir
}
