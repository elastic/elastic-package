// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package mcptools

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMCPServer_Connect_NilURL(t *testing.T) {
	server := &MCPServer{}
	err := server.Connect()
	// With nil URL, it should return nil (skip) not error
	require.NoError(t, err)
	assert.Nil(t, server.Toolset)
}

func TestMCPServer_Close(t *testing.T) {
	server := &MCPServer{}

	// Should not error even if toolset is nil
	err := server.Close()
	require.NoError(t, err)
	assert.Nil(t, server.Toolset)
}

func TestLoadToolsets_NoConfigFile(t *testing.T) {
	// Create a temporary directory that doesn't have the config file
	tempDir := t.TempDir()
	originalEnv := os.Getenv("ELASTIC_PACKAGE_DATA_HOME")
	defer func() {
		if originalEnv == "" {
			os.Unsetenv("ELASTIC_PACKAGE_DATA_HOME")
		} else {
			os.Setenv("ELASTIC_PACKAGE_DATA_HOME", originalEnv)
		}
	}()

	// Set ELASTIC_PACKAGE_DATA_HOME to temp directory so LocationManager looks there
	os.Setenv("ELASTIC_PACKAGE_DATA_HOME", tempDir)

	result := LoadToolsets()
	assert.Nil(t, result, "Expected nil when config file doesn't exist")
}

func TestLoadToolsets_InvalidJSON(t *testing.T) {
	// Create a temporary config directory
	tempDir := t.TempDir()
	configDir := filepath.Join(tempDir, "llm_config")
	err := os.MkdirAll(configDir, 0o755)
	require.NoError(t, err)

	// Write invalid JSON
	mcpFile := filepath.Join(configDir, "mcp.json")
	err = os.WriteFile(mcpFile, []byte("invalid json {{{"), 0o644)
	require.NoError(t, err)

	originalEnv := os.Getenv("ELASTIC_PACKAGE_DATA_HOME")
	defer func() {
		if originalEnv == "" {
			os.Unsetenv("ELASTIC_PACKAGE_DATA_HOME")
		} else {
			os.Setenv("ELASTIC_PACKAGE_DATA_HOME", originalEnv)
		}
	}()
	os.Setenv("ELASTIC_PACKAGE_DATA_HOME", tempDir)

	result := LoadToolsets()
	assert.Nil(t, result, "Expected nil when JSON is invalid")
}

func TestLoadToolsets_ValidConfig_NoServers(t *testing.T) {
	// Create a temporary config directory
	tempDir := t.TempDir()
	configDir := filepath.Join(tempDir, "llm_config")
	err := os.MkdirAll(configDir, 0o755)
	require.NoError(t, err)

	// Create valid JSON with no servers
	config := MCPJson{
		Servers: map[string]MCPServer{},
	}
	data, err := json.Marshal(config)
	require.NoError(t, err)

	mcpFile := filepath.Join(configDir, "mcp.json")
	err = os.WriteFile(mcpFile, data, 0o644)
	require.NoError(t, err)

	originalEnv := os.Getenv("ELASTIC_PACKAGE_DATA_HOME")
	defer func() {
		if originalEnv == "" {
			os.Unsetenv("ELASTIC_PACKAGE_DATA_HOME")
		} else {
			os.Setenv("ELASTIC_PACKAGE_DATA_HOME", originalEnv)
		}
	}()
	os.Setenv("ELASTIC_PACKAGE_DATA_HOME", tempDir)

	result := LoadToolsets()
	// With no servers, we should get empty slice
	assert.Empty(t, result)
}

func TestLoadToolsets_ValidConfig_ServerWithoutURL(t *testing.T) {
	// Create a temporary config directory
	tempDir := t.TempDir()
	configDir := filepath.Join(tempDir, "llm_config")
	err := os.MkdirAll(configDir, 0o755)
	require.NoError(t, err)

	// Create valid JSON with server but no URL (should be skipped)
	command := "/usr/bin/node"
	config := MCPJson{
		Servers: map[string]MCPServer{
			"test-server": {
				Command: &command,
				Args:    []string{"server.js"},
			},
		},
	}
	data, err := json.Marshal(config)
	require.NoError(t, err)

	mcpFile := filepath.Join(configDir, "mcp.json")
	err = os.WriteFile(mcpFile, data, 0o644)
	require.NoError(t, err)

	originalEnv := os.Getenv("ELASTIC_PACKAGE_DATA_HOME")
	defer func() {
		if originalEnv == "" {
			os.Unsetenv("ELASTIC_PACKAGE_DATA_HOME")
		} else {
			os.Setenv("ELASTIC_PACKAGE_DATA_HOME", originalEnv)
		}
	}()
	os.Setenv("ELASTIC_PACKAGE_DATA_HOME", tempDir)

	result := LoadToolsets()
	// Server without URL should be skipped
	assert.Empty(t, result)
}
