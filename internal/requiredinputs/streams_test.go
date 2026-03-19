// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package requiredinputs

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/elastic-package/internal/packages"
)

// TestBundleDataStreamTemplates_MultiplePolicyTemplates verifies that templates from ALL
// policy templates in the input package are bundled, not just the first one (Issue 5).
func TestBundleDataStreamTemplates_MultiplePolicyTemplates(t *testing.T) {
	buildRootPath := t.TempDir()
	buildRoot, err := os.OpenRoot(buildRootPath)
	require.NoError(t, err)
	defer buildRoot.Close()

	r := &RequiredInputsResolver{}

	datastreamDir := filepath.Join("data_stream", "test_ds")
	err = buildRoot.MkdirAll(datastreamDir, 0755)
	require.NoError(t, err)

	manifestBytes := []byte(`
streams:
  - package: sql
`)
	err = buildRoot.WriteFile(filepath.Join(datastreamDir, "manifest.yml"), manifestBytes, 0644)
	require.NoError(t, err)

	fakeInputDir := createFakeInputWithMultiplePolicyTemplates(t)
	inputPkgPaths := map[string]string{"sql": fakeInputDir}

	err = r.bundleDataStreamTemplates(inputPkgPaths, buildRoot)
	require.NoError(t, err)

	// All templates from both policy templates must be present.
	_, err = buildRoot.ReadFile(filepath.Join(datastreamDir, "agent", "stream", "sql-input.yml.hbs"))
	require.NoError(t, err, "template from first policy_template must be bundled")
	_, err = buildRoot.ReadFile(filepath.Join(datastreamDir, "agent", "stream", "sql-metrics.yml.hbs"))
	require.NoError(t, err, "template from second policy_template must be bundled")
	_, err = buildRoot.ReadFile(filepath.Join(datastreamDir, "agent", "stream", "sql-extra.yml.hbs"))
	require.NoError(t, err, "extra template from second policy_template must be bundled")

	updated, err := buildRoot.ReadFile(filepath.Join(datastreamDir, "manifest.yml"))
	require.NoError(t, err)
	updatedManifest, err := packages.ReadDataStreamManifestBytes(updated)
	require.NoError(t, err)
	require.Len(t, updatedManifest.Streams, 1)
	assert.Equal(t, []string{"sql-input.yml.hbs", "sql-metrics.yml.hbs", "sql-extra.yml.hbs"}, updatedManifest.Streams[0].TemplatePaths)
}

func TestBundleDataStreamTemplates_SuccessTemplatesCopied(t *testing.T) {
	buildRootPath := t.TempDir()
	buildRoot, err := os.OpenRoot(buildRootPath)
	require.NoError(t, err)
	defer buildRoot.Close()

	r := &RequiredInputsResolver{}

	datastreamDir := filepath.Join("data_stream", "test_ds")
	err = buildRoot.MkdirAll(datastreamDir, 0755)
	require.NoError(t, err)
	// create current package manifest with one data stream input referencing an input package template
	// it has an existing template, so both the existing and input package template should be copied and the manifest updated to reference both
	manifestBytes := []byte(`
streams:
  - package: sql
    template_path: existing.yml.hbs
`)
	err = buildRoot.WriteFile(filepath.Join(datastreamDir, "manifest.yml"), manifestBytes, 0644)
	require.NoError(t, err)
	err = buildRoot.MkdirAll(filepath.Join(datastreamDir, "agent", "stream"), 0755)
	require.NoError(t, err)
	err = buildRoot.WriteFile(filepath.Join(datastreamDir, "agent", "stream", "existing.yml.hbs"), []byte("existing content"), 0644)
	require.NoError(t, err)

	fakeInputDir := createFakeInputHelper(t)
	inputPkgPaths := map[string]string{"sql": fakeInputDir}

	err = r.bundleDataStreamTemplates(inputPkgPaths, buildRoot)
	require.NoError(t, err)

	// Files exist.
	_, err = buildRoot.ReadFile(filepath.Join(datastreamDir, "agent", "stream", "sql-input.yml.hbs"))
	require.NoError(t, err)
	_, err = buildRoot.ReadFile(filepath.Join(datastreamDir, "agent", "stream", "existing.yml.hbs"))
	require.NoError(t, err)

	// Written manifest has template_paths set and template_path removed for that input.
	updated, err := buildRoot.ReadFile(filepath.Join(datastreamDir, "manifest.yml"))
	require.NoError(t, err)
	updatedManifest, err := packages.ReadDataStreamManifestBytes(updated)
	require.NoError(t, err)
	require.Len(t, updatedManifest.Streams, 1)
	input := updatedManifest.Streams[0]
	assert.Empty(t, input.TemplatePath)
	assert.Equal(t, []string{"sql-input.yml.hbs", "existing.yml.hbs"}, input.TemplatePaths)
}

// TestBundleDataStreamTemplates_BundlesWithoutDataStreamsAssociation verifies that a data stream
// stream entry with package: X IS bundled even when the root policy template has no data_streams
// field. Bundling is driven solely by the data stream manifest's streams[].package reference.
func TestBundleDataStreamTemplates_BundlesWithoutDataStreamsAssociation(t *testing.T) {
	buildRootPath := t.TempDir()
	buildRoot, err := os.OpenRoot(buildRootPath)
	require.NoError(t, err)
	defer buildRoot.Close()

	r := &RequiredInputsResolver{}

	datastreamDir := filepath.Join("data_stream", "test_ds")
	err = buildRoot.MkdirAll(datastreamDir, 0755)
	require.NoError(t, err)

	manifestBytes := []byte(`
streams:
  - package: sql
`)
	err = buildRoot.WriteFile(filepath.Join(datastreamDir, "manifest.yml"), manifestBytes, 0644)
	require.NoError(t, err)

	fakeInputDir := createFakeInputHelper(t)
	inputPkgPaths := map[string]string{"sql": fakeInputDir}

	err = r.bundleDataStreamTemplates(inputPkgPaths, buildRoot)
	require.NoError(t, err)

	// Template must be bundled even without a data_streams association in the root manifest.
	_, err = buildRoot.ReadFile(filepath.Join(datastreamDir, "agent", "stream", "sql-input.yml.hbs"))
	require.NoError(t, err, "template must be bundled when stream references an input package, regardless of data_streams field")

	// The data stream manifest must have template_paths set.
	updated, err := buildRoot.ReadFile(filepath.Join(datastreamDir, "manifest.yml"))
	require.NoError(t, err)
	updatedManifest, err := packages.ReadDataStreamManifestBytes(updated)
	require.NoError(t, err)
	require.Len(t, updatedManifest.Streams, 1)
	assert.Equal(t, []string{"sql-input.yml.hbs"}, updatedManifest.Streams[0].TemplatePaths)
}
