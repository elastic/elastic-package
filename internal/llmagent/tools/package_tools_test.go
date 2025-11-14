// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidatePathInRoot(t *testing.T) {
	tmpDir := t.TempDir()

	// Create some test directories so EvalSymlinks works consistently
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "data_stream", "logs"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "foo"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "bar"), 0o755))

	tests := []struct {
		name        string
		userPath    string
		shouldError bool
	}{
		{
			name:        "valid relative path",
			userPath:    "data_stream/logs",
			shouldError: false,
		},
		{
			name:        "root path",
			userPath:    "",
			shouldError: false,
		},
		{
			name:        "path traversal attack",
			userPath:    "../../../etc/passwd",
			shouldError: true,
		},
		{
			name:        "path with dot dot escape",
			userPath:    "foo/../../../etc/passwd",
			shouldError: true,
		},
		{
			name:        "valid path with dot dot inside",
			userPath:    "foo/../bar",
			shouldError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := validatePathInRoot(tmpDir, tt.userPath)
			if tt.shouldError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, result)
			}
		})
	}
}

func TestListDirectoryHandler(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test directory structure
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "data_stream"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "docs"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "manifest.yml"), []byte("test"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "docs", "README.md"), []byte("generated"), 0o644))

	handler := listDirectoryHandler(tmpDir)

	tests := []struct {
		name              string
		args              map[string]string
		expectError       bool
		expectContains    []string
		expectNotContains []string
	}{
		{
			name:              "list root directory",
			args:              map[string]string{"path": ""},
			expectContains:    []string{"data_stream/", "manifest.yml"},
			expectNotContains: []string{"docs/"}, // docs should be hidden
		},
		{
			name:           "list subdirectory",
			args:           map[string]string{"path": "data_stream"},
			expectContains: []string{"Contents of data_stream"},
		},
		{
			name:        "invalid path traversal",
			args:        map[string]string{"path": "../../etc"},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			argsJSON, _ := json.Marshal(tt.args)
			result, err := handler(context.Background(), string(argsJSON))
			require.NoError(t, err)
			require.NotNil(t, result)

			if tt.expectError {
				assert.NotEmpty(t, result.Error)
			} else {
				assert.Empty(t, result.Error)
				for _, contains := range tt.expectContains {
					assert.Contains(t, result.Content, contains)
				}
				for _, notContains := range tt.expectNotContains {
					assert.NotContains(t, result.Content, notContains)
				}
			}
		})
	}
}

func TestReadFileHandler(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test files
	testContent := "test content"
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte(testContent), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "docs"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "docs", "README.md"), []byte("generated"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "docs", "knowledge_base"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "docs", "knowledge_base", "info.md"), []byte("knowledge"), 0o644))

	handler := readFileHandler(tmpDir)

	tests := []struct {
		name          string
		path          string
		expectError   bool
		expectContent string
	}{
		{
			name:          "read valid file",
			path:          "test.txt",
			expectContent: testContent,
		},
		{
			name:        "block generated docs",
			path:        "docs/README.md",
			expectError: true,
		},
		{
			name:          "allow knowledge_base",
			path:          "docs/knowledge_base/info.md",
			expectContent: "knowledge",
		},
		{
			name:        "path traversal",
			path:        "../../../etc/passwd",
			expectError: true,
		},
		{
			name:        "nonexistent file",
			path:        "nonexistent.txt",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := map[string]string{"path": tt.path}
			argsJSON, _ := json.Marshal(args)
			result, err := handler(context.Background(), string(argsJSON))
			require.NoError(t, err)
			require.NotNil(t, result)

			if tt.expectError {
				assert.NotEmpty(t, result.Error)
			} else {
				assert.Empty(t, result.Error)
				assert.Equal(t, tt.expectContent, result.Content)
			}
		})
	}
}

func TestWriteFileHandler(t *testing.T) {
	tmpDir := t.TempDir()

	handler := writeFileHandler(tmpDir)

	tests := []struct {
		name        string
		path        string
		content     string
		expectError bool
	}{
		{
			name:        "write outside allowed directory - manifest",
			path:        "manifest.yml",
			expectError: true,
		},
		{
			name:        "write in docs root - not allowed",
			path:        "docs/README.md",
			expectError: true,
		},
		{
			name:        "write in _dev root - not allowed",
			path:        "_dev/README.md",
			expectError: true,
		},
		{
			name:        "path traversal attempt",
			path:        "_dev/build/docs/../../../etc/passwd",
			expectError: true,
		},
		{
			name:        "absolute path attempt",
			path:        "/etc/passwd",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := map[string]interface{}{
				"path":    tt.path,
				"content": tt.content,
			}
			argsJSON, _ := json.Marshal(args)
			result, err := handler(context.Background(), string(argsJSON))
			require.NoError(t, err)
			require.NotNil(t, result)

			if tt.expectError {
				assert.NotEmpty(t, result.Error, "Expected error for path: %s", tt.path)
			} else {
				assert.Empty(t, result.Error, "Unexpected error: %s", result.Error)
			}
		})
	}
}

func TestGetReadmeTemplateHandler(t *testing.T) {
	handler := getReadmeTemplateHandler()
	result, err := handler(context.Background(), "{}")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Empty(t, result.Error)
	assert.NotEmpty(t, result.Content)
}

func TestGetExampleReadmeHandler(t *testing.T) {
	handler := getExampleReadmeHandler()
	result, err := handler(context.Background(), "{}")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Empty(t, result.Error)
	assert.NotEmpty(t, result.Content)
}

func TestPackageTools(t *testing.T) {
	tmpDir := t.TempDir()
	tools := PackageTools(tmpDir)

	// Verify all expected tools are present
	expectedTools := []string{
		"list_directory",
		"read_file",
		"write_file",
		"get_readme_template",
		"get_example_readme",
	}

	assert.Len(t, tools, len(expectedTools))

	for i, name := range expectedTools {
		assert.Equal(t, name, tools[i].Name)
		assert.NotEmpty(t, tools[i].Description)
		assert.NotNil(t, tools[i].Handler)
		assert.NotNil(t, tools[i].Parameters)
	}
}
