// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package stack

import (
	"bytes"
	"context"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/elastic/go-resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/elastic-package/internal/profile"
)

// testFacterData contains default test values for resource facters.
var testFacterData = resource.StaticFacter{
	"kibana_version":        "8.11.0",
	"elasticsearch_version": "8.11.0",
	"agent_version":         "8.11.0",
	"username":              "elastic",
	"password":              "changeme",
	"kibana_host":           "https://kibana:5601",
	"elasticsearch_host":    "https://elasticsearch:9200",
	"fleet_url":             "https://fleet-server:8220",
	"apm_enabled":           "false",
	"logstash_enabled":      "false",
	"self_monitor_enabled":  "false",
	"kibana_http2_enabled":  "true",
	"logsdb_enabled":        "false",
	"elastic_subscription":  "basic",
	"geoip_dir":             "./ingest-geoip",
	"agent_publish_ports":   "6791",
	"api_key":               "",
	"enrollment_token":      "",
}

// Helper function to create a test profile
func createTestProfile(t *testing.T, profileName string) *profile.Profile {
	tempDir := t.TempDir()
	profilesPath := filepath.Join(tempDir, "profiles")

	// Set environment variable to use our temporary directory
	originalEnv := os.Getenv("ELASTIC_PACKAGE_DATA_HOME")
	os.Setenv("ELASTIC_PACKAGE_DATA_HOME", tempDir)
	t.Cleanup(func() {
		os.Setenv("ELASTIC_PACKAGE_DATA_HOME", originalEnv)
	})

	err := profile.CreateProfile(profile.Options{
		ProfilesDirPath: profilesPath,
		Name:            profileName,
	})
	require.NoError(t, err)

	p, err := profile.LoadProfile(profileName)
	require.NoError(t, err)
	return p
}

// createTestResourceContext creates a resource context with default test values and the given profile registered.
func createTestResourceContext(p *profile.Profile) resource.Context {
	resourceManager := resource.NewManager()
	resourceManager.AddFacter(testFacterData)
	resourceManager.RegisterProvider("profile", p)
	return resourceManager.Context(context.Background())
}

func TestKibanaConfigWithCustomContent_NoCustomConfig(t *testing.T) {
	p := createTestProfile(t, "test-profile")

	var logBuffer bytes.Buffer
	log.SetOutput(&logBuffer)
	defer log.SetOutput(os.Stderr)

	ctx := createTestResourceContext(p)

	var output bytes.Buffer
	err := kibanaCustomContent()(ctx, &output)
	require.NoError(t, err)

	logOutput := logBuffer.String()
	assert.NotContains(t, logOutput, "Custom Kibana configuration detected")
}

func TestKibanaConfigWithCustomContent_WithCustomConfig(t *testing.T) {
	p := createTestProfile(t, "test-profile")

	customConfigPath := p.Path(KibanaDevConfigFile)
	customConfigContent := "logging.loggers:\n  - name: root\n    level: debug\n"
	err := os.WriteFile(customConfigPath, []byte(customConfigContent), 0644)
	require.NoError(t, err)
	defer os.Remove(customConfigPath)

	var logBuffer bytes.Buffer
	log.SetOutput(&logBuffer)
	defer log.SetOutput(os.Stderr)

	ctx := createTestResourceContext(p)

	var output bytes.Buffer
	err = kibanaCustomContent()(ctx, &output)
	require.NoError(t, err)

	generatedContent := output.String()
	assert.Contains(t, generatedContent, "# Custom Kibana Configuration")
	assert.Contains(t, generatedContent, "logging.loggers:")
	assert.Contains(t, generatedContent, "- name: root")
	assert.Contains(t, generatedContent, "level: debug")

	logOutput := logBuffer.String()
	assert.Contains(t, logOutput, "Custom Kibana configuration detected")
	assert.Contains(t, logOutput, KibanaDevConfigFile)
	assert.Contains(t, logOutput, "this may affect Kibana behavior")
}

func TestKibanaConfigWithCustomContent_FileNaming(t *testing.T) {
	p := createTestProfile(t, "test-profile")

	customConfigPath := p.Path(KibanaDevConfigFile)
	customConfigContent := "logging.loggers:\n  - name: test\n    level: debug\n"
	err := os.WriteFile(customConfigPath, []byte(customConfigContent), 0644)
	require.NoError(t, err)
	defer os.Remove(customConfigPath)

	var logBuffer bytes.Buffer
	log.SetOutput(&logBuffer)
	defer log.SetOutput(os.Stderr)

	ctx := createTestResourceContext(p)

	var output bytes.Buffer
	err = kibanaCustomContent()(ctx, &output)
	require.NoError(t, err)

	logOutput := logBuffer.String()
	assert.Contains(t, logOutput, KibanaDevConfigFile)
	assert.Contains(t, logOutput, "kibana.dev.yml")
}

func TestKibanaConfigWithCustomContent_NoTemplateProcessing(t *testing.T) {
	p := createTestProfile(t, "test-profile")

	customConfigPath := p.Path(KibanaDevConfigFile)
	customConfigContent := "server.name: kibana-{{ fact \"kibana_version\" }}\nlogging.level: {{ if eq .debug \"true\" }}debug{{ else }}info{{ end }}\n"
	err := os.WriteFile(customConfigPath, []byte(customConfigContent), 0644)
	require.NoError(t, err)
	defer os.Remove(customConfigPath)

	ctx := createTestResourceContext(p)

	var output bytes.Buffer
	err = kibanaCustomContent()(ctx, &output)
	require.NoError(t, err)

	generatedContent := output.String()
	assert.Contains(t, generatedContent, "server.name: kibana-{{ fact \"kibana_version\" }}")
	assert.Contains(t, generatedContent, "logging.level: {{ if eq .debug \"true\" }}debug{{ else }}info{{ end }}")
}

func TestKibanaConfigWithCustomContent_ErrorCases(t *testing.T) {
	p := createTestProfile(t, "test-profile")

	customConfigPath := p.Path(KibanaDevConfigFile)
	err := os.MkdirAll(customConfigPath, 0755)
	require.NoError(t, err)
	defer os.RemoveAll(customConfigPath)

	ctx := createTestResourceContext(p)

	var output bytes.Buffer
	err = kibanaCustomContent()(ctx, &output)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read custom kibana config")
}

func TestKibanaDevConfigFileConstant(t *testing.T) {
	assert.Equal(t, "kibana.dev.yml", KibanaDevConfigFile)
}

// Benchmark test to ensure performance is acceptable
func BenchmarkKibanaConfigWithCustomContent(b *testing.B) {
	tempDir := b.TempDir()
	profilesPath := filepath.Join(tempDir, "profiles")
	profileName := "benchmark-profile"

	originalEnv := os.Getenv("ELASTIC_PACKAGE_DATA_HOME")
	os.Setenv("ELASTIC_PACKAGE_DATA_HOME", tempDir)
	defer os.Setenv("ELASTIC_PACKAGE_DATA_HOME", originalEnv)

	err := profile.CreateProfile(profile.Options{
		ProfilesDirPath: profilesPath,
		Name:            profileName,
	})
	require.NoError(b, err)

	p, err := profile.LoadProfile(profileName)
	require.NoError(b, err)

	customConfigPath := p.Path(KibanaDevConfigFile)
	customConfigContent := strings.Repeat("logging.loggers:\n  - name: test\n    level: debug\n", 10)
	err = os.WriteFile(customConfigPath, []byte(customConfigContent), 0644)
	require.NoError(b, err)
	defer os.Remove(customConfigPath)

	resourceManager := resource.NewManager()
	resourceManager.AddFacter(testFacterData)
	resourceManager.RegisterProvider("profile", p)
	ctx := resourceManager.Context(context.Background())

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var output bytes.Buffer
		err := kibanaCustomContent()(ctx, &output)
		if err != nil {
			b.Fatal(err)
		}
	}
}
