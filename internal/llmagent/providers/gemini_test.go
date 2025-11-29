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

func TestNewGeminiProvider(t *testing.T) {
	t.Run("creates provider with custom config", func(t *testing.T) {
		config := GeminiConfig{
			APIKey:   "test-api-key",
			ModelID:  "gemini-2.0-pro",
			Endpoint: "https://test.googleapis.com",
		}

		provider := NewGeminiProvider(config)

		assert.NotNil(t, provider)
		assert.Equal(t, "test-api-key", provider.apiKey)
		assert.Equal(t, "gemini-2.0-pro", provider.modelID)
		assert.Equal(t, "https://test.googleapis.com", provider.endpoint)
		assert.NotNil(t, provider.client)
	})

	t.Run("uses default model and endpoint", func(t *testing.T) {
		config := GeminiConfig{
			APIKey: "test-api-key",
		}

		provider := NewGeminiProvider(config)

		assert.Equal(t, "gemini-2.5-pro", provider.modelID)
		assert.Equal(t, "https://generativelanguage.googleapis.com/v1beta", provider.endpoint)
	})
}

func TestGeminiProvider_Name(t *testing.T) {
	provider := NewGeminiProvider(GeminiConfig{APIKey: "test"})
	assert.Equal(t, "Gemini", provider.Name())
}

func TestGeminiProvider_GenerateResponse(t *testing.T) {
	t.Run("successful response with text", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "POST", r.Method)
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

			response := googleResponse{
				Candidates: []googleCandidate{
					{
						Content: googleContent{
							Parts: []googlePart{
								{Text: "Hello, how can I help?"},
							},
						},
						FinishReason: finishReasonStop,
					},
				},
			}
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		provider := NewGeminiProvider(GeminiConfig{
			APIKey:   "test-key",
			Endpoint: server.URL,
		})

		resp, err := provider.GenerateResponse(context.Background(), "test prompt", nil)

		require.NoError(t, err)
		assert.Equal(t, "Hello, how can I help?", resp.Content)
		assert.True(t, resp.Finished)
		assert.Empty(t, resp.ToolCalls)
	})

	t.Run("response with function call", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			response := googleResponse{
				Candidates: []googleCandidate{
					{
						Content: googleContent{
							Parts: []googlePart{
								{
									FunctionCall: &googleFunctionCall{
										Name: "test_function",
										Args: map[string]interface{}{
											"arg1": "value1",
										},
									},
								},
							},
						},
						FinishReason: "",
					},
				},
			}
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		provider := NewGeminiProvider(GeminiConfig{
			APIKey:   "test-key",
			Endpoint: server.URL,
		})

		tools := []Tool{
			{
				Name:        "test_function",
				Description: "A test function",
				Parameters: map[string]interface{}{
					"type": "object",
				},
			},
		}

		resp, err := provider.GenerateResponse(context.Background(), "test prompt", tools)

		require.NoError(t, err)
		assert.Len(t, resp.ToolCalls, 1)
		assert.Equal(t, "test_function", resp.ToolCalls[0].Name)
		assert.Contains(t, resp.ToolCalls[0].Arguments, "value1")
		assert.False(t, resp.Finished)
	})

	t.Run("handles max tokens finish reason", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			response := googleResponse{
				Candidates: []googleCandidate{
					{
						Content: googleContent{
							Parts: []googlePart{
								{Text: "Partial response..."},
							},
						},
						FinishReason: finishReasonMaxTokens,
					},
				},
			}
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		provider := NewGeminiProvider(GeminiConfig{
			APIKey:   "test-key",
			Endpoint: server.URL,
		})

		resp, err := provider.GenerateResponse(context.Background(), "test prompt", nil)

		require.NoError(t, err)
		assert.True(t, resp.Finished)
		assert.Contains(t, resp.Content, "maximum response length")
	})

	t.Run("handles API error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("invalid request"))
		}))
		defer server.Close()

		provider := NewGeminiProvider(GeminiConfig{
			APIKey:   "test-key",
			Endpoint: server.URL,
		})

		_, err := provider.GenerateResponse(context.Background(), "test prompt", nil)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "400")
	})
}
