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

	"github.com/elastic/elastic-package/internal/llmagent/framework"
	"github.com/elastic/elastic-package/internal/llmagent/providers"
)

// mockProvider implements a minimal LLMProvider for testing
type mockProvider struct{}

func (m *mockProvider) GenerateResponse(ctx context.Context, prompt string, tools []providers.Tool) (*providers.LLMResponse, error) {
	return &providers.LLMResponse{
		Content:  "mock response",
		Finished: true,
	}, nil
}

func (m *mockProvider) Name() string {
	return "mock"
}

func TestNewDocumentationAgent(t *testing.T) {
	tests := []struct {
		name          string
		provider      providers.LLMProvider
		packageRoot   string
		targetDocFile string
		setupFunc     func(*testing.T) string // Returns packageRoot path
		expectError   bool
		errorContains string
	}{
		{
			name:          "valid parameters",
			provider:      &mockProvider{},
			targetDocFile: "README.md",
			setupFunc: func(t *testing.T) string {
				// Create temporary directory with minimal manifest.yml
				tmpDir := t.TempDir()
				manifestContent := `format_version: "3.0.0"
name: test
title: Test Package
version: "1.0.0"
type: integration
`
				manifestPath := filepath.Join(tmpDir, "manifest.yml")
				if err := os.WriteFile(manifestPath, []byte(manifestContent), 0o644); err != nil {
					t.Fatalf("Failed to create test manifest: %v", err)
				}
				return tmpDir
			},
			expectError: false,
		},
		{
			name:          "nil provider",
			provider:      nil,
			packageRoot:   "/some/path",
			targetDocFile: "README.md",
			expectError:   true,
			errorContains: "provider cannot be nil",
		},
		{
			name:          "empty packageRoot",
			provider:      &mockProvider{},
			packageRoot:   "",
			targetDocFile: "README.md",
			expectError:   true,
			errorContains: "packageRoot cannot be empty",
		},
		{
			name:          "empty targetDocFile",
			provider:      &mockProvider{},
			packageRoot:   "/some/path",
			targetDocFile: "",
			expectError:   true,
			errorContains: "targetDocFile cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			packageRoot := tt.packageRoot
			if tt.setupFunc != nil {
				packageRoot = tt.setupFunc(t)
			}

			agent, err := NewDocumentationAgent(tt.provider, packageRoot, tt.targetDocFile, nil)

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
				assert.Nil(t, agent)
			} else {
				require.NoError(t, err)
				require.NotNil(t, agent)
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
	assert.NotEmpty(t, analyzer.tokenLimitIndicators)
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
		conversation   []framework.ConversationEntry
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
			conversation: []framework.ConversationEntry{
				{Type: "tool_result", Content: "✅ success"},
			},
			expectedStatus: responseSuccess,
		},
		{
			name:           "token limit indicator",
			content:        "I reached the maximum response length and need to continue",
			conversation:   nil,
			expectedStatus: responseTokenLimit,
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
			conversation: []framework.ConversationEntry{
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
		{
			name:           "token limit with specific phrase",
			content:        "Due to length constraints, I'll need to break this into sections",
			conversation:   nil,
			expectedStatus: responseTokenLimit,
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
		conversation []framework.ConversationEntry
		expected     bool
	}{
		{
			name:         "empty conversation",
			conversation: []framework.ConversationEntry{},
			expected:     false,
		},
		{
			name: "recent success",
			conversation: []framework.ConversationEntry{
				{Type: "tool_result", Content: "✅ success - file written"},
			},
			expected: true,
		},
		{
			name: "recent error marker",
			conversation: []framework.ConversationEntry{
				{Type: "tool_result", Content: "❌ error - file not found"},
			},
			expected: false,
		},
		{
			name: "success followed by error",
			conversation: []framework.ConversationEntry{
				{Type: "tool_result", Content: "successfully wrote file"},
				{Type: "tool_result", Content: "❌ error - something failed"},
			},
			expected: false,
		},
		{
			name: "error followed by success",
			conversation: []framework.ConversationEntry{
				{Type: "tool_result", Content: "❌ error - something failed"},
				{Type: "tool_result", Content: "completed successfully"},
			},
			expected: true,
		},
		{
			name: "success beyond lookback window",
			conversation: []framework.ConversationEntry{
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
			conversation: []framework.ConversationEntry{
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
