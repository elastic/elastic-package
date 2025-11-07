// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package providers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLocalProvider(t *testing.T) {
	t.Run("creates provider with custom config", func(t *testing.T) {
		config := LocalConfig{
			Endpoint: "http://localhost:8080",
			ModelID:  "custom-model",
			APIKey:   "test-key",
		}

		provider := NewLocalProvider(config)

		assert.NotNil(t, provider)
		assert.Equal(t, "http://localhost:8080", provider.endpoint)
		assert.Equal(t, "custom-model", provider.modelID)
		assert.Equal(t, "test-key", provider.apiKey)
		assert.NotNil(t, provider.client)
	})

	t.Run("uses default model and endpoint", func(t *testing.T) {
		config := LocalConfig{}

		provider := NewLocalProvider(config)

		assert.Equal(t, "llama2", provider.modelID)
		assert.Equal(t, "http://localhost:11434", provider.endpoint)
		assert.Empty(t, provider.apiKey)
	})
}

func TestLocalProvider_Name(t *testing.T) {
	provider := NewLocalProvider(LocalConfig{})
	assert.Equal(t, "Local LLM", provider.Name())
}

func TestLocalProvider_GenerateResponse(t *testing.T) {
	t.Run("successful response with text", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "POST", r.Method)
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
			assert.Equal(t, "/v1/chat/completions", r.URL.Path)

			response := openaiResponse{
				Choices: []choice{
					{
						Message: openaiMessage{
							Role:    "assistant",
							Content: "This is a test response",
						},
						FinishReason: "stop",
					},
				},
			}
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		provider := NewLocalProvider(LocalConfig{
			Endpoint: server.URL,
		})

		resp, err := provider.GenerateResponse(context.Background(), "test prompt", nil)

		require.NoError(t, err)
		assert.Equal(t, "This is a test response", resp.Content)
		assert.True(t, resp.Finished)
		assert.Empty(t, resp.ToolCalls)
	})

	t.Run("response with tool calls", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			response := openaiResponse{
				Choices: []choice{
					{
						Message: openaiMessage{
							Role:    "assistant",
							Content: "",
							ToolCalls: []openaiToolCall{
								{
									ID:   "call_123",
									Type: "function",
									Function: openaiFunction{
										Name:      "test_tool",
										Arguments: `{"param": "value"}`,
									},
								},
							},
						},
						FinishReason: "tool_calls",
					},
				},
			}
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		provider := NewLocalProvider(LocalConfig{
			Endpoint: server.URL,
		})

		tools := []Tool{
			{
				Name:        "test_tool",
				Description: "A test tool",
				Parameters: map[string]interface{}{
					"type": "object",
				},
			},
		}

		resp, err := provider.GenerateResponse(context.Background(), "test prompt", tools)

		require.NoError(t, err)
		assert.Len(t, resp.ToolCalls, 1)
		assert.Equal(t, "call_123", resp.ToolCalls[0].ID)
		assert.Equal(t, "test_tool", resp.ToolCalls[0].Name)
		assert.Contains(t, resp.ToolCalls[0].Arguments, "value")
		assert.False(t, resp.Finished)
	})

	t.Run("includes authorization header when API key provided", func(t *testing.T) {
		var capturedAuthHeader string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedAuthHeader = r.Header.Get("Authorization")
			response := openaiResponse{
				Choices: []choice{
					{
						Message: openaiMessage{
							Content: "test",
						},
						FinishReason: "stop",
					},
				},
			}
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		provider := NewLocalProvider(LocalConfig{
			Endpoint: server.URL,
			APIKey:   "secret-key",
		})

		_, err := provider.GenerateResponse(context.Background(), "test", nil)

		require.NoError(t, err)
		assert.Equal(t, "Bearer secret-key", capturedAuthHeader)
	})

	t.Run("handles API error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("server error"))
		}))
		defer server.Close()

		provider := NewLocalProvider(LocalConfig{
			Endpoint: server.URL,
		})

		_, err := provider.GenerateResponse(context.Background(), "test", nil)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "500")
	})

	t.Run("handles empty response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			response := openaiResponse{
				Choices: []choice{},
			}
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		provider := NewLocalProvider(LocalConfig{
			Endpoint: server.URL,
		})

		resp, err := provider.GenerateResponse(context.Background(), "test", nil)

		require.NoError(t, err)
		assert.Empty(t, resp.Content)
		assert.False(t, resp.Finished)
		assert.Empty(t, resp.ToolCalls)
	})
}
