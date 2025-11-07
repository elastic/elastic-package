// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package framework

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/elastic-package/internal/llmagent/providers"
)

// mockProvider is a mock LLM provider for testing
type mockProvider struct {
	responses []*providers.LLMResponse
	callCount int
}

func (m *mockProvider) GenerateResponse(ctx context.Context, prompt string, tools []providers.Tool) (*providers.LLMResponse, error) {
	if m.callCount >= len(m.responses) {
		return nil, errors.New("no more responses configured")
	}
	response := m.responses[m.callCount]
	m.callCount++
	return response, nil
}

func (m *mockProvider) Name() string {
	return "mock"
}

func TestNewAgent(t *testing.T) {
	provider := &mockProvider{}
	tools := []providers.Tool{
		{Name: "test_tool", Description: "Test tool"},
	}

	agent := NewAgent(provider, tools)

	assert.NotNil(t, agent)
	assert.Equal(t, provider, agent.provider)
	assert.Equal(t, tools, agent.tools)
}

func TestExecuteTask_SuccessfulCompletion(t *testing.T) {
	provider := &mockProvider{
		responses: []*providers.LLMResponse{
			{
				Content:  "Task completed successfully",
				Finished: true,
			},
		},
	}

	agent := NewAgent(provider, nil)
	result, err := agent.ExecuteTask(context.Background(), "Do something")

	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, "Task completed successfully", result.FinalContent)
	assert.Len(t, result.Conversation, 2) // User prompt + assistant response
}

func TestExecuteTask_WithSuccessfulToolCall(t *testing.T) {
	toolCalled := false
	testTool := providers.Tool{
		Name:        "test_tool",
		Description: "A test tool",
		Handler: func(ctx context.Context, arguments string) (*providers.ToolResult, error) {
			toolCalled = true
			return &providers.ToolResult{
				Content: "Tool executed successfully",
			}, nil
		},
	}

	provider := &mockProvider{
		responses: []*providers.LLMResponse{
			{
				Content: "I'll use the test tool",
				ToolCalls: []providers.ToolCall{
					{ID: "1", Name: "test_tool", Arguments: "{}"},
				},
			},
			{
				Content:  "Tool result received, task complete",
				Finished: true,
			},
		},
	}

	agent := NewAgent(provider, []providers.Tool{testTool})
	result, err := agent.ExecuteTask(context.Background(), "Use the tool")

	require.NoError(t, err)
	assert.True(t, toolCalled)
	assert.True(t, result.Success)
	assert.Contains(t, result.Conversation[2].Content, "SUCCESS")
	assert.Contains(t, result.Conversation[2].Content, "Tool executed successfully")
}

func TestExecuteTask_WithToolError(t *testing.T) {
	testTool := providers.Tool{
		Name:        "failing_tool",
		Description: "A tool that fails",
		Handler: func(ctx context.Context, arguments string) (*providers.ToolResult, error) {
			return &providers.ToolResult{
				Error: "tool execution failed",
			}, nil
		},
	}

	provider := &mockProvider{
		responses: []*providers.LLMResponse{
			{
				Content: "I'll use the failing tool",
				ToolCalls: []providers.ToolCall{
					{ID: "1", Name: "failing_tool", Arguments: "{}"},
				},
			},
			{
				Content:  "Tool failed, but I'll handle it",
				Finished: true,
			},
		},
	}

	agent := NewAgent(provider, []providers.Tool{testTool})
	result, err := agent.ExecuteTask(context.Background(), "Use the tool")

	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Contains(t, result.Conversation[2].Content, "ERROR")
	assert.Contains(t, result.Conversation[2].Content, "tool execution failed")
}

func TestExecuteTask_MaxIterationsReached(t *testing.T) {
	// Provider that never finishes
	responses := make([]*providers.LLMResponse, maxIterations+1)
	for i := range responses {
		responses[i] = &providers.LLMResponse{
			Content:  "Still working...",
			Finished: false,
		}
	}
	provider := &mockProvider{responses: responses}

	agent := NewAgent(provider, nil)
	result, err := agent.ExecuteTask(context.Background(), "Never-ending task")

	require.NoError(t, err)
	assert.False(t, result.Success)
	assert.Contains(t, result.FinalContent, "maximum iterations")
}

func TestExecuteTask_ToolNotFound(t *testing.T) {
	provider := &mockProvider{
		responses: []*providers.LLMResponse{
			{
				Content: "I'll use a non-existent tool",
				ToolCalls: []providers.ToolCall{
					{ID: "1", Name: "nonexistent_tool", Arguments: "{}"},
				},
			},
			{
				Content:  "Handled the error",
				Finished: true,
			},
		},
	}

	agent := NewAgent(provider, nil)
	result, err := agent.ExecuteTask(context.Background(), "Use unknown tool")

	require.NoError(t, err)
	assert.Contains(t, result.Conversation[2].Content, "ERROR")
	assert.Contains(t, result.Conversation[2].Content, "tool not found")
}

func TestExecuteTask_FalseErrorDetection(t *testing.T) {
	testTool := providers.Tool{
		Name: "working_tool",
		Handler: func(ctx context.Context, arguments string) (*providers.ToolResult, error) {
			return &providers.ToolResult{Content: "Success"}, nil
		},
	}

	provider := &mockProvider{
		responses: []*providers.LLMResponse{
			{
				Content: "Using tool",
				ToolCalls: []providers.ToolCall{
					{ID: "1", Name: "working_tool", Arguments: "{}"},
				},
			},
			{
				Content:  "I encountered an error while trying to call the function",
				Finished: false,
			},
			{
				Content:  "Actually, everything worked fine",
				Finished: true,
			},
		},
	}

	agent := NewAgent(provider, []providers.Tool{testTool})
	result, err := agent.ExecuteTask(context.Background(), "Test false error")

	require.NoError(t, err)
	assert.True(t, result.Success)
	// Should have injected a clarification message
	assert.Contains(t, result.Conversation[4].Content, "IMPORTANT CLARIFICATION")
}

func TestDetectFalseToolError(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		recentTools []ToolExecutionInfo
		expected    bool
	}{
		{
			name:        "no recent tools",
			content:     "I encountered an error",
			recentTools: []ToolExecutionInfo{},
			expected:    false,
		},
		{
			name:    "error indicator with successful tools",
			content: "I encountered an error while trying to call the function",
			recentTools: []ToolExecutionInfo{
				{ToolName: "test", Success: true, ResultType: "success"},
			},
			expected: true,
		},
		{
			name:    "error indicator with actual error",
			content: "I encountered an error",
			recentTools: []ToolExecutionInfo{
				{ToolName: "test", Success: false, ResultType: "error"},
			},
			expected: false,
		},
		{
			name:    "no error indicator",
			content: "Everything is working fine",
			recentTools: []ToolExecutionInfo{
				{ToolName: "test", Success: true, ResultType: "success"},
			},
			expected: false,
		},
	}

	agent := &Agent{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := agent.detectFalseToolError(tt.content, tt.recentTools)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatToolSuccess(t *testing.T) {
	agent := &Agent{}
	result := agent.formatToolSuccess("my_tool", "operation succeeded")

	assert.Contains(t, result, "SUCCESS")
	assert.Contains(t, result, "my_tool")
	assert.Contains(t, result, "operation succeeded")
}

func TestFormatToolError(t *testing.T) {
	agent := &Agent{}
	result := agent.formatToolError("my_tool", errors.New("something went wrong"))

	assert.Contains(t, result, "ERROR")
	assert.Contains(t, result, "my_tool")
	assert.Contains(t, result, "something went wrong")
}

func TestBuildPrompt(t *testing.T) {
	agent := &Agent{}
	conversation := []ConversationEntry{
		{Type: "user", Content: "Hello"},
		{Type: "assistant", Content: "Hi there"},
		{Type: "tool_result", Content: "Tool executed"},
	}

	prompt := agent.buildPrompt(conversation)

	assert.Contains(t, prompt, "Human: Hello")
	assert.Contains(t, prompt, "Assistant: Hi there")
	assert.Contains(t, prompt, "Tool Result: Tool executed")
}

func TestBuildToolClarificationPrompt(t *testing.T) {
	agent := &Agent{}
	recentTools := []ToolExecutionInfo{
		{ToolName: "tool1", Success: true, ResultType: "success", Result: "All good"},
		{ToolName: "tool2", Success: false, ResultType: "error", Result: "Failed"},
	}

	prompt := agent.buildToolClarificationPrompt(recentTools)

	assert.Contains(t, prompt, "IMPORTANT CLARIFICATION")
	assert.Contains(t, prompt, "tool1")
	assert.Contains(t, prompt, "tool2")
	assert.Contains(t, prompt, "SUCCEEDED")
	assert.Contains(t, prompt, "FAILED")
}
