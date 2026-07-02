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

// createFakeInputWithDatasetVar creates an input package that declares
// data_stream.dataset alongside a regular user-facing var (paths). Used to
// verify that the bundler excludes data_stream.dataset from composable
// integration stream vars while preserving the other vars.
func createFakeInputWithDatasetVar(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	pkgDir := filepath.Join(dir, "input_with_dataset_var")
	require.NoError(t, os.Mkdir(pkgDir, 0755))
	manifest := []byte(`name: input_with_dataset_var
version: 0.1.0
type: input
policy_templates:
  - name: test_logs
    type: logs
    title: Test Logs
    description: Input package that exposes data_stream.dataset.
    input: logfile
    template_path: input.yml.hbs
    vars:
      - name: data_stream.dataset
        type: text
        title: Dataset name
        required: true
      - name: paths
        type: text
        title: Paths
        multi: true
        required: true
        show_user: true
        default:
          - /var/log/*.log
`)
	require.NoError(t, os.WriteFile(filepath.Join(pkgDir, "manifest.yml"), manifest, 0644))
	require.NoError(t, os.MkdirAll(filepath.Join(pkgDir, "agent", "input"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(pkgDir, "agent", "input", "input.yml.hbs"), []byte("paths: {{paths}}\ndataset: {{data_stream.dataset}}\n"), 0644))
	return pkgDir
}

// createFakeInputWithOnlyDatasetVar creates an input package whose only var is
// data_stream.dataset. Used to verify that excluding data_stream.dataset from
// the base vars (leaving zero base vars) still falls through to merge the
// composable data stream's own var overrides, instead of short-circuiting.
func createFakeInputWithOnlyDatasetVar(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	pkgDir := filepath.Join(dir, "input_only_dataset_var")
	require.NoError(t, os.Mkdir(pkgDir, 0755))
	manifest := []byte(`name: input_only_dataset_var
version: 0.1.0
type: input
policy_templates:
  - name: test_logs
    type: logs
    title: Test Logs
    description: Input package whose only var is data_stream.dataset.
    input: logfile
    template_path: input.yml.hbs
    vars:
      - name: data_stream.dataset
        type: text
        title: Dataset name
        required: true
`)
	require.NoError(t, os.WriteFile(filepath.Join(pkgDir, "manifest.yml"), manifest, 0644))
	require.NoError(t, os.MkdirAll(filepath.Join(pkgDir, "agent", "input"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(pkgDir, "agent", "input", "input.yml.hbs"), []byte("dataset: {{data_stream.dataset}}\n"), 0644))
	return pkgDir
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
