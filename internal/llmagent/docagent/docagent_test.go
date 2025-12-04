// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package docagent

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/elastic-package/internal/llmagent/agent"
)

func TestNewDocumentationAgent_Validation(t *testing.T) {
	tests := []struct {
		name          string
		apiKey        string
		packageRoot   string
		targetDocFile string
		setupFunc     func(*testing.T) string // Returns packageRoot path
		expectError   bool
		errorContains string
	}{
		{
			name:          "empty API key",
			apiKey:        "",
			packageRoot:   "/some/path",
			targetDocFile: "README.md",
			expectError:   true,
			errorContains: "API key cannot be empty",
		},
		{
			name:          "empty packageRoot",
			apiKey:        "test-key",
			packageRoot:   "",
			targetDocFile: "README.md",
			expectError:   true,
			errorContains: "packageRoot cannot be empty",
		},
		{
			name:          "empty targetDocFile",
			apiKey:        "test-key",
			packageRoot:   "/some/path",
			targetDocFile: "",
			expectError:   true,
			errorContains: "targetDocFile cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			packageRoot := tt.packageRoot
			if tt.setupFunc != nil {
				packageRoot = tt.setupFunc(t)
			}

			cfg := AgentConfig{
				APIKey:      tt.apiKey,
				PackageRoot: packageRoot,
				DocFile:     tt.targetDocFile,
			}
			docAgent, err := NewDocumentationAgent(ctx, cfg)

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
				assert.Nil(t, docAgent)
			} else {
				require.NoError(t, err)
				require.NotNil(t, docAgent)
			}
		})
	}
}

func TestNewResponseAnalyzer(t *testing.T) {
	analyzer := NewResponseAnalyzer()

	require.NotNil(t, analyzer)
	assert.NotEmpty(t, analyzer.successIndicators)
	assert.NotEmpty(t, analyzer.errorIndicators)
	assert.NotEmpty(t, analyzer.errorMarkers)
}

func TestResponseAnalyzer_ContainsAnyIndicator(t *testing.T) {
	analyzer := NewResponseAnalyzer()

	tests := []struct {
		name       string
		content    string
		indicators []string
		expected   bool
	}{
		{
			name:       "exact match",
			content:    "This is an error message",
			indicators: []string{"error message"},
			expected:   true,
		},
		{
			name:       "case insensitive match",
			content:    "This is an ERROR message",
			indicators: []string{"error message"},
			expected:   true,
		},
		{
			name:       "no match",
			content:    "This is a success message",
			indicators: []string{"error", "failed"},
			expected:   false,
		},
		{
			name:       "partial match",
			content:    "Task failed successfully",
			indicators: []string{"failed"},
			expected:   true,
		},
		{
			name:       "empty content",
			content:    "",
			indicators: []string{"error"},
			expected:   false,
		},
		{
			name:       "empty indicators",
			content:    "some content",
			indicators: []string{},
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := analyzer.containsAnyIndicator(tt.content, tt.indicators)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestResponseAnalyzer_AnalyzeResponse(t *testing.T) {
	analyzer := NewResponseAnalyzer()

	tests := []struct {
		name           string
		content        string
		conversation   []agent.ConversationEntry
		expectedStatus responseStatus
	}{
		{
			name:           "empty content without tools",
			content:        "",
			conversation:   nil,
			expectedStatus: responseEmpty,
		},
		{
			name:    "empty content with successful tools",
			content: "",
			conversation: []agent.ConversationEntry{
				{Type: "tool_result", Content: "✅ success"},
			},
			expectedStatus: responseSuccess,
		},
		{
			name:           "error indicator",
			content:        "I encountered an error while processing",
			conversation:   nil,
			expectedStatus: responseError,
		},
		{
			name:    "error indicator but tools succeeded",
			content: "I encountered an error while processing",
			conversation: []agent.ConversationEntry{
				{Type: "tool_result", Content: "successfully wrote file"},
			},
			expectedStatus: responseSuccess,
		},
		{
			name:           "normal success response",
			content:        "I have completed the documentation update",
			conversation:   nil,
			expectedStatus: responseSuccess,
		},
		{
			name:           "multiple error indicators",
			content:        "Something went wrong and I'm unable to complete the task",
			conversation:   nil,
			expectedStatus: responseError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analysis := analyzer.AnalyzeResponse(tt.content, tt.conversation)
			assert.Equal(t, tt.expectedStatus, analysis.Status)
			assert.NotEmpty(t, analysis.Message)
		})
	}
}

func TestResponseAnalyzer_HasRecentSuccessfulTools(t *testing.T) {
	analyzer := NewResponseAnalyzer()

	tests := []struct {
		name         string
		conversation []agent.ConversationEntry
		expected     bool
	}{
		{
			name:         "empty conversation",
			conversation: []agent.ConversationEntry{},
			expected:     false,
		},
		{
			name: "recent success",
			conversation: []agent.ConversationEntry{
				{Type: "tool_result", Content: "✅ success - file written"},
			},
			expected: true,
		},
		{
			name: "recent error marker",
			conversation: []agent.ConversationEntry{
				{Type: "tool_result", Content: "❌ error - file not found"},
			},
			expected: false,
		},
		{
			name: "success followed by error",
			conversation: []agent.ConversationEntry{
				{Type: "tool_result", Content: "successfully wrote file"},
				{Type: "tool_result", Content: "❌ error - something failed"},
			},
			expected: false,
		},
		{
			name: "error followed by success",
			conversation: []agent.ConversationEntry{
				{Type: "tool_result", Content: "❌ error - something failed"},
				{Type: "tool_result", Content: "completed successfully"},
			},
			expected: true,
		},
		{
			name: "success beyond lookback window",
			conversation: []agent.ConversationEntry{
				{Type: "tool_result", Content: "✅ success"},
				{Type: "user", Content: "message 1"},
				{Type: "user", Content: "message 2"},
				{Type: "user", Content: "message 3"},
				{Type: "user", Content: "message 4"},
				{Type: "user", Content: "message 5"},
				{Type: "user", Content: "message 6"},
			},
			expected: false,
		},
		{
			name: "non-tool entries",
			conversation: []agent.ConversationEntry{
				{Type: "user", Content: "user message"},
				{Type: "assistant", Content: "assistant message"},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := analyzer.hasRecentSuccessfulTools(tt.conversation)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestWriteDocumentation(t *testing.T) {
	tmpDir := t.TempDir()

	// Create minimal DocumentationAgent for testing
	da := &DocumentationAgent{
		packageRoot:   tmpDir,
		targetDocFile: "README.md",
	}

	testContent := "# Test Documentation\n\nThis is a test."
	docPath := filepath.Join(tmpDir, "_dev", "build", "docs", "README.md")

	// Test writing documentation
	err := da.writeDocumentation(docPath, testContent)
	require.NoError(t, err)

	// Verify file was created
	content, err := os.ReadFile(docPath)
	require.NoError(t, err)
	assert.Equal(t, testContent, string(content))
}
