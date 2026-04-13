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

// ---- unit tests --------------------------------------------------------------

// TestLoadInputPkgInfo verifies that metadata is correctly extracted from an
// input package manifest directory.
func TestLoadInputPkgInfo(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "manifest.yml"), []byte(`
name: my_input_pkg
title: My Input Package
description: A test input package.
version: 0.1.0
type: input
policy_templates:
  - name: logs
    input: logfile
    type: logs
`), 0644))

	info, err := loadInputPkgInfo(dir)
	require.NoError(t, err)
	assert.Equal(t, "logfile", info.identifier)
	assert.Equal(t, "My Input Package", info.pkgTitle)
	assert.Equal(t, "A test input package.", info.pkgDescription)
}

func TestLoadInputPkgInfo_NoPolicyTemplates(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "manifest.yml"), []byte(`
name: empty_pkg
version: 0.1.0
type: input
`), 0644))

	_, err := loadInputPkgInfo(dir)
	assert.ErrorContains(t, err, "no policy templates")
}

func TestLoadInputPkgInfo_EmptyInputIdentifier(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "manifest.yml"), []byte(`
name: bad_pkg
version: 0.1.0
type: input
policy_templates:
  - name: logs
    type: logs
`), 0644))

	_, err := loadInputPkgInfo(dir)
	assert.ErrorContains(t, err, "no input identifier")
}

// ---- integration tests -------------------------------------------------------

// TestResolveStreamInputTypes_ReplacesPackageWithType verifies that a
// policy_templates[].inputs entry with package: is replaced by type: and that
// the package: key is removed.
func TestResolveStreamInputTypes_ReplacesPackageWithType(t *testing.T) {
	inputPkgDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(inputPkgDir, "manifest.yml"), []byte(`
name: test_input
title: Test Input
description: A test input package.
version: 0.1.0
type: input
policy_templates:
  - name: logs
    input: logfile
    type: logs
`), 0644))

	buildRoot := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(buildRoot, "manifest.yml"), []byte(`
format_version: 3.0.0
name: my_integration
version: 0.1.0
type: integration
requires:
  input:
    - package: test_input
      version: 0.1.0
policy_templates:
  - name: logs
    title: Logs
    description: Collect logs
    inputs:
      - package: test_input
        title: Collect logs via test input
        description: Use the test input to collect logs
`), 0644))

	epr := &fakeEprClient{
		downloadPackageFunc: func(packageName, packageVersion, tmpDir string) (string, error) {
			return inputPkgDir, nil
		},
	}
	resolver := NewRequiredInputsResolver(epr)
	require.NoError(t, resolver.Bundle(buildRoot))

	manifestBytes, err := os.ReadFile(filepath.Join(buildRoot, "manifest.yml"))
	require.NoError(t, err)
	m, err := packages.ReadPackageManifestBytes(manifestBytes)
	require.NoError(t, err)

	require.Len(t, m.PolicyTemplates[0].Inputs, 1)
	assert.Equal(t, "logfile", m.PolicyTemplates[0].Inputs[0].Type)
	assert.Empty(t, m.PolicyTemplates[0].Inputs[0].Package)
}

// TestResolveStreamInputTypes_PreservesExistingTitleAndDescription verifies
// that title and description already set in the composable package input entry
// are preserved and not overwritten by the input package's values.
func TestResolveStreamInputTypes_PreservesExistingTitleAndDescription(t *testing.T) {
	inputPkgDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(inputPkgDir, "manifest.yml"), []byte(`
name: test_input
title: Input Pkg Title
description: Input pkg description.
version: 0.1.0
type: input
policy_templates:
  - name: logs
    input: logfile
    type: logs
`), 0644))

	buildRoot := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(buildRoot, "manifest.yml"), []byte(`
format_version: 3.0.0
name: my_integration
version: 0.1.0
type: integration
requires:
  input:
    - package: test_input
      version: 0.1.0
policy_templates:
  - name: logs
    title: Logs
    description: Collect logs
    inputs:
      - package: test_input
        title: My Custom Title
        description: My custom description.
`), 0644))

	epr := &fakeEprClient{
		downloadPackageFunc: func(packageName, packageVersion, tmpDir string) (string, error) {
			return inputPkgDir, nil
		},
	}
	resolver := NewRequiredInputsResolver(epr)
	require.NoError(t, resolver.Bundle(buildRoot))

	manifestBytes, err := os.ReadFile(filepath.Join(buildRoot, "manifest.yml"))
	require.NoError(t, err)

	// Check raw YAML to verify title/description are preserved verbatim.
	assert.Contains(t, string(manifestBytes), "My Custom Title")
	assert.Contains(t, string(manifestBytes), "My custom description.")
	assert.NotContains(t, string(manifestBytes), "Input Pkg Title")
}

// TestResolveStreamInputTypes_PopulatesTitleFromInputPkg verifies that when
// the composable package input entry has no title/description, they are
// populated from the input package manifest.
func TestResolveStreamInputTypes_PopulatesTitleFromInputPkg(t *testing.T) {
	inputPkgDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(inputPkgDir, "manifest.yml"), []byte(`
name: test_input
title: Input Pkg Title
description: Input pkg description.
version: 0.1.0
type: input
policy_templates:
  - name: logs
    input: logfile
    type: logs
`), 0644))

	buildRoot := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(buildRoot, "manifest.yml"), []byte(`
format_version: 3.0.0
name: my_integration
version: 0.1.0
type: integration
requires:
  input:
    - package: test_input
      version: 0.1.0
policy_templates:
  - name: logs
    title: Logs
    description: Collect logs
    inputs:
      - package: test_input
`), 0644))

	epr := &fakeEprClient{
		downloadPackageFunc: func(packageName, packageVersion, tmpDir string) (string, error) {
			return inputPkgDir, nil
		},
	}
	resolver := NewRequiredInputsResolver(epr)
	require.NoError(t, resolver.Bundle(buildRoot))

	manifestBytes, err := os.ReadFile(filepath.Join(buildRoot, "manifest.yml"))
	require.NoError(t, err)

	assert.Contains(t, string(manifestBytes), "Input Pkg Title")
	assert.Contains(t, string(manifestBytes), "Input pkg description.")
}

// TestResolveStreamInputTypes_SkipsNonPackageInputs verifies that inputs
// declared with type: (no package:) are not modified.
func TestResolveStreamInputTypes_SkipsNonPackageInputs(t *testing.T) {
	inputPkgDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(inputPkgDir, "manifest.yml"), []byte(`
name: test_input
title: Test Input
version: 0.1.0
type: input
policy_templates:
  - name: logs
    input: logfile
    type: logs
`), 0644))

	buildRoot := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(buildRoot, "manifest.yml"), []byte(`
format_version: 3.0.0
name: my_integration
version: 0.1.0
type: integration
requires:
  input:
    - package: test_input
      version: 0.1.0
policy_templates:
  - name: logs
    title: Logs
    description: Collect logs
    inputs:
      - package: test_input
        title: From pkg
        description: From pkg.
      - type: metrics
        title: Direct metrics
        description: Direct metrics input.
`), 0644))

	epr := &fakeEprClient{
		downloadPackageFunc: func(packageName, packageVersion, tmpDir string) (string, error) {
			return inputPkgDir, nil
		},
	}
	resolver := NewRequiredInputsResolver(epr)
	require.NoError(t, resolver.Bundle(buildRoot))

	manifestBytes, err := os.ReadFile(filepath.Join(buildRoot, "manifest.yml"))
	require.NoError(t, err)
	m, err := packages.ReadPackageManifestBytes(manifestBytes)
	require.NoError(t, err)

	require.Len(t, m.PolicyTemplates[0].Inputs, 2)
	assert.Equal(t, "logfile", m.PolicyTemplates[0].Inputs[0].Type)
	assert.Empty(t, m.PolicyTemplates[0].Inputs[0].Package)
	assert.Equal(t, "metrics", m.PolicyTemplates[0].Inputs[1].Type)
	assert.Empty(t, m.PolicyTemplates[0].Inputs[1].Package)
}

// TestResolveStreamInputTypes_DataStreamStreamReplacement verifies that
// streams[].package in data stream manifests is replaced with streams[].input.
func TestResolveStreamInputTypes_DataStreamStreamReplacement(t *testing.T) {
	inputPkgDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(inputPkgDir, "manifest.yml"), []byte(`
name: test_input
title: Test Input
description: Test input pkg.
version: 0.1.0
type: input
policy_templates:
  - name: logs
    input: logfile
    type: logs
`), 0644))

	buildRoot := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(buildRoot, "manifest.yml"), []byte(`
format_version: 3.0.0
name: my_integration
version: 0.1.0
type: integration
requires:
  input:
    - package: test_input
      version: 0.1.0
policy_templates:
  - name: logs
    title: Logs
    description: Collect logs
    data_streams:
      - test_logs
    inputs:
      - package: test_input
        title: Collect logs
        description: Collect logs.
`), 0644))

	dsDir := filepath.Join(buildRoot, "data_stream", "test_logs")
	require.NoError(t, os.MkdirAll(dsDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dsDir, "manifest.yml"), []byte(`
title: Test Logs
type: logs
streams:
  - package: test_input
    title: Test log stream
    description: Collect test logs.
`), 0644))

	epr := &fakeEprClient{
		downloadPackageFunc: func(packageName, packageVersion, tmpDir string) (string, error) {
			return inputPkgDir, nil
		},
	}
	resolver := NewRequiredInputsResolver(epr)
	require.NoError(t, resolver.Bundle(buildRoot))
	dsManifestBytes, err := os.ReadFile(filepath.Join(dsDir, "manifest.yml"))
	require.NoError(t, err)
	dsManifest, err := packages.ReadDataStreamManifestBytes(dsManifestBytes)
	require.NoError(t, err)

	require.Len(t, dsManifest.Streams, 1)
	assert.Equal(t, "logfile", dsManifest.Streams[0].Input)
	assert.Empty(t, dsManifest.Streams[0].Package)
	assert.Equal(t, "Test log stream", dsManifest.Streams[0].Title)
}

// TestResolveStreamInputTypes_SkipsNonPackageStreams verifies that streams
// declared with input: (no package:) are not modified.
func TestResolveStreamInputTypes_SkipsNonPackageStreams(t *testing.T) {
	inputPkgDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(inputPkgDir, "manifest.yml"), []byte(`
name: test_input
title: Test Input
version: 0.1.0
type: input
policy_templates:
  - name: logs
    input: logfile
    type: logs
`), 0644))

	buildRoot := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(buildRoot, "manifest.yml"), []byte(`
format_version: 3.0.0
name: my_integration
version: 0.1.0
type: integration
requires:
  input:
    - package: test_input
      version: 0.1.0
policy_templates:
  - name: logs
    title: Logs
    description: Collect logs
    data_streams:
      - test_logs
    inputs:
      - package: test_input
        title: Collect logs
        description: Collect logs.
`), 0644))

	dsDir := filepath.Join(buildRoot, "data_stream", "test_logs")
	require.NoError(t, os.MkdirAll(dsDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dsDir, "manifest.yml"), []byte(`
title: Test Logs
type: logs
streams:
  - package: test_input
    title: From pkg
    description: From pkg.
  - input: metrics
    title: Direct metrics
    description: Direct metrics stream.
`), 0644))

	epr := &fakeEprClient{
		downloadPackageFunc: func(packageName, packageVersion, tmpDir string) (string, error) {
			return inputPkgDir, nil
		},
	}
	resolver := NewRequiredInputsResolver(epr)
	require.NoError(t, resolver.Bundle(buildRoot))

	dsManifestBytes, err := os.ReadFile(filepath.Join(dsDir, "manifest.yml"))
	require.NoError(t, err)
	dsManifest, err := packages.ReadDataStreamManifestBytes(dsManifestBytes)
	require.NoError(t, err)

	require.Len(t, dsManifest.Streams, 2)
	assert.Equal(t, "logfile", dsManifest.Streams[0].Input)
	assert.Empty(t, dsManifest.Streams[0].Package)
	assert.Equal(t, "metrics", dsManifest.Streams[1].Input)
	assert.Empty(t, dsManifest.Streams[1].Package)
}

// TestResolveStreamInputTypes_FieldBundlingFixture runs the full
// Bundle pipeline on the with_field_bundling fixture and
// verifies that package: references are replaced in both the main manifest and
// the data stream manifest.
func TestResolveStreamInputTypes_FieldBundlingFixture(t *testing.T) {
	buildPackageRoot := copyFixturePackage(t, "with_field_bundling")
	resolver := NewRequiredInputsResolver(makeFakeEprForFieldBundling(t))
	require.NoError(t, resolver.Bundle(buildPackageRoot))

	// Check main manifest: package: fields_input_pkg → type: logfile
	manifestBytes, err := os.ReadFile(filepath.Join(buildPackageRoot, "manifest.yml"))
	require.NoError(t, err)
	m, err := packages.ReadPackageManifestBytes(manifestBytes)
	require.NoError(t, err)
	require.Len(t, m.PolicyTemplates[0].Inputs, 1)
	assert.Equal(t, "logfile", m.PolicyTemplates[0].Inputs[0].Type)
	assert.Empty(t, m.PolicyTemplates[0].Inputs[0].Package)

	// Check data stream manifest: package: fields_input_pkg → input: logfile
	dsManifestBytes, err := os.ReadFile(filepath.Join(buildPackageRoot, "data_stream", "field_logs", "manifest.yml"))
	require.NoError(t, err)
	dsManifest, err := packages.ReadDataStreamManifestBytes(dsManifestBytes)
	require.NoError(t, err)
	require.Len(t, dsManifest.Streams, 1)
	assert.Equal(t, "logfile", dsManifest.Streams[0].Input)
	assert.Empty(t, dsManifest.Streams[0].Package)
	assert.NotEmpty(t, dsManifest.Streams[0].Title)
}
