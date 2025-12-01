// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package providers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLLMProviderInterface(t *testing.T) {
	t.Run("GeminiProvider implements LLMProvider", func(t *testing.T) {
		var _ LLMProvider = &GeminiProvider{}
	})

	t.Run("LocalProvider implements LLMProvider", func(t *testing.T) {
		var _ LLMProvider = &LocalProvider{}
	})
}

func TestLLMResponse(t *testing.T) {
	t.Run("creates response with tool calls", func(t *testing.T) {
		response := &LLMResponse{
			Content: "test content",
			ToolCalls: []ToolCall{
				{
					ID:        "call_1",
					Name:      "test_tool",
					Arguments: `{"arg1": "value1"}`,
				},
			},
			Finished: false,
		}

		assert.Equal(t, "test content", response.Content)
		assert.Len(t, response.ToolCalls, 1)
		assert.Equal(t, "call_1", response.ToolCalls[0].ID)
		assert.False(t, response.Finished)
	})
}

func TestToolResult(t *testing.T) {
	t.Run("creates tool result with content", func(t *testing.T) {
		result := &ToolResult{
			Content: "success",
			Error:   "",
		}

		assert.Equal(t, "success", result.Content)
		assert.Empty(t, result.Error)
	})

	t.Run("creates tool result with error", func(t *testing.T) {
		result := &ToolResult{
			Content: "",
			Error:   "failed to execute",
		}

		assert.Empty(t, result.Content)
		assert.Equal(t, "failed to execute", result.Error)
	})
}
