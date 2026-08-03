// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/elastic-package/internal/llmagent/tracing"
	"github.com/elastic/elastic-package/internal/profile"
)

func TestTracingConfigDefaults(t *testing.T) {
	cfg, err := TracingConfig(nil)
	require.NoError(t, err)

	assert.False(t, cfg.Enabled)
	assert.Equal(t, tracing.DefaultEndpoint, cfg.Endpoint)
	assert.Equal(t, tracing.DefaultProjectName, cfg.ProjectName)
	assert.Empty(t, cfg.APIKey)
	assert.Empty(t, cfg.Headers)
}

func TestTracingConfig(t *testing.T) {
	profilesDir := t.TempDir()
	const profileName = "tracing-test"

	err := profile.CreateProfile(profile.Options{
		ProfilesDirPath: profilesDir,
		Name:            profileName,
	})
	require.NoError(t, err)

	configPath := filepath.Join(profilesDir, profileName, profile.PackageProfileConfigFile)
	err = os.WriteFile(configPath, []byte(`
llm.tracing.enabled: true
llm.tracing.endpoint: "https://collector.example.test/v1/traces"
llm.tracing.api_key: "secret"
llm.tracing.project_name: "documentation-agent"
llm.tracing.headers:
  Authorization: "Bearer token"
  x-tenant-id: "tenant"
`), 0o600)
	require.NoError(t, err)

	p, err := profile.LoadProfileFrom(profilesDir, profileName)
	require.NoError(t, err)

	cfg, err := TracingConfig(p)
	require.NoError(t, err)
	assert.True(t, cfg.Enabled)
	assert.Equal(t, "https://collector.example.test/v1/traces", cfg.Endpoint)
	assert.Equal(t, "secret", cfg.APIKey)
	assert.Equal(t, "documentation-agent", cfg.ProjectName)
	assert.Equal(t, map[string]string{
		"Authorization": "Bearer token",
		"x-tenant-id":   "tenant",
	}, cfg.Headers)
}
