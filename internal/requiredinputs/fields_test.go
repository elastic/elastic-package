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
	"gopkg.in/yaml.v3"
)

// ---- unit tests --------------------------------------------------------------

// TestLoadFieldNodesFromBytes verifies that field YAML sequences are parsed
// correctly into individual yaml.Node pointers.
func TestLoadFieldNodesFromBytes(t *testing.T) {
	t.Run("valid sequence", func(t *testing.T) {
		data := []byte(`
- name: data_stream.type
  type: constant_keyword
  description: Data stream type.
- name: message
  type: text
  description: Log message.
`)
		nodes, err := loadFieldNodesFromBytes(data)
		require.NoError(t, err)
		require.Len(t, nodes, 2)
		assert.Equal(t, "data_stream.type", fieldNodeName(nodes[0]))
		assert.Equal(t, "message", fieldNodeName(nodes[1]))
	})

	t.Run("empty document", func(t *testing.T) {
		nodes, err := loadFieldNodesFromBytes([]byte(""))
		require.NoError(t, err)
		assert.Empty(t, nodes)
	})

	t.Run("invalid YAML", func(t *testing.T) {
		_, err := loadFieldNodesFromBytes([]byte(":\t:invalid"))
		assert.Error(t, err)
	})

	t.Run("non-sequence root", func(t *testing.T) {
		data := []byte(`name: foo\ntype: keyword`)
		_, err := loadFieldNodesFromBytes(data)
		assert.Error(t, err)
	})
}

// TestFieldNodeName verifies extraction of the "name" field from a YAML
// mapping node representing a field definition.
func TestFieldNodeName(t *testing.T) {
	t.Run("node with name", func(t *testing.T) {
		n := &yaml.Node{Kind: yaml.MappingNode}
		upsertKey(n, "name", &yaml.Node{Kind: yaml.ScalarNode, Value: "message"})
		assert.Equal(t, "message", fieldNodeName(n))
	})

	t.Run("node without name", func(t *testing.T) {
		n := &yaml.Node{Kind: yaml.MappingNode}
		assert.Equal(t, "", fieldNodeName(n))
	})

	t.Run("nil node", func(t *testing.T) {
		assert.Equal(t, "", fieldNodeName(nil))
	})
}

// TestCollectExistingFieldNames verifies that field names are collected from
// all YAML files in a data stream's fields/ directory.
func TestCollectExistingFieldNames(t *testing.T) {
	t.Run("collects names from multiple files", func(t *testing.T) {
		tmpDir := t.TempDir()
		buildRoot, err := os.OpenRoot(tmpDir)
		require.NoError(t, err)
		defer buildRoot.Close()

		require.NoError(t, buildRoot.MkdirAll("data_stream/logs/fields", 0755))
		require.NoError(t, buildRoot.WriteFile("data_stream/logs/fields/base-fields.yml", []byte(`
- name: "@timestamp"
  type: date
- name: data_stream.type
  type: constant_keyword
`), 0644))
		require.NoError(t, buildRoot.WriteFile("data_stream/logs/fields/extra-fields.yml", []byte(`
- name: message
  type: text
`), 0644))

		names, err := collectExistingFieldNames("data_stream/logs", buildRoot)
		require.NoError(t, err)
		assert.True(t, names["@timestamp"])
		assert.True(t, names["data_stream.type"])
		assert.True(t, names["message"])
		assert.Len(t, names, 3)
	})

	t.Run("returns empty set when fields directory does not exist", func(t *testing.T) {
		tmpDir := t.TempDir()
		buildRoot, err := os.OpenRoot(tmpDir)
		require.NoError(t, err)
		defer buildRoot.Close()

		require.NoError(t, buildRoot.MkdirAll("data_stream/logs", 0755))

		names, err := collectExistingFieldNames("data_stream/logs", buildRoot)
		require.NoError(t, err)
		assert.Empty(t, names)
	})
}

// ---- integration tests -------------------------------------------------------

// makeFakeEprForFieldBundling supplies the fields_input_pkg fixture path as if
// it were downloaded from the registry, so integration tests do not need a
// running stack.
func makeFakeEprForFieldBundling(t *testing.T) *fakeEprClient {
	t.Helper()
	inputPkgPath := filepath.Join("..", "..", "test", "manual_packages", "required_inputs", "fields_input_pkg")
	return &fakeEprClient{
		downloadPackageFunc: func(packageName, packageVersion, tmpDir string) (string, error) {
			return inputPkgPath, nil
		},
	}
}

// TestBundleDataStreamFields_PartialOverlap verifies the primary field bundling
// scenario: fields already present in the integration data stream are skipped
// (integration wins), and only fields unique to the input package are written
// to <datastream>/fields/<inputPkgName>-fields.yml.
func TestBundleDataStreamFields_PartialOverlap(t *testing.T) {
	// with_field_bundling has data_stream/field_logs/fields/base-fields.yml with
	// 4 common fields. fields_input_pkg has those same 4 plus "message" and
	// "log.level". After bundling, only "message" and "log.level" should appear
	// in the generated file.
	buildPackageRoot := copyFixturePackage(t, "with_field_bundling")
	resolver, err := NewRequiredInputsResolver(makeFakeEprForFieldBundling(t))
	require.NoError(t, err)

	err = resolver.Bundle(buildPackageRoot)
	require.NoError(t, err)

	bundledPath := filepath.Join(buildPackageRoot, "data_stream", "field_logs", "fields", "fields_input_pkg-fields.yml")
	data, err := os.ReadFile(bundledPath)
	require.NoError(t, err, "bundled fields file should exist")

	nodes, err := loadFieldNodesFromBytes(data)
	require.NoError(t, err)
	require.Len(t, nodes, 2)

	names := make([]string, 0, len(nodes))
	for _, n := range nodes {
		names = append(names, fieldNodeName(n))
	}
	assert.ElementsMatch(t, []string{"message", "log.level"}, names)

	// Original base-fields.yml must be untouched.
	originalData, err := os.ReadFile(filepath.Join(buildPackageRoot, "data_stream", "field_logs", "fields", "base-fields.yml"))
	require.NoError(t, err)
	originalNodes, err := loadFieldNodesFromBytes(originalData)
	require.NoError(t, err)
	assert.Len(t, originalNodes, 4)
}

// TestBundleDataStreamFields_AllFieldsOverlap verifies that when all fields in
// the input package are already present in the integration data stream, no
// bundled file is created (nothing to add).
func TestBundleDataStreamFields_AllFieldsOverlap(t *testing.T) {
	// with_input_package_requires has data_stream/test_logs/fields/base-fields.yml
	// with the same 4 fields as test_input_pkg. No new fields → no output file.
	inputPkgPath := filepath.Join("..", "..", "test", "manual_packages", "required_inputs", "test_input_pkg")
	epr := &fakeEprClient{
		downloadPackageFunc: func(packageName, packageVersion, tmpDir string) (string, error) {
			return inputPkgPath, nil
		},
	}

	buildPackageRoot := copyFixturePackage(t, "with_input_package_requires")
	resolver, err := NewRequiredInputsResolver(epr)
	require.NoError(t, err)

	err = resolver.Bundle(buildPackageRoot)
	require.NoError(t, err)

	bundledPath := filepath.Join(buildPackageRoot, "data_stream", "test_logs", "fields", "test_input_pkg-fields.yml")
	_, statErr := os.Stat(bundledPath)
	assert.True(t, os.IsNotExist(statErr), "bundled fields file should not be created when all fields already exist")
}

// TestBundleDataStreamFields_NoFieldsInInputPkg verifies that when the input
// package has no fields/ directory, no error occurs and no file is written.
func TestBundleDataStreamFields_NoFieldsInInputPkg(t *testing.T) {
	// Create a minimal input package without a fields/ directory.
	inputPkgDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(inputPkgDir, "manifest.yml"), []byte(`
name: no_fields_pkg
version: 0.1.0
type: input
policy_templates:
  - name: t
    input: logfile
    template_path: input.yml.hbs
`), 0644))
	require.NoError(t, os.MkdirAll(filepath.Join(inputPkgDir, "agent", "input"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(inputPkgDir, "agent", "input", "input.yml.hbs"), []byte(""), 0644))

	epr := &fakeEprClient{
		downloadPackageFunc: func(packageName, packageVersion, tmpDir string) (string, error) {
			return inputPkgDir, nil
		},
	}

	buildPackageRoot := copyFixturePackage(t, "with_field_bundling")
	// Patch manifest to reference no_fields_pkg instead.
	manifestPath := filepath.Join(buildPackageRoot, "manifest.yml")
	manifestData, err := os.ReadFile(manifestPath)
	require.NoError(t, err)
	patched := []byte(`format_version: 3.6.0
name: with_field_bundling
title: Integration With Field Bundling
version: 0.1.0
type: integration
categories:
  - custom
conditions:
  kibana:
    version: "^8.0.0"
  elastic:
    subscription: basic
requires:
  input:
    - package: no_fields_pkg
      version: "0.1.0"
policy_templates:
  - name: field_logs
    title: Field Logs
    description: Collect logs
    data_streams:
      - field_logs
    inputs:
      - package: no_fields_pkg
        title: Collect logs
        description: Use the no fields input package
owner:
  github: elastic/integrations
  type: elastic
`)
	_ = manifestData // not used further
	require.NoError(t, os.WriteFile(manifestPath, patched, 0644))

	// Also patch the data stream manifest to reference no_fields_pkg.
	dsManifestPath := filepath.Join(buildPackageRoot, "data_stream", "field_logs", "manifest.yml")
	require.NoError(t, os.WriteFile(dsManifestPath, []byte(`title: Field Logs
type: logs
streams:
  - package: no_fields_pkg
    title: Field Logs
    description: Collect field logs.
`), 0644))

	resolver, err := NewRequiredInputsResolver(epr)
	require.NoError(t, err)

	err = resolver.Bundle(buildPackageRoot)
	require.NoError(t, err)

	// No bundled fields file should be created.
	bundledPath := filepath.Join(buildPackageRoot, "data_stream", "field_logs", "fields", "no_fields_pkg-fields.yml")
	_, statErr := os.Stat(bundledPath)
	assert.True(t, os.IsNotExist(statErr), "no fields file should be created when input package has no fields")
}

// TestBundleDataStreamFields_StreamWithoutPackage verifies that data stream
// streams with no package reference are skipped without error.
func TestBundleDataStreamFields_StreamWithoutPackage(t *testing.T) {
	// with_input_package_requires has a second stream with input: logs (no package).
	// The test confirms this is processed without error and no unexpected files appear.
	inputPkgPath := filepath.Join("..", "..", "test", "manual_packages", "required_inputs", "test_input_pkg")
	epr := &fakeEprClient{
		downloadPackageFunc: func(packageName, packageVersion, tmpDir string) (string, error) {
			return inputPkgPath, nil
		},
	}

	buildPackageRoot := copyFixturePackage(t, "with_input_package_requires")
	resolver, err := NewRequiredInputsResolver(epr)
	require.NoError(t, err)

	err = resolver.Bundle(buildPackageRoot)
	require.NoError(t, err)

	// The non-package stream (logs input) should not produce a bundled fields file.
	_, statErr := os.Stat(filepath.Join(buildPackageRoot, "data_stream", "test_logs", "fields", "-fields.yml"))
	assert.True(t, os.IsNotExist(statErr))
}
