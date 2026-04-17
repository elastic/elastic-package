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

func TestReadGlobalTestConfig_Requires(t *testing.T) {
	packageRoot := t.TempDir()
	devTestDir := filepath.Join(packageRoot, "_dev", "test")
	require.NoError(t, os.MkdirAll(devTestDir, 0755))

	configYAML := []byte(`
system:
  parallel: true
requires:
  - package: my_input_pkg
    source: "../my_input_pkg"
  - package: other_pkg
    source: "/absolute/path/to/other_pkg"
`)
	require.NoError(t, os.WriteFile(filepath.Join(devTestDir, "config.yml"), configYAML, 0644))

	cfg, err := ReadGlobalTestConfig(packageRoot)
	require.NoError(t, err)

	assert.True(t, cfg.System.Parallel)
	assert.Len(t, cfg.Requires, 2)
	assert.Equal(t, "my_input_pkg", cfg.Requires[0].Package)
	assert.Equal(t, "../my_input_pkg", cfg.Requires[0].Source)
	assert.Equal(t, "other_pkg", cfg.Requires[1].Package)
	assert.Equal(t, "/absolute/path/to/other_pkg", cfg.Requires[1].Source)
}

func TestRequiresSourceOverrides_RelativePaths(t *testing.T) {
	packageRoot := "/some/package/root"

	cfg := &globalTestConfig{
		Requires: []RequiresTestOverride{
			{Package: "my_input_pkg", Source: "../my_input_pkg"},
			{Package: "other_pkg", Source: "local/other_pkg"},
		},
	}

	overrides := cfg.RequiresSourceOverrides(packageRoot)

	require.NotNil(t, overrides)
	assert.Equal(t, filepath.Join(packageRoot, "../my_input_pkg"), overrides["my_input_pkg"])
	assert.Equal(t, filepath.Join(packageRoot, "local/other_pkg"), overrides["other_pkg"])
}

func TestRequiresSourceOverrides_AbsolutePath(t *testing.T) {
	packageRoot := "/some/package/root"

	cfg := &globalTestConfig{
		Requires: []RequiresTestOverride{
			{Package: "abs_pkg", Source: "/absolute/path/to/abs_pkg"},
		},
	}

	overrides := cfg.RequiresSourceOverrides(packageRoot)

	require.NotNil(t, overrides)
	assert.Equal(t, "/absolute/path/to/abs_pkg", overrides["abs_pkg"])
}

func TestRequiresSourceOverrides_Empty(t *testing.T) {
	cfg := &globalTestConfig{}
	assert.Nil(t, cfg.RequiresSourceOverrides("/some/root"))
}
