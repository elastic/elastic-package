// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package testrunner

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadGlobalTestConfig_RequiresSource(t *testing.T) {
	packageRoot := t.TempDir()
	inputPkg := filepath.Join(packageRoot, "my_input_pkg")
	require.NoError(t, os.MkdirAll(inputPkg, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(inputPkg, "manifest.yml"), []byte(`format_version: 3.6.0
name: my_input_pkg
title: My Input
version: 0.1.0
type: input
`), 0644))

	devTestDir := filepath.Join(packageRoot, "_dev", "test")
	require.NoError(t, os.MkdirAll(devTestDir, 0755))

	configYAML := []byte(`
system:
  parallel: true
  requires:
    - source: "my_input_pkg"
policy:
  requires:
    - package: other_input
      version: "1.0.0"
`)
	require.NoError(t, os.WriteFile(filepath.Join(devTestDir, "config.yml"), configYAML, 0644))

	cfg, err := ReadGlobalTestConfig(packageRoot)
	require.NoError(t, err)

	assert.True(t, cfg.System.Parallel)
	require.Len(t, cfg.System.Requires, 1)
	assert.Equal(t, "my_input_pkg", cfg.System.Requires[0].Source)
	require.Len(t, cfg.Policy.Requires, 1)
	assert.Equal(t, "other_input", cfg.Policy.Requires[0].Package)
	assert.Equal(t, "1.0.0", cfg.Policy.Requires[0].Version)

	overrides, err := cfg.System.RequiresSourceOverrides(packageRoot)
	require.NoError(t, err)
	require.Len(t, overrides, 1)
	assert.Equal(t, filepath.Join(packageRoot, "my_input_pkg"), overrides["my_input_pkg"])
}

func TestReadGlobalTestConfig_InvalidRequiresPackageAndSource(t *testing.T) {
	packageRoot := t.TempDir()
	devTestDir := filepath.Join(packageRoot, "_dev", "test")
	require.NoError(t, os.MkdirAll(devTestDir, 0755))

	configYAML := []byte(`
system:
  requires:
    - package: my_input_pkg
      source: "../x"
`)
	require.NoError(t, os.WriteFile(filepath.Join(devTestDir, "config.yml"), configYAML, 0644))

	_, err := ReadGlobalTestConfig(packageRoot)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid requires")
}

func TestRequiresSourceOverrides_RelativePaths(t *testing.T) {
	packageRoot := t.TempDir()
	inputDir := filepath.Join(packageRoot, "nested", "my_input_pkg")
	require.NoError(t, os.MkdirAll(inputDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(inputDir, "manifest.yml"), []byte(`format_version: 3.6.0
name: my_input_pkg
title: My Input
version: 0.1.0
type: input
`), 0644))

	cfg := GlobalRunnerTestConfig{
		Requires: []PackageTestRequirement{
			{Source: "nested/my_input_pkg"},
			{Source: "nested/my_input_pkg/../my_input_pkg"},
		},
	}

	overrides, err := cfg.RequiresSourceOverrides(packageRoot)
	require.NoError(t, err)
	require.Len(t, overrides, 1)
	assert.Equal(t, filepath.Join(packageRoot, "nested", "my_input_pkg"), overrides["my_input_pkg"])
}

func TestRequiresSourceOverrides_AbsolutePath(t *testing.T) {
	packageRoot := t.TempDir()
	absPkg := filepath.Join(packageRoot, "abs_pkg")
	require.NoError(t, os.MkdirAll(absPkg, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(absPkg, "manifest.yml"), []byte(`format_version: 3.6.0
name: abs_pkg
title: Abs
version: 0.1.0
type: input
`), 0644))

	cfg := GlobalRunnerTestConfig{
		Requires: []PackageTestRequirement{
			{Source: absPkg},
		},
	}

	overrides, err := cfg.RequiresSourceOverrides(packageRoot)
	require.NoError(t, err)
	require.Len(t, overrides, 1)
	assert.Equal(t, absPkg, overrides["abs_pkg"])
}

func TestRequiresSourceOverrides_Empty(t *testing.T) {
	cfg := GlobalRunnerTestConfig{}
	overrides, err := cfg.RequiresSourceOverrides("/some/root")
	require.NoError(t, err)
	assert.Nil(t, overrides)
}

func TestMergedRequiresSourceOverrides_consistent(t *testing.T) {
	packageRoot := t.TempDir()
	inputDir := filepath.Join(packageRoot, "ci_input_pkg")
	require.NoError(t, os.MkdirAll(inputDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(inputDir, "manifest.yml"), []byte(`format_version: 3.6.0
name: ci_input_pkg
title: CI Input
version: 0.1.0
type: input
`), 0644))

	cfg := &globalTestConfig{
		System: GlobalRunnerTestConfig{
			Requires: []PackageTestRequirement{{Source: "ci_input_pkg"}},
		},
		Policy: GlobalRunnerTestConfig{
			Requires: []PackageTestRequirement{{Source: "./ci_input_pkg"}},
		},
	}

	merged, err := cfg.MergedRequiresSourceOverrides(packageRoot)
	require.NoError(t, err)
	require.Len(t, merged, 1)
	assert.Equal(t, inputDir, merged["ci_input_pkg"])
}

func TestMergedRequiresSourceOverrides_conflict(t *testing.T) {
	packageRoot := t.TempDir()
	dirA := filepath.Join(packageRoot, "a")
	dirB := filepath.Join(packageRoot, "b")
	for _, d := range []string{dirA, dirB} {
		require.NoError(t, os.MkdirAll(d, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(d, "manifest.yml"), []byte(`format_version: 3.6.0
name: same_name
title: X
version: 0.1.0
type: input
`), 0644))
	}

	cfg := &globalTestConfig{
		System: GlobalRunnerTestConfig{
			Requires: []PackageTestRequirement{{Source: "a"}},
		},
		Policy: GlobalRunnerTestConfig{
			Requires: []PackageTestRequirement{{Source: "b"}},
		},
	}

	_, err := cfg.MergedRequiresSourceOverrides(packageRoot)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "conflicting requires source")
}
