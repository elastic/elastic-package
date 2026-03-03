// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package builder

import (
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/registry"
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
	err := bundleInputPackageTemplates(intPkgRoot, intPkgRoot, nil, nil)
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
	err := bundleInputPackageTemplates(pkgRoot, pkgRoot, nil, nil)
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
	eprClient := registry.NewClient(registry.ProductionURL)
	err := bundleInputPackageTemplates(intPkgRoot, intPkgRoot, eprClient, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing_package")
}

// ---------------------------------------------------------------------------
// bundleInputPackageTemplates – success path with local source override
// ---------------------------------------------------------------------------

// TestBundleInputPackageTemplates_WithSourceOverride exercises the full
// bundling path using local test fixture packages. The integration package
// with_input_package_requires requires sql_input; we override the resolution
// to point to the local test_input_pkg fixture, avoiding any registry access.
func TestBundleInputPackageTemplates_WithSourceOverride(t *testing.T) {
	// Resolve paths relative to this test file.
	_, thisFile, _, _ := runtime.Caller(0)
	repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..")
	fixturesRoot := filepath.Join(repoRoot, "test", "packages", "required_inputs")

	integrationSrc := filepath.Join(fixturesRoot, "with_input_package_requires")
	inputPkgSrc := filepath.Join(fixturesRoot, "test_input_pkg")

	// Copy the integration package into a temp dir so the test can modify it.
	buildRoot := filepath.Join(t.TempDir(), "with_input_package_requires")
	require.NoError(t, testCopyDir(integrationSrc, buildRoot))

	overrides := map[string]packages.RequiresOverride{
		"sql_input": {Package: "sql_input", Source: inputPkgSrc},
	}

	err := bundleInputPackageTemplates(buildRoot, buildRoot, nil, overrides)
	require.NoError(t, err)

	agentStreamDir := filepath.Join(buildRoot, "data_stream", "test_logs", "agent", "stream")

	t.Run("templates copied with package prefix", func(t *testing.T) {
		for _, name := range []string{"sql_input-input.yml.hbs", "sql_input-extra.yml.hbs"} {
			destPath := filepath.Join(agentStreamDir, name)
			assert.FileExists(t, destPath, "expected copied template %s", name)
		}

		// Verify content matches the originals.
		assertSameContent(t,
			filepath.Join(inputPkgSrc, "agent", "input", "input.yml.hbs"),
			filepath.Join(agentStreamDir, "sql_input-input.yml.hbs"),
		)
		assertSameContent(t,
			filepath.Join(inputPkgSrc, "agent", "input", "extra.yml.hbs"),
			filepath.Join(agentStreamDir, "sql_input-extra.yml.hbs"),
		)
	})

	t.Run("input-package stream has template_paths", func(t *testing.T) {
		dsManifest, err := packages.ReadDataStreamManifest(
			filepath.Join(buildRoot, "data_stream", "test_logs", packages.DataStreamManifestFile),
		)
		require.NoError(t, err)
		require.Len(t, dsManifest.Streams, 2)

		s := dsManifest.Streams[0]
		assert.Equal(t, []string{"sql_input-input.yml.hbs", "sql_input-extra.yml.hbs"}, s.TemplatePaths)
		assert.Empty(t, s.TemplatePath, "singular template_path should be absent")
	})

	t.Run("non-input stream is unchanged", func(t *testing.T) {
		dsManifest, err := packages.ReadDataStreamManifest(
			filepath.Join(buildRoot, "data_stream", "test_logs", packages.DataStreamManifestFile),
		)
		require.NoError(t, err)
		require.Len(t, dsManifest.Streams, 2)

		s := dsManifest.Streams[1]
		assert.Equal(t, "stream.yml.hbs", s.TemplatePath)
		assert.Empty(t, s.TemplatePaths)
	})

	t.Run("integration own template still exists", func(t *testing.T) {
		assert.FileExists(t, filepath.Join(agentStreamDir, "stream.yml.hbs"))
	})
}

// testCopyDir recursively copies src to dst.
func testCopyDir(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dst, info.Mode()); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())
		if entry.IsDir() {
			if err := testCopyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := testCopyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}
	return nil
}

// testCopyFile copies a single file from src to dst.
func testCopyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

// assertSameContent asserts that two files have identical contents.
func assertSameContent(t *testing.T, wantPath, gotPath string) {
	t.Helper()
	want, err := os.ReadFile(wantPath)
	require.NoError(t, err)
	got, err := os.ReadFile(gotPath)
	require.NoError(t, err)
	assert.Equal(t, string(want), string(got))
}
