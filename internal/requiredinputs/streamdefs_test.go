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

// TestLoadInputPkgInfo_MultiplePolicyTemplatesUsesFirstInput verifies that when
// an input package declares more than one policy template, loadInputPkgInfo
// keeps the input identifier from the first template (see streamdefs.go). This
// matches resolveStreamInputTypes behavior and the warning logged for the
// ambiguous multi-template case.
func TestLoadInputPkgInfo_MultiplePolicyTemplatesUsesFirstInput(t *testing.T) {
	dir := createFakeInputWithMultiplePolicyTemplates(t)
	info, err := loadInputPkgInfo(dir)
	require.NoError(t, err)
	assert.Equal(t, "sql", info.identifier)
	assert.NotEqual(t, "sql/metrics", info.identifier)
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

// TestResolveStreamInputTypes_InputPkgWithMultiplePolicyTemplatesUsesFirst
// exercises Bundle when the required input package has several policy
// templates with different input identifiers: resolution must use the first
// template only so composable manifests stay consistent with loadInputPkgInfo.
func TestResolveStreamInputTypes_InputPkgWithMultiplePolicyTemplatesUsesFirst(t *testing.T) {
	inputPkgDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(inputPkgDir, "manifest.yml"), []byte(`
name: dual_template_input
title: Dual Template Input
description: Input with two policy templates.
version: 0.1.0
type: input
policy_templates:
  - name: first
    input: logfile
    type: logs
  - name: second
    input: winlog
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
    - package: dual_template_input
      version: 0.1.0
policy_templates:
  - name: logs
    title: Logs
    description: Collect logs
    inputs:
      - package: dual_template_input
        title: Collect logs via dual-template input
        description: Use the input package
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

// TestBuildStreamInputRefs_NoDuplicate verifies that when all required packages have unique
// input types within every policy template, the refs map contains the type identifier (no
// disambiguation needed, so no name will be set on the input).
func TestBuildStreamInputRefs_NoDuplicate(t *testing.T) {
	manifest := &packages.PackageManifest{
		PolicyTemplates: []packages.PolicyTemplate{
			{
				Inputs: []packages.Input{
					{Package: "pkg_a"},
					{Package: "pkg_b"},
				},
			},
		},
	}
	infoByPkg := map[string]inputPkgInfo{
		"pkg_a": {pkgName: "pkg_a", identifier: "logfile"},
		"pkg_b": {pkgName: "pkg_b", identifier: "winlog"},
	}

	refs := buildStreamInputRefs(manifest, infoByPkg)

	assert.Equal(t, "logfile", refs["pkg_a"])
	assert.Equal(t, "winlog", refs["pkg_b"])
}

// TestBuildStreamInputRefs_DuplicateType verifies that when two required packages in the same
// policy template resolve to the same type, both are assigned their package name as the qualifier.
func TestBuildStreamInputRefs_DuplicateType(t *testing.T) {
	manifest := &packages.PackageManifest{
		PolicyTemplates: []packages.PolicyTemplate{
			{
				Inputs: []packages.Input{
					{Package: "pkg_a"},
					{Package: "pkg_b"},
				},
			},
		},
	}
	infoByPkg := map[string]inputPkgInfo{
		"pkg_a": {pkgName: "pkg_a", identifier: "otelcol"},
		"pkg_b": {pkgName: "pkg_b", identifier: "otelcol"},
	}

	refs := buildStreamInputRefs(manifest, infoByPkg)

	assert.Equal(t, "pkg_a", refs["pkg_a"])
	assert.Equal(t, "pkg_b", refs["pkg_b"])
}

// TestResolveStreamInputTypes_DuplicateTypeInputs verifies that when two required input packages
// share the same type within a policy template, Bundle sets a unique name on each input and
// uses that name as the stream.input in data stream manifests.
func TestResolveStreamInputTypes_DuplicateTypeInputs(t *testing.T) {
	pkgADir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(pkgADir, "manifest.yml"), []byte(`
name: otelcol_logs_pkg
title: OTel Logs
description: Logs via otelcol.
version: 0.1.0
type: input
policy_templates:
  - name: logs
    input: otelcol
    type: logs
`), 0644))

	pkgBDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(pkgBDir, "manifest.yml"), []byte(`
name: otelcol_metrics_pkg
title: OTel Metrics
description: Metrics via otelcol.
version: 0.1.0
type: input
policy_templates:
  - name: metrics
    input: otelcol
    type: metrics
`), 0644))

	buildRoot := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(buildRoot, "manifest.yml"), []byte(`
format_version: 3.6.0
name: my_integration
version: 0.1.0
type: integration
requires:
  input:
    - package: otelcol_logs_pkg
      version: 0.1.0
    - package: otelcol_metrics_pkg
      version: 0.1.0
policy_templates:
  - name: otel
    title: OTel
    description: Collect via OTel
    data_streams:
      - otel_logs
      - otel_metrics
    inputs:
      - package: otelcol_logs_pkg
        title: OTel logs
        description: Collect logs via otelcol.
      - package: otelcol_metrics_pkg
        title: OTel metrics
        description: Collect metrics via otelcol.
`), 0644))

	logsDir := filepath.Join(buildRoot, "data_stream", "otel_logs")
	require.NoError(t, os.MkdirAll(logsDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(logsDir, "manifest.yml"), []byte(`
title: OTel Logs
type: logs
streams:
  - package: otelcol_logs_pkg
    title: OTel log stream
`), 0644))

	metricsDir := filepath.Join(buildRoot, "data_stream", "otel_metrics")
	require.NoError(t, os.MkdirAll(metricsDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(metricsDir, "manifest.yml"), []byte(`
title: OTel Metrics
type: metrics
streams:
  - package: otelcol_metrics_pkg
    title: OTel metrics stream
`), 0644))

	downloadCalls := map[string]string{
		"otelcol_logs_pkg":    pkgADir,
		"otelcol_metrics_pkg": pkgBDir,
	}
	epr := &fakeEprClient{
		downloadPackageFunc: func(packageName, packageVersion, tmpDir string) (string, error) {
			return downloadCalls[packageName], nil
		},
	}
	resolver := NewRequiredInputsResolver(epr)
	require.NoError(t, resolver.Bundle(buildRoot))

	manifestBytes, err := os.ReadFile(filepath.Join(buildRoot, "manifest.yml"))
	require.NoError(t, err)
	m, err := packages.ReadPackageManifestBytes(manifestBytes)
	require.NoError(t, err)

	require.Len(t, m.PolicyTemplates[0].Inputs, 2)
	assert.Equal(t, "otelcol", m.PolicyTemplates[0].Inputs[0].Type)
	assert.Equal(t, "otelcol_logs_pkg", m.PolicyTemplates[0].Inputs[0].Name)
	assert.Empty(t, m.PolicyTemplates[0].Inputs[0].Package)
	assert.Equal(t, "otelcol", m.PolicyTemplates[0].Inputs[1].Type)
	assert.Equal(t, "otelcol_metrics_pkg", m.PolicyTemplates[0].Inputs[1].Name)
	assert.Empty(t, m.PolicyTemplates[0].Inputs[1].Package)

	logsManifestBytes, err := os.ReadFile(filepath.Join(logsDir, "manifest.yml"))
	require.NoError(t, err)
	logsManifest, err := packages.ReadDataStreamManifestBytes(logsManifestBytes)
	require.NoError(t, err)
	require.Len(t, logsManifest.Streams, 1)
	assert.Equal(t, "otelcol_logs_pkg", logsManifest.Streams[0].Input)
	assert.Empty(t, logsManifest.Streams[0].Package)

	metricsManifestBytes, err := os.ReadFile(filepath.Join(metricsDir, "manifest.yml"))
	require.NoError(t, err)
	metricsManifest, err := packages.ReadDataStreamManifestBytes(metricsManifestBytes)
	require.NoError(t, err)
	require.Len(t, metricsManifest.Streams, 1)
	assert.Equal(t, "otelcol_metrics_pkg", metricsManifest.Streams[0].Input)
	assert.Empty(t, metricsManifest.Streams[0].Package)
}

// TestResolveStreamInputTypes_UniqueTypesNoName verifies that when all required input packages
// have distinct types, no name field is set on any input and stream.input uses the type identifier.
func TestResolveStreamInputTypes_UniqueTypesNoName(t *testing.T) {
	pkgADir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(pkgADir, "manifest.yml"), []byte(`
name: logs_pkg
title: Logs Pkg
description: Logs input.
version: 0.1.0
type: input
policy_templates:
  - name: logs
    input: logfile
    type: logs
`), 0644))

	pkgBDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(pkgBDir, "manifest.yml"), []byte(`
name: winlog_pkg
title: Winlog Pkg
description: Windows event logs input.
version: 0.1.0
type: input
policy_templates:
  - name: winlog
    input: winlog
    type: logs
`), 0644))

	buildRoot := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(buildRoot, "manifest.yml"), []byte(`
format_version: 3.6.0
name: my_integration
version: 0.1.0
type: integration
requires:
  input:
    - package: logs_pkg
      version: 0.1.0
    - package: winlog_pkg
      version: 0.1.0
policy_templates:
  - name: combined
    title: Combined
    description: Collect via two distinct input types
    data_streams:
      - combined_logs
    inputs:
      - package: logs_pkg
        title: Log files
      - package: winlog_pkg
        title: Windows Event Logs
`), 0644))

	dsDir := filepath.Join(buildRoot, "data_stream", "combined_logs")
	require.NoError(t, os.MkdirAll(dsDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dsDir, "manifest.yml"), []byte(`
title: Combined Logs
type: logs
streams:
  - package: logs_pkg
    title: Log files stream
  - package: winlog_pkg
    title: Windows event logs stream
`), 0644))

	downloadCalls := map[string]string{
		"logs_pkg":   pkgADir,
		"winlog_pkg": pkgBDir,
	}
	epr := &fakeEprClient{
		downloadPackageFunc: func(packageName, packageVersion, tmpDir string) (string, error) {
			return downloadCalls[packageName], nil
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
	assert.Empty(t, m.PolicyTemplates[0].Inputs[0].Name, "no name when types are unique")
	assert.Empty(t, m.PolicyTemplates[0].Inputs[0].Package)
	assert.Equal(t, "winlog", m.PolicyTemplates[0].Inputs[1].Type)
	assert.Empty(t, m.PolicyTemplates[0].Inputs[1].Name, "no name when types are unique")
	assert.Empty(t, m.PolicyTemplates[0].Inputs[1].Package)

	dsManifestBytes, err := os.ReadFile(filepath.Join(dsDir, "manifest.yml"))
	require.NoError(t, err)
	dsManifest, err := packages.ReadDataStreamManifestBytes(dsManifestBytes)
	require.NoError(t, err)
	require.Len(t, dsManifest.Streams, 2)
	assert.Equal(t, "logfile", dsManifest.Streams[0].Input)
	assert.Equal(t, "winlog", dsManifest.Streams[1].Input)
}

// TestResolveStreamInputTypes_FieldBundlingFixture runs the full
// Bundle pipeline on the composable CI integration fixture and
// verifies that package: references are replaced in both the main manifest and
// the data stream manifest.
func TestResolveStreamInputTypes_FieldBundlingFixture(t *testing.T) {
	buildPackageRoot := copyComposableIntegrationFixture(t)
	resolver := NewRequiredInputsResolver(makeFakeEprForFieldBundling(t))
	require.NoError(t, resolver.Bundle(buildPackageRoot))

	// Check main manifest: package-backed input → type: logfile; native logs input unchanged.
	manifestBytes, err := os.ReadFile(filepath.Join(buildPackageRoot, "manifest.yml"))
	require.NoError(t, err)
	m, err := packages.ReadPackageManifestBytes(manifestBytes)
	require.NoError(t, err)
	require.Len(t, m.PolicyTemplates[0].Inputs, 2)
	assert.Equal(t, "logfile", m.PolicyTemplates[0].Inputs[0].Type)
	assert.Empty(t, m.PolicyTemplates[0].Inputs[0].Package)
	assert.Equal(t, "logs", m.PolicyTemplates[0].Inputs[1].Type)
	assert.Empty(t, m.PolicyTemplates[0].Inputs[1].Package)

	// Check data stream manifest: package stream → input: logfile; native stream stays logfile.
	dsManifestBytes, err := os.ReadFile(filepath.Join(buildPackageRoot, "data_stream", "ci_composable_logs", "manifest.yml"))
	require.NoError(t, err)
	dsManifest, err := packages.ReadDataStreamManifestBytes(dsManifestBytes)
	require.NoError(t, err)
	require.Len(t, dsManifest.Streams, 2)
	assert.Equal(t, "logfile", dsManifest.Streams[0].Input)
	assert.Empty(t, dsManifest.Streams[0].Package)
	assert.NotEmpty(t, dsManifest.Streams[0].Title)
	assert.Equal(t, "logfile", dsManifest.Streams[1].Input)
	assert.Empty(t, dsManifest.Streams[1].Package)
}
