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
	"gopkg.in/yaml.v3"

	"github.com/elastic/elastic-package/internal/files"
	"github.com/elastic/elastic-package/internal/packages"
)

// ---------------------------------------------------------------------------
// setStreamTemplatePaths
// ---------------------------------------------------------------------------

func TestSetStreamTemplatePaths(t *testing.T) {
	load := func(t *testing.T, name string) *yaml.Node {
		t.Helper()
		raw, err := os.ReadFile(filepath.Join("testdata", "input_packages", name))
		require.NoError(t, err)
		var doc yaml.Node
		require.NoError(t, yaml.Unmarshal(raw, &doc))
		return &doc
	}

	t.Run("replaces template_path with template_paths", func(t *testing.T) {
		doc := load(t, "ds_manifest_with_template_path.yml")
		err := setStreamTemplatePaths(doc, 0, []string{"input-pkg-input.yml.hbs", "stream.yml.hbs"})
		require.NoError(t, err)

		out, err := formatYAMLNode(doc)
		require.NoError(t, err)

		var result struct {
			Streams []struct {
				TemplatePath  string   `yaml:"template_path"`
				TemplatePaths []string `yaml:"template_paths"`
			} `yaml:"streams"`
		}
		require.NoError(t, yaml.Unmarshal(out, &result))

		require.Len(t, result.Streams, 1)
		assert.Empty(t, result.Streams[0].TemplatePath, "template_path should be removed")
		assert.Equal(t, []string{"input-pkg-input.yml.hbs", "stream.yml.hbs"}, result.Streams[0].TemplatePaths)
	})

	t.Run("replaces existing template_paths", func(t *testing.T) {
		doc := load(t, "ds_manifest_with_template_paths.yml")
		err := setStreamTemplatePaths(doc, 0, []string{"input-pkg-input.yml.hbs", "stream.yml.hbs", "extra.yml.hbs"})
		require.NoError(t, err)

		out, err := formatYAMLNode(doc)
		require.NoError(t, err)

		var result struct {
			Streams []struct {
				TemplatePath  string   `yaml:"template_path"`
				TemplatePaths []string `yaml:"template_paths"`
			} `yaml:"streams"`
		}
		require.NoError(t, yaml.Unmarshal(out, &result))

		require.Len(t, result.Streams, 1)
		assert.Empty(t, result.Streams[0].TemplatePath)
		assert.Equal(t, []string{"input-pkg-input.yml.hbs", "stream.yml.hbs", "extra.yml.hbs"}, result.Streams[0].TemplatePaths)
	})

	t.Run("sets template_paths when no prior template field", func(t *testing.T) {
		doc := load(t, "ds_manifest_no_template.yml")
		err := setStreamTemplatePaths(doc, 0, []string{"input-pkg-input.yml.hbs"})
		require.NoError(t, err)

		out, err := formatYAMLNode(doc)
		require.NoError(t, err)

		var result struct {
			Streams []struct {
				TemplatePaths []string `yaml:"template_paths"`
			} `yaml:"streams"`
		}
		require.NoError(t, yaml.Unmarshal(out, &result))

		require.Len(t, result.Streams, 1)
		assert.Equal(t, []string{"input-pkg-input.yml.hbs"}, result.Streams[0].TemplatePaths)
	})

	t.Run("invalid stream index", func(t *testing.T) {
		doc := load(t, "ds_manifest_no_template.yml")
		err := setStreamTemplatePaths(doc, 5, []string{"input.yml.hbs"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "out of range")
	})
}

// ---------------------------------------------------------------------------
// bundleInputPackageTemplates – edge cases
// ---------------------------------------------------------------------------

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))
}

func TestBundleInputPackageTemplates_NoRequires(t *testing.T) {
	root := t.TempDir()
	intPkgRoot := filepath.Join(root, "packages", "simple_integration")
	require.NoError(t, os.MkdirAll(intPkgRoot, 0755))
	writeFile(t, filepath.Join(intPkgRoot, "manifest.yml"), `
format_version: 3.6.0
name: simple_integration
title: Simple
description: No requires.
version: 1.0.0
type: integration
categories: [custom]
conditions:
  kibana:
    version: "^8.0.0"
  elastic:
    subscription: basic
policy_templates: []
owner:
  github: elastic/integrations
  type: elastic
`)
	err := bundleInputPackageTemplates(intPkgRoot, intPkgRoot, "", nil)
	require.NoError(t, err)
}

func TestBundleInputPackageTemplates_InputPackage(t *testing.T) {
	root := t.TempDir()
	pkgRoot := filepath.Join(root, "packages", "input_pkg")
	require.NoError(t, os.MkdirAll(pkgRoot, 0755))
	writeFile(t, filepath.Join(pkgRoot, "manifest.yml"), `
format_version: 3.6.0
name: input_pkg
title: Input
description: Input pkg.
version: 1.0.0
type: input
categories: [custom]
conditions:
  kibana:
    version: "^8.0.0"
  elastic:
    subscription: basic
policy_templates:
  - name: test
    type: logs
    title: Test
    description: Test.
    input: logfile
    template_path: input.yml.hbs
    vars: []
owner:
  github: elastic/integrations
  type: elastic
`)
	err := bundleInputPackageTemplates(pkgRoot, pkgRoot, "", nil)
	require.NoError(t, err)
}

func TestBundleInputPackageTemplates_RequiredPkgNotFound(t *testing.T) {
	root := t.TempDir()
	intPkgRoot := filepath.Join(root, "packages", "my_integration")
	require.NoError(t, os.MkdirAll(intPkgRoot, 0755))
	writeFile(t, filepath.Join(intPkgRoot, "manifest.yml"), `
format_version: 3.6.0
name: my_integration
title: My Integration
description: Has a missing required input package.
version: 1.0.0
type: integration
categories: [custom]
conditions:
  kibana:
    version: "^8.0.0"
  elastic:
    subscription: basic
requires:
  input:
    - package: missing_package
      version: "1.0.0"
policy_templates: []
owner:
  github: elastic/integrations
  type: elastic
`)
	err := bundleInputPackageTemplates(intPkgRoot, intPkgRoot, "", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing_package")
}

// ---------------------------------------------------------------------------
// bundleInputPackageTemplates – integration tests using real package fixtures
// ---------------------------------------------------------------------------

func TestBundleInputPackageTemplates_TemplatesAreBundled(t *testing.T) {
	repoRoot, err := files.FindRepositoryRoot()
	require.NoError(t, err)
	t.Cleanup(func() { repoRoot.Close() })
	root := repoRoot.Name()

	fixture := filepath.Join(root, "test", "packages", "required_inputs", "with_input_package_requires")

	// Copy the integration package fixture to a temp directory so that
	// bundleInputPackageTemplates can modify its data stream manifests without
	// touching the on-disk fixture.
	buildDir := t.TempDir()
	require.NoError(t, files.CopyAll(fixture, buildDir))

	overrides := map[string]packages.RequiresOverride{
		"test_input_pkg": {Package: "test_input_pkg", Source: "../test_input_pkg"},
	}
	err = bundleInputPackageTemplates(fixture, buildDir, "", overrides)
	require.NoError(t, err)

	agentStreamDir := filepath.Join(buildDir, "data_stream", "test_logs", "agent", "stream")
	inputPkgDir := filepath.Join(root, "test", "packages", "required_inputs", "test_input_pkg", "agent", "input")

	// Input package templates must be copied with the "<pkgname>-" prefix and
	// their content must be identical to the source files in the input package.
	for _, name := range []string{"input.yml.hbs", "extra.yml.hbs"} {
		copied := filepath.Join(agentStreamDir, "test_input_pkg-"+name)
		source := filepath.Join(inputPkgDir, name)

		require.FileExists(t, copied)

		copiedContent, err := os.ReadFile(copied)
		require.NoError(t, err)
		sourceContent, err := os.ReadFile(source)
		require.NoError(t, err)

		assert.Equal(t, sourceContent, copiedContent, "copied template %q must match source", name)
	}

	// The integration's own stream template must be preserved.
	assert.FileExists(t, filepath.Join(agentStreamDir, "stream.yml.hbs"))

	raw, err := os.ReadFile(filepath.Join(buildDir, "data_stream", "test_logs", "manifest.yml"))
	require.NoError(t, err)

	var dsManifest struct {
		Streams []struct {
			TemplatePath  string   `yaml:"template_path"`
			TemplatePaths []string `yaml:"template_paths"`
		} `yaml:"streams"`
	}
	require.NoError(t, yaml.Unmarshal(raw, &dsManifest))

	require.Len(t, dsManifest.Streams, 2)
	s := dsManifest.Streams[0]

	assert.Empty(t, s.TemplatePath, "singular template_path must be removed after bundling")
	// Order is driven by the input package manifest's template_paths declaration.
	// Stream 0 has no own template, so only the input package templates are listed.
	assert.Equal(t, []string{
		"test_input_pkg-input.yml.hbs",
		"test_input_pkg-extra.yml.hbs",
	}, s.TemplatePaths)

	// Stream 1 (input: logs) is not touched by bundling — its template_path must be unchanged.
	s1 := dsManifest.Streams[1]
	assert.Equal(t, "stream.yml.hbs", s1.TemplatePath, "stream 1 template_path must be unchanged")
	assert.Empty(t, s1.TemplatePaths, "stream 1 must not gain template_paths")

	// Only the two files declared in the input package manifest must be present;
	// no undeclared files from agent/input/ should be bundled.
	entries, err := os.ReadDir(agentStreamDir)
	require.NoError(t, err)
	var bundledNames []string
	for _, e := range entries {
		bundledNames = append(bundledNames, e.Name())
	}
	assert.ElementsMatch(t, []string{
		"test_input_pkg-input.yml.hbs",
		"test_input_pkg-extra.yml.hbs",
		"stream.yml.hbs",
	}, bundledNames, "agent/stream should contain exactly the declared templates")
}

// TestBundleInputPackageTemplates_FixtureIsUnmodified verifies that re-running
// bundleInputPackageTemplates on the real fixture directory does not modify the
// source files (the fixture itself is the source, not the build output).
func TestBundleInputPackageTemplates_FixtureIsUnmodified(t *testing.T) {
	repoRoot, err := files.FindRepositoryRoot()
	require.NoError(t, err)
	t.Cleanup(func() { repoRoot.Close() })
	root := repoRoot.Name()

	fixture := filepath.Join(root, "test", "packages", "required_inputs", "with_input_package_requires")
	dsManifestPath := filepath.Join(fixture, "data_stream", "test_logs", "manifest.yml")

	original, err := os.ReadFile(dsManifestPath)
	require.NoError(t, err)

	// bundleInputPackageTemplates operates on a copy (buildDir), not on fixture.
	buildDir := t.TempDir()
	require.NoError(t, files.CopyAll(fixture, buildDir))
	overrides := map[string]packages.RequiresOverride{
		"test_input_pkg": {Package: "test_input_pkg", Source: "../test_input_pkg"},
	}
	require.NoError(t, bundleInputPackageTemplates(fixture, buildDir, "", overrides))

	after, err := os.ReadFile(dsManifestPath)
	require.NoError(t, err)

	assert.Equal(t, string(original), string(after), "source fixture must not be modified by the build step")
}
