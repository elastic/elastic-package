// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package mcptools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/elastic-package/internal/llmagent/providers"
)

func TestMCPServer_Connect_NilURL(t *testing.T) {
	server := &MCPServer{}
	err := server.Connect()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "URL is required")
}

func TestMCPServer_Close(t *testing.T) {
	server := &MCPServer{}

	// Should not error even if session is nil
	err := server.Close()
	require.NoError(t, err)
	assert.Nil(t, server.session)
}

func TestLoadTools_NoConfigFile(t *testing.T) {
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

	result := LoadTools()
	assert.Nil(t, result, "Expected nil when config file doesn't exist")
}

func TestLoadTools_InvalidJSON(t *testing.T) {
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

	result := LoadTools()
	assert.Nil(t, result, "Expected nil when JSON is invalid")
}

func TestLoadTools_ValidConfig_NoServers(t *testing.T) {
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

	result := LoadTools()
	require.NotNil(t, result)
	assert.Empty(t, result.Servers)
}

func TestLoadTools_ValidConfig_WithPrompts(t *testing.T) {
	// Create a temporary config directory
	tempDir := t.TempDir()
	configDir := filepath.Join(tempDir, "llm_config")
	err := os.MkdirAll(configDir, 0o755)
	require.NoError(t, err)

	// Create valid JSON with prompts
	initialPrompt := "initial.txt"
	revisionPrompt := "revision.txt"
	config := MCPJson{
		InitialPrompt:  &initialPrompt,
		RevisionPrompt: &revisionPrompt,
		Servers:        map[string]MCPServer{},
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

	result := LoadTools()
	require.NotNil(t, result)
	assert.NotNil(t, result.InitialPrompt)
	assert.Equal(t, "initial.txt", *result.InitialPrompt)
	assert.NotNil(t, result.RevisionPrompt)
	assert.Equal(t, "revision.txt", *result.RevisionPrompt)
}

func TestLoadTools_ValidConfig_ServerWithoutURL(t *testing.T) {
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

	result := LoadTools()
	require.NotNil(t, result)
	assert.Len(t, result.Servers, 1)
	// Server should exist but not be connected (no Tools loaded)
	server := result.Servers["test-server"]
	assert.NotNil(t, server.Command)
	assert.Nil(t, server.session)
}

func TestMCPServer_ToolHandler(t *testing.T) {
	// Test that the tool handler function signature works correctly
	toolName := "test-tool"
	server := &MCPServer{}

	// Create a mock tool (without actually connecting to MCP server)
	// This tests the tool structure and handler setup
	handler := func(ctx context.Context, arguments string) (*providers.ToolResult, error) {
		return &providers.ToolResult{Content: "test result"}, nil
	}

	// Verify handler can be called
	ctx := context.Background()
	result, err := handler(ctx, `{"test": "value"}`)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "test result", result.Content)

	// Verify tool structure matches expected format
	tool := providers.Tool{
		Name:        toolName,
		Description: "Test tool description",
		Parameters: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
			"required":   []string{},
		},
		Handler: handler,
	}

	assert.Equal(t, toolName, tool.Name)
	assert.NotNil(t, tool.Handler)
	assert.NotNil(t, tool.Parameters)

	// Verify server tools can be appended
	server.Tools = append(server.Tools, tool)
	assert.Len(t, server.Tools, 1)
	assert.Equal(t, toolName, server.Tools[0].Name)
}
