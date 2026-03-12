// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package requiredinputs

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/elastic/elastic-package/internal/packages"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBundleDataStreamTemplates_SuccessTemplatesCopied(t *testing.T) {
	buildRootPath := t.TempDir()
	buildRoot, err := os.OpenRoot(buildRootPath)
	require.NoError(t, err)

	r := &InputRequiredResolver{buildRoot: buildRoot}
	defer r.Cleanup()

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

	fakeInputDir := createFakeInputHelper(t)
	inputPkgPaths := map[string]string{"sql": fakeInputDir}

	err = r.bundleDataStreamTemplates(inputPkgPaths)
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

func TestBundleDataStreamTemplates_SuccessTemplatesCopied_DefaultEmptyTemplatePath(t *testing.T) {
	buildRootPath := t.TempDir()
	buildRoot, err := os.OpenRoot(buildRootPath)
	require.NoError(t, err)

	r := &InputRequiredResolver{buildRoot: buildRoot}
	defer r.Cleanup()

	datastreamDir := filepath.Join("data_stream", "test_ds")
	err = buildRoot.MkdirAll(datastreamDir, 0755)
	require.NoError(t, err)
	// create current package manifest with one data stream input referencing an input package template
	// it has an existing template, so both the existing and input package template should be copied and the manifest updated to reference both
	manifestBytes := []byte(`
streams:
  - package: sql
`)
	err = buildRoot.WriteFile(filepath.Join(datastreamDir, "manifest.yml"), manifestBytes, 0644)
	require.NoError(t, err)
	err = buildRoot.MkdirAll(filepath.Join(datastreamDir, "agent", "stream"), 0755)
	require.NoError(t, err)
	err = buildRoot.WriteFile(filepath.Join(datastreamDir, "agent", "stream", "stream.yml.hbs"), []byte("existing content"), 0644)

	fakeInputDir := createFakeInputHelper(t)
	inputPkgPaths := map[string]string{"sql": fakeInputDir}

	err = r.bundleDataStreamTemplates(inputPkgPaths)
	require.NoError(t, err)

	// Files exist.
	_, err = buildRoot.ReadFile(filepath.Join(datastreamDir, "agent", "stream", "sql-input.yml.hbs"))
	require.NoError(t, err)
	_, err = buildRoot.ReadFile(filepath.Join(datastreamDir, "agent", "stream", "stream.yml.hbs"))
	require.NoError(t, err)

	// Written manifest has template_paths set and template_path removed for that input.
	updated, err := buildRoot.ReadFile(filepath.Join(datastreamDir, "manifest.yml"))
	require.NoError(t, err)
	updatedManifest, err := packages.ReadDataStreamManifestBytes(updated)
	require.NoError(t, err)
	require.Len(t, updatedManifest.Streams, 1)
	input := updatedManifest.Streams[0]
	assert.Empty(t, input.TemplatePath)
	assert.Equal(t, []string{"sql-input.yml.hbs", "stream.yml.hbs"}, input.TemplatePaths)
}
