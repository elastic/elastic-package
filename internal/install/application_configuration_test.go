// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package install

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/elastic-package/internal/configuration/locations"
)

func TestSelectElasticAgentImageName_NoVersion(t *testing.T) {
	var version string
	selected := selectElasticAgentImageName(version, "")
	assert.Equal(t, elasticAgentWolfiImageName, selected)
}

func TestSelectElasticAgentImageName_OlderStack(t *testing.T) {
	version := "7.14.99-SNAPSHOT"
	selected := selectElasticAgentImageName(version, "")
	assert.Equal(t, elasticAgentLegacyImageName, selected)
}

func TestSelectElasticAgentImageName_FirstStackWithCompleteAgent(t *testing.T) {
	version := stackVersion715
	selected := selectElasticAgentImageName(version, "")
	assert.Equal(t, elasticAgentCompleteLegacyImageName, selected)
}

func TestSelectElasticAgentImageName_NextStackWithAgentComplete(t *testing.T) {
	version := "7.16.0-SNAPSHOT"
	selected := selectElasticAgentImageName(version, "")
	assert.Equal(t, elasticAgentCompleteLegacyImageName, selected)
}

func TestSelectElasticAgentImageName_OwnNamespace(t *testing.T) {
	version := "8.2.0-SNAPSHOT"
	selected := selectElasticAgentImageName(version, "")
	assert.Equal(t, elasticAgentCompleteImageName, selected)
}

func TestSelectElasticAgentImageName_OwnNamespace_Release(t *testing.T) {
	version := "8.2.0"
	selected := selectElasticAgentImageName(version, "")
	assert.Equal(t, elasticAgentCompleteImageName, selected)
}

func TestSelectElasticAgentImageName_NextStackInOwnNamespace(t *testing.T) {
	version := "8.4.0-SNAPSHOT"
	selected := selectElasticAgentImageName(version, "")
	assert.Equal(t, elasticAgentCompleteImageName, selected)
}

func TestSelectElasticAgentImageName_DefaultImage816_WithoutEnvVar(t *testing.T) {
	version := stackVersion8160
	// Try to keep the test agnostic from the environment variables defined in CI
	t.Setenv(disableElasticAgentWolfiEnvVar, "")
	os.Unsetenv(disableElasticAgentWolfiEnvVar)

	selected := selectElasticAgentImageName(version, "")
	assert.Equal(t, elasticAgentWolfiImageName, selected)
}

func TestSelectElasticAgentImageName_DisableWolfiImageEnvVar(t *testing.T) {
	version := stackVersion8160
	t.Setenv(disableElasticAgentWolfiEnvVar, "true")
	selected := selectElasticAgentImageName(version, "")
	assert.Equal(t, elasticAgentCompleteImageName, selected)
}
func TestSelectElasticAgentImageName_EnableWolfiImageEnvVar(t *testing.T) {
	version := stackVersion8160
	t.Setenv(disableElasticAgentWolfiEnvVar, "false")
	selected := selectElasticAgentImageName(version, "")
	assert.Equal(t, elasticAgentWolfiImageName, selected)
}

func TestSelectCompleteElasticAgentImageName_ForceCompleteImage_NonWolfi(t *testing.T) {
	version := "8.15.0"
	selected := selectElasticAgentImageName(version, "complete")
	assert.Equal(t, elasticAgentCompleteImageName, selected)
}

func TestSelectCompleteElasticAgentImageName_ForceCompleteImage_Wolfi(t *testing.T) {
	version := stackVersion8160
	selected := selectElasticAgentImageName(version, "complete")
	assert.Equal(t, elasticAgentCompleteWolfiImageName, selected)
}

func TestSelectCompleteElasticAgentImageName_ForceDefaultImage_DisabledEnvVar(t *testing.T) {
	version := stackVersion8160
	t.Setenv(disableElasticAgentWolfiEnvVar, "true")
	selected := selectElasticAgentImageName(version, "default")
	assert.Equal(t, elasticAgentCompleteImageName, selected)
}

func TestSelectCompleteElasticAgentImageName_ForceDefaultImage_EnabledEnvVar(t *testing.T) {
	version := stackVersion8160
	t.Setenv(disableElasticAgentWolfiEnvVar, "false")
	selected := selectElasticAgentImageName(version, "default")
	assert.Equal(t, elasticAgentWolfiImageName, selected)
}

func TestSelectCompleteElasticAgentImageName_ForceDefaultImageOldStack(t *testing.T) {
	version := "8.15.0-SNAPSHOT"
	selected := selectElasticAgentImageName(version, "default")
	assert.Equal(t, elasticAgentCompleteImageName, selected)
}

func TestSelectCompleteElasticAgentImageName_ForceSystemDImage(t *testing.T) {
	version := stackVersion8160
	selected := selectElasticAgentImageName(version, "systemd")
	assert.Equal(t, elasticAgentImageName, selected)
}

func TestSelectCompleteElasticAgentImageName_ForceSystemDImageOldStack(t *testing.T) {
	version := stackVersion715
	selected := selectElasticAgentImageName(version, "systemd")
	assert.Equal(t, elasticAgentLegacyImageName, selected)
}

func TestExistingApplicationConfigurationECSSchema(t *testing.T) {
	cases := []struct {
		name              string
		create            bool
		settingData       string
		expectedECSSchema string
	}{
		{
			name:   "custom ECS schema",
			create: true,
			settingData: `
schema_urls:
  ecs_base: "https://custom-ecs.example"
`,
			expectedECSSchema: "https://custom-ecs.example",
		},
		{
			name:   "no customizations",
			create: true,
			settingData: `
profiles:
  current: default
`,
			expectedECSSchema: "https://raw.githubusercontent.com/elastic/ecs",
		},
		{
			name:              "config file not created",
			create:            false,
			expectedECSSchema: "https://raw.githubusercontent.com/elastic/ecs",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			t.Setenv("ELASTIC_PACKAGE_DATA_HOME", tmpDir)
			configPath, err := locations.NewLocationManager()
			require.NoError(t, err)

			if tc.create {
				configFilePath := filepath.Join(configPath.RootDir(), applicationConfigurationYmlFile)

				err = os.WriteFile(configFilePath, []byte(tc.settingData), 0644)
				require.NoError(t, err)
			}

			config, err := Configuration()
			require.NoError(t, err)
			assert.Equal(t, tc.expectedECSSchema, config.SchemaURLs().ECSBase())
		})
	}
}
