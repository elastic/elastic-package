// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package install

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSelectElasticAgentImageName_NoVersion(t *testing.T) {
	var version string
	selected := selectElasticAgentImageName(version, "")
	assert.Equal(t, selected, elasticAgentLegacyImageName)
}

func TestSelectElasticAgentImageName_OlderStack(t *testing.T) {
	version := "7.14.99-SNAPSHOT"
	selected := selectElasticAgentImageName(version, "")
	assert.Equal(t, selected, elasticAgentLegacyImageName)
}

func TestSelectElasticAgentImageName_FirstStackWithCompleteAgent(t *testing.T) {
	version := stackVersion715
	selected := selectElasticAgentImageName(version, "")
	assert.Equal(t, selected, elasticAgentCompleteLegacyImageName)
}

func TestSelectElasticAgentImageName_NextStackWithAgentComplete(t *testing.T) {
	version := "7.16.0-SNAPSHOT"
	selected := selectElasticAgentImageName(version, "")
	assert.Equal(t, selected, elasticAgentCompleteLegacyImageName)
}

func TestSelectElasticAgentImageName_OwnNamespace(t *testing.T) {
	version := "8.2.0-SNAPSHOT"
	selected := selectElasticAgentImageName(version, "")
	assert.Equal(t, selected, elasticAgentCompleteImageName)
}

func TestSelectElasticAgentImageName_OwnNamespace_Release(t *testing.T) {
	version := "8.2.0"
	selected := selectElasticAgentImageName(version, "")
	assert.Equal(t, selected, elasticAgentCompleteImageName)
}

func TestSelectElasticAgentImageName_NextStackInOwnNamespace(t *testing.T) {
	version := "8.4.0-SNAPSHOT"
	selected := selectElasticAgentImageName(version, "")
	assert.Equal(t, selected, elasticAgentCompleteImageName)
}

func TestSelectElasticAgentImageName_DefaultImage816_WithoutEnvVar(t *testing.T) {
	version := stackVersion8160
	// Try to keep the test agnostic from the environment variables defined in CI
	t.Setenv(disableElasticAgentWolfiEnvVar, "")
	os.Unsetenv(disableElasticAgentWolfiEnvVar)

	selected := selectElasticAgentImageName(version, "")
	assert.Equal(t, selected, elasticAgentWolfiImageName)
}

func TestSelectElasticAgentImageName_DisableWolfiImageEnvVar(t *testing.T) {
	version := stackVersion8160
	t.Setenv(disableElasticAgentWolfiEnvVar, "true")
	selected := selectElasticAgentImageName(version, "")
	assert.Equal(t, selected, elasticAgentCompleteImageName)
}
func TestSelectElasticAgentImageName_EnableWolfiImageEnvVar(t *testing.T) {
	version := stackVersion8160
	t.Setenv(disableElasticAgentWolfiEnvVar, "false")
	selected := selectElasticAgentImageName(version, "")
	assert.Equal(t, selected, elasticAgentWolfiImageName)
}

func TestSelectCompleteElasticAgentImageName_ForceCompleteImage(t *testing.T) {
	version := stackVersion8160
	selected := selectElasticAgentImageName(version, "complete")
	assert.Equal(t, selected, elasticAgentCompleteImageName)
}

func TestSelectCompleteElasticAgentImageName_ForceDefaultImage_DisabledEnvVar(t *testing.T) {
	version := stackVersion8160
	t.Setenv(disableElasticAgentWolfiEnvVar, "true")
	selected := selectElasticAgentImageName(version, "default")
	assert.Equal(t, selected, elasticAgentCompleteImageName)
}

func TestSelectCompleteElasticAgentImageName_ForceDefaultImage_EnabledEnvVar(t *testing.T) {
	version := stackVersion8160
	t.Setenv(disableElasticAgentWolfiEnvVar, "false")
	selected := selectElasticAgentImageName(version, "default")
	assert.Equal(t, selected, elasticAgentWolfiImageName)
}

func TestSelectCompleteElasticAgentImageName_ForceDefaultImageOldStack(t *testing.T) {
	version := "8.15.0-SNAPSHOT"
	selected := selectElasticAgentImageName(version, "default")
	assert.Equal(t, selected, elasticAgentCompleteImageName)
}

func TestSelectCompleteElasticAgentImageName_ForceSystemDImage(t *testing.T) {
	version := stackVersion8160
	selected := selectElasticAgentImageName(version, "systemd")
	assert.Equal(t, selected, elasticAgentImageName)
}

func TestSelectCompleteElasticAgentImageName_ForceSystemDImageOldStack(t *testing.T) {
	version := stackVersion715
	selected := selectElasticAgentImageName(version, "systemd")
	assert.Equal(t, selected, elasticAgentLegacyImageName)
}
