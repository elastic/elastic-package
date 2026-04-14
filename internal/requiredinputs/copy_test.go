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
)

// buildRootFor creates a temporary os.Root for use in tests.
func buildRootFor(t *testing.T) *os.Root {
	t.Helper()
	root, err := os.OpenRoot(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { root.Close() })
	return root
}

// TestCollectAndCopyPolicyTemplateFiles_SingleTemplatePath verifies that a package whose
// policy_template declares a single template_path is copied into destDir with the
// "<pkgName>-<name>" prefix, and that the returned slice contains exactly that name.
func TestCollectAndCopyPolicyTemplateFiles_SingleTemplatePath(t *testing.T) {
	inputPkgDir := createFakeInputHelper(t)
	buildRoot := buildRootFor(t)

	destDir := filepath.Join("agent", "input")
	got, err := collectAndCopyPolicyTemplateFiles(inputPkgDir, "sql", destDir, buildRoot)
	require.NoError(t, err)

	assert.Equal(t, []string{"sql-input.yml.hbs"}, got)

	content, err := buildRoot.ReadFile(filepath.Join(destDir, "sql-input.yml.hbs"))
	require.NoError(t, err)
	assert.Equal(t, "template content", string(content))
}

// TestCollectAndCopyPolicyTemplateFiles_MultipleTemplatePaths verifies that all names listed
// in template_paths across multiple policy_templates are copied.
func TestCollectAndCopyPolicyTemplateFiles_MultipleTemplatePaths(t *testing.T) {
	inputPkgDir := createFakeInputWithMultiplePolicyTemplates(t)
	buildRoot := buildRootFor(t)

	destDir := filepath.Join("agent", "input")
	got, err := collectAndCopyPolicyTemplateFiles(inputPkgDir, "sql", destDir, buildRoot)
	require.NoError(t, err)

	assert.Equal(t, []string{"sql-input.yml.hbs", "sql-metrics.yml.hbs", "sql-extra.yml.hbs"}, got)

	for _, name := range []string{"sql-input.yml.hbs", "sql-metrics.yml.hbs", "sql-extra.yml.hbs"} {
		_, err := buildRoot.ReadFile(filepath.Join(destDir, name))
		require.NoError(t, err, "expected %s to exist in destDir", name)
	}
}

// TestCollectAndCopyPolicyTemplateFiles_Deduplication verifies that when the same template name
// appears in more than one policy_template it is only copied once.
func TestCollectAndCopyPolicyTemplateFiles_Deduplication(t *testing.T) {
	inputPkgDir := t.TempDir()
	manifest := []byte(`name: sql
version: 0.1.0
type: input
policy_templates:
  - input: sql
    template_path: shared.yml.hbs
  - input: sql/metrics
    template_path: shared.yml.hbs
`)
	err := os.WriteFile(filepath.Join(inputPkgDir, "manifest.yml"), manifest, 0644)
	require.NoError(t, err)
	err = os.MkdirAll(filepath.Join(inputPkgDir, "agent", "input"), 0755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(inputPkgDir, "agent", "input", "shared.yml.hbs"), []byte("shared"), 0644)
	require.NoError(t, err)

	buildRoot := buildRootFor(t)
	destDir := filepath.Join("agent", "input")

	got, err := collectAndCopyPolicyTemplateFiles(inputPkgDir, "sql", destDir, buildRoot)
	require.NoError(t, err)

	// Returned slice must contain the prefixed name exactly once.
	assert.Equal(t, []string{"sql-shared.yml.hbs"}, got)
}

// TestCollectAndCopyPolicyTemplateFiles_NoTemplates verifies that a package whose
// policy_templates have neither template_path nor template_paths returns an empty slice
// without error.
func TestCollectAndCopyPolicyTemplateFiles_NoTemplates(t *testing.T) {
	inputPkgDir := t.TempDir()
	manifest := []byte(`name: sql
version: 0.1.0
type: input
policy_templates:
  - input: sql
`)
	err := os.WriteFile(filepath.Join(inputPkgDir, "manifest.yml"), manifest, 0644)
	require.NoError(t, err)
	err = os.MkdirAll(filepath.Join(inputPkgDir, "agent", "input"), 0755)
	require.NoError(t, err)

	buildRoot := buildRootFor(t)

	got, err := collectAndCopyPolicyTemplateFiles(inputPkgDir, "sql", "agent/input", buildRoot)
	require.NoError(t, err)
	assert.Empty(t, got)
}

// TestCollectAndCopyPolicyTemplateFiles_MissingTemplateFile verifies that when a template
// name is declared in the manifest but the corresponding file is absent from agent/input/,
// the function returns an error.
func TestCollectAndCopyPolicyTemplateFiles_MissingTemplateFile(t *testing.T) {
	inputPkgDir := t.TempDir()
	manifest := []byte(`name: sql
version: 0.1.0
type: input
policy_templates:
  - input: sql
    template_path: missing.yml.hbs
`)
	err := os.WriteFile(filepath.Join(inputPkgDir, "manifest.yml"), manifest, 0644)
	require.NoError(t, err)
	err = os.MkdirAll(filepath.Join(inputPkgDir, "agent", "input"), 0755)
	require.NoError(t, err)
	// Intentionally do NOT create missing.yml.hbs.

	buildRoot := buildRootFor(t)

	_, err = collectAndCopyPolicyTemplateFiles(inputPkgDir, "sql", "agent/input", buildRoot)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing.yml.hbs")
}

// TestCollectAndCopyPolicyTemplateFiles_InvalidPackagePath verifies that a non-existent
// package path returns an error from openPackageFS.
func TestCollectAndCopyPolicyTemplateFiles_InvalidPackagePath(t *testing.T) {
	buildRoot := buildRootFor(t)

	_, err := collectAndCopyPolicyTemplateFiles("/nonexistent/path", "sql", "agent/input", buildRoot)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to open input package")
}

// TestCollectAndCopyPolicyTemplateFiles_CustomDestDir verifies that files are written to the
// caller-supplied destDir, not hardcoded to agent/input. This covers the data-stream use-case
// where destDir is data_stream/<name>/agent/stream.
func TestCollectAndCopyPolicyTemplateFiles_CustomDestDir(t *testing.T) {
	inputPkgDir := createFakeInputHelper(t)
	buildRoot := buildRootFor(t)

	destDir := filepath.Join("data_stream", "logs", "agent", "stream")
	got, err := collectAndCopyPolicyTemplateFiles(inputPkgDir, "sql", destDir, buildRoot)
	require.NoError(t, err)

	assert.Equal(t, []string{"sql-input.yml.hbs"}, got)

	_, err = buildRoot.ReadFile(filepath.Join(destDir, "sql-input.yml.hbs"))
	require.NoError(t, err, "file must be written to the custom destDir")

	// Must NOT appear in agent/input.
	_, err = buildRoot.ReadFile(filepath.Join("agent", "input", "sql-input.yml.hbs"))
	assert.Error(t, err, "file must not be written to agent/input when a custom destDir is given")
}

// TestCollectAndCopyPolicyTemplateFiles_FileContentPreserved verifies that the byte content
// of the template is copied verbatim without modification.
func TestCollectAndCopyPolicyTemplateFiles_FileContentPreserved(t *testing.T) {
	inputPkgDir := t.TempDir()
	originalContent := []byte("{{#each processors}}\n- {{this}}\n{{/each}}")
	manifest := []byte(`name: sql
version: 0.1.0
type: input
policy_templates:
  - input: sql
    template_path: input.yml.hbs
`)
	err := os.WriteFile(filepath.Join(inputPkgDir, "manifest.yml"), manifest, 0644)
	require.NoError(t, err)
	err = os.MkdirAll(filepath.Join(inputPkgDir, "agent", "input"), 0755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(inputPkgDir, "agent", "input", "input.yml.hbs"), originalContent, 0644)
	require.NoError(t, err)

	buildRoot := buildRootFor(t)

	_, err = collectAndCopyPolicyTemplateFiles(inputPkgDir, "sql", "agent/input", buildRoot)
	require.NoError(t, err)

	copied, err := buildRoot.ReadFile(filepath.Join("agent", "input", "sql-input.yml.hbs"))
	require.NoError(t, err)
	assert.Equal(t, originalContent, copied)
}
