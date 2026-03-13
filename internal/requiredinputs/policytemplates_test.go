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

func TestBundlePolicyTemplatesInputPackageTemplates_InvalidYAML(t *testing.T) {
	buildRootPath := t.TempDir()
	buildRoot, err := os.OpenRoot(buildRootPath)
	require.NoError(t, err)

	r := &RequiredInputsResolver{}

	manifestBytes := []byte("foo: [")
	manifest, _ := packages.ReadPackageManifestBytes(manifestBytes) // may be nil/partial

	err = r.bundlePolicyTemplatesInputPackageTemplates(manifestBytes, manifest, nil, buildRoot)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parsing manifest YAML")
}

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

func TestBundlePolicyTemplatesInputPackageTemplates_SuccessTemplatesCopied(t *testing.T) {
	buildRootPath := t.TempDir()
	buildRoot, err := os.OpenRoot(buildRootPath)
	require.NoError(t, err)

	r := &RequiredInputsResolver{}

	// create current package manifest with one policy template input referencing an input package template
	// it has an existing template, so both the existing and input package template should be copied and the manifest updated to reference both
	manifestBytes := []byte(`
type: integration
requires:
  input:
    - package: sql
      version: 0.1.0
policy_templates:
  - inputs:
    - package: sql
      template_path: existing.yml.hbs
`)
	err = buildRoot.WriteFile("manifest.yml", manifestBytes, 0644)
	require.NoError(t, err)
	err = buildRoot.MkdirAll(filepath.Join("agent", "input"), 0755)
	require.NoError(t, err)
	err = buildRoot.WriteFile(filepath.Join("agent", "input", "existing.yml.hbs"), []byte("existing content"), 0644)
	require.NoError(t, err)

	// parse manifest to pass to function
	manifest, err := packages.ReadPackageManifestBytes(manifestBytes)
	require.NoError(t, err)

	fakeInputDir := createFakeInputHelper(t)
	inputPkgPaths := map[string]string{"sql": fakeInputDir}

	err = r.bundlePolicyTemplatesInputPackageTemplates(manifestBytes, manifest, inputPkgPaths, buildRoot)
	require.NoError(t, err)

	// Files exist.
	_, err = buildRoot.ReadFile(filepath.Join("agent", "input", "sql-input.yml.hbs"))
	require.NoError(t, err)
	_, err = buildRoot.ReadFile(filepath.Join("agent", "input", "existing.yml.hbs"))
	require.NoError(t, err)

	// Written manifest has template_paths set and template_path removed for that input.
	updated, err := buildRoot.ReadFile("manifest.yml")
	require.NoError(t, err)
	updatedManifest, err := packages.ReadPackageManifestBytes(updated)
	require.NoError(t, err)
	require.Len(t, updatedManifest.PolicyTemplates, 1)
	require.Len(t, updatedManifest.PolicyTemplates[0].Inputs, 1)
	input := updatedManifest.PolicyTemplates[0].Inputs[0]
	assert.Empty(t, input.TemplatePath)
	assert.Equal(t, []string{"sql-input.yml.hbs", "existing.yml.hbs"}, input.TemplatePaths)
}

func TestBundlePolicyTemplatesInputPackageTemplates_SuccessTemplatesCopied_DefaultEmptyTemplatePath(t *testing.T) {
	buildRootPath := t.TempDir()
	buildRoot, err := os.OpenRoot(buildRootPath)
	require.NoError(t, err)

	r := &RequiredInputsResolver{}

	// create current package manifest with one policy template input referencing an input package template
	// it has an existing template, so both the existing and input package template should be copied and the manifest updated to reference both
	manifestBytes := []byte(`
type: integration
requires:
  input:
    - package: sql
      version: 0.1.0
policy_templates:
  - inputs:
    - package: sql
`)
	err = buildRoot.WriteFile("manifest.yml", manifestBytes, 0644)
	require.NoError(t, err)
	err = buildRoot.MkdirAll(filepath.Join("agent", "input"), 0755)
	require.NoError(t, err)
	err = buildRoot.WriteFile(filepath.Join("agent", "input", "input.yml.hbs"), []byte("existing content"), 0644)
	require.NoError(t, err)

	// parse manifest to pass to function
	manifest, err := packages.ReadPackageManifestBytes(manifestBytes)
	require.NoError(t, err)

	fakeInputDir := createFakeInputHelper(t)
	inputPkgPaths := map[string]string{"sql": fakeInputDir}

	err = r.bundlePolicyTemplatesInputPackageTemplates(manifestBytes, manifest, inputPkgPaths, buildRoot)
	require.NoError(t, err)

	// Files exist.
	_, err = buildRoot.ReadFile(filepath.Join("agent", "input", "sql-input.yml.hbs"))
	require.NoError(t, err)
	_, err = buildRoot.ReadFile(filepath.Join("agent", "input", "input.yml.hbs"))
	require.NoError(t, err)

	// Written manifest has template_paths set and template_path removed for that input.
	updated, err := buildRoot.ReadFile("manifest.yml")
	require.NoError(t, err)
	updatedManifest, err := packages.ReadPackageManifestBytes(updated)
	require.NoError(t, err)
	require.Len(t, updatedManifest.PolicyTemplates, 1)
	require.Len(t, updatedManifest.PolicyTemplates[0].Inputs, 1)
	input := updatedManifest.PolicyTemplates[0].Inputs[0]
	assert.Empty(t, input.TemplatePath)
	assert.Equal(t, []string{"sql-input.yml.hbs", "input.yml.hbs"}, input.TemplatePaths)
}
