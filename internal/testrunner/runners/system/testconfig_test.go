// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package system

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/elastic-package/internal/servicedeployer"
)

func TestNewConfig(t *testing.T) {
	t.Run("minimal config loads successfully", func(t *testing.T) {
		dir := t.TempDir()
		configPath := filepath.Join(dir, "test-some-config.yml")
		err := os.WriteFile(configPath, []byte(`
input: log
service: nginx
`), 0644)
		require.NoError(t, err)

		cfg, err := newConfig(configPath, servicedeployer.ServiceInfo{}, "")
		require.NoError(t, err)
		require.NotNil(t, cfg)
		assert.Equal(t, "log", cfg.Input)
		assert.Equal(t, "nginx", cfg.Service)
		assert.Empty(t, cfg.Vars)
	})

	t.Run("vars with data_stream.dataset are detected", func(t *testing.T) {
		dir := t.TempDir()
		configPath := filepath.Join(dir, "test-dataset-config.yml")
		err := os.WriteFile(configPath, []byte(`
input: log
service: nginx
vars:
  data_stream.dataset: other.name
`), 0644)
		require.NoError(t, err)

		cfg, err := newConfig(configPath, servicedeployer.ServiceInfo{}, "")
		require.NoError(t, err)
		require.NotNil(t, cfg)

		v, err := cfg.Vars.GetValue("data_stream.dataset")
		require.NoError(t, err)
		ds, ok := v.(string)
		require.True(t, ok, "data_stream.dataset should be a string")
		assert.Equal(t, "other.name", ds)
	})

	t.Run("vars with data_stream.dataset and other vars", func(t *testing.T) {
		dir := t.TempDir()
		configPath := filepath.Join(dir, "test-multi-vars-config.yml")
		err := os.WriteFile(configPath, []byte(`
input: log
service: nginx
vars:
  data_stream.dataset: other.name
  some.other.var: value
`), 0644)
		require.NoError(t, err)

		cfg, err := newConfig(configPath, servicedeployer.ServiceInfo{}, "")
		require.NoError(t, err)
		require.NotNil(t, cfg)

		v, err := cfg.Vars.GetValue("data_stream.dataset")
		require.NoError(t, err)
		ds, ok := v.(string)
		require.True(t, ok)
		assert.Equal(t, "other.name", ds)

		v2, err := cfg.Vars.GetValue("some.other.var")
		require.NoError(t, err)
		val, ok := v2.(string)
		require.True(t, ok)
		assert.Equal(t, "value", val)
	})

	t.Run("data_stream.vars are detected", func(t *testing.T) {
		dir := t.TempDir()
		configPath := filepath.Join(dir, "test-datastream-vars-config.yml")
		err := os.WriteFile(configPath, []byte(`
input: log
service: nginx
data_stream:
  vars:
    dataset: my.dataset
`), 0644)
		require.NoError(t, err)

		cfg, err := newConfig(configPath, servicedeployer.ServiceInfo{}, "")
		require.NoError(t, err)
		require.NotNil(t, cfg)

		v, err := cfg.DataStream.Vars.GetValue("dataset")
		require.NoError(t, err)
		ds, ok := v.(string)
		require.True(t, ok)
		assert.Equal(t, "my.dataset", ds)
	})

	t.Run("missing config file returns error", func(t *testing.T) {
		dir := t.TempDir()
		configPath := filepath.Join(dir, "test-nonexistent-config.yml")

		cfg, err := newConfig(configPath, servicedeployer.ServiceInfo{}, "")
		assert.Error(t, err)
		assert.Nil(t, cfg)
		assert.Contains(t, err.Error(), "unable to find system test configuration file")
	})
}

func TestNewConfig_ConfigName(t *testing.T) {
	t.Run("name is derived from config file", func(t *testing.T) {
		dir := t.TempDir()
		configPath := filepath.Join(dir, "test-my-scenario-config.yml")
		err := os.WriteFile(configPath, []byte(`
input: log
service: nginx
`), 0644)
		require.NoError(t, err)

		cfg, err := newConfig(configPath, servicedeployer.ServiceInfo{}, "")
		require.NoError(t, err)
		assert.Equal(t, "my-scenario", cfg.Name())
	})

	t.Run("name includes variant when set", func(t *testing.T) {
		dir := t.TempDir()
		configPath := filepath.Join(dir, "test-my-scenario-config.yml")
		err := os.WriteFile(configPath, []byte(`
input: log
service: nginx
`), 0644)
		require.NoError(t, err)

		cfg, err := newConfig(configPath, servicedeployer.ServiceInfo{}, "variant-a")
		require.NoError(t, err)
		assert.Equal(t, "my-scenario (variant: variant-a)", cfg.Name())
	})
}

// Ensure that vars with data_stream.dataset are correctly parsed in the config.
func TestNewConfig_DataStreamDatasetVar(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "test-dataset-config.yml")
	err := os.WriteFile(configPath, []byte(`
input: log
service: nginx
vars:
  data_stream.dataset: other.name
`), 0644)
	require.NoError(t, err)

	cfg, err := newConfig(configPath, servicedeployer.ServiceInfo{}, "")
	require.NoError(t, err)
	require.NotNil(t, cfg)

	v, _ := cfg.Vars.GetValue("data_stream.dataset")
	assert.Equal(t, "other.name", v, "vars.data_stream.dataset should be available in config vars")
}
