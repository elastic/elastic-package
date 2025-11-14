// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/elastic/elastic-package/internal/logger"
)

// LocalProvider implements LLMProvider for local LLM servers (Ollama, LocalAI, etc.)
type LocalProvider struct {
	endpoint string
	modelID  string
	apiKey   string // Optional for some local servers
	client   *http.Client
}

// LocalConfig holds configuration for the Local LLM provider
type LocalConfig struct {
	Endpoint string
	ModelID  string
	APIKey   string
}

// OpenAI-compatible types for API communication
type openaiRequest struct {
	Model       string          `json:"model"`
	Messages    []openaiMessage `json:"messages"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Temperature float64         `json:"temperature,omitempty"`
	Stream      bool            `json:"stream,omitempty"`
	Tools       []openaiTool    `json:"tools,omitempty"`
	ToolChoice  string          `json:"tool_choice,omitempty"`
}

type openaiResponse struct {
	Choices []choice `json:"choices"`
	Usage   usage    `json:"usage,omitempty"`
}

type openaiMessage struct {
	Role      string           `json:"role"`
	Content   string           `json:"content"`
	ToolCalls []openaiToolCall `json:"tool_calls,omitempty"`
}

type openaiTool struct {
	Type     string         `json:"type"`
	Function openaiFunction `json:"function"`
}

type openaiFunction struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
	Arguments   string                 `json:"arguments,omitempty"`
}

type openaiToolCall struct {
	ID       string         `json:"id"`
	Type     string         `json:"type"`
	Function openaiFunction `json:"function"`
}

type choice struct {
	Index        int           `json:"index"`
	Message      openaiMessage `json:"message"`
	FinishReason string        `json:"finish_reason"`
}

type usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// NewLocalProvider creates a new Local LLM provider
func NewLocalProvider(config LocalConfig) *LocalProvider {
	if config.ModelID == "" {
		config.ModelID = "llama2" // Default model for Ollama
	}
	if config.Endpoint == "" {
		config.Endpoint = "http://localhost:11434" // Default Ollama endpoint
	}

	// Debug logging with masked API key for security
	logger.Debugf("Creating Local LLM provider with model: %s, endpoint: %s",
		config.ModelID, config.Endpoint)
	if config.APIKey != "" {
		logger.Debugf("API key (masked for security): %s", maskAPIKey(config.APIKey))
	} else {
		logger.Debugf("No API key configured")
	}

	return &LocalProvider{
		endpoint: config.Endpoint,
		modelID:  config.ModelID,
		apiKey:   config.APIKey,
		client: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

// Name returns the provider name
func (l *LocalProvider) Name() string {
	return "Local LLM"
}

// GenerateResponse sends a prompt to the local LLM and returns the response
func (l *LocalProvider) GenerateResponse(ctx context.Context, prompt string, tools []Tool) (*LLMResponse, error) {
	// Convert tools to OpenAI format
	openaiTools := make([]openaiTool, len(tools))
	for i, t := range tools {
		openaiTools[i] = openaiTool{
			Type: "function",
			Function: openaiFunction{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.Parameters,
			},
		}
	}

	// Prepare request payload using OpenAI-compatible format
	requestPayload := openaiRequest{
		Model: l.modelID,
		Messages: []openaiMessage{
			{
				Role:    "user",
				Content: prompt,
			},
		},
		MaxTokens:   8192, // Increased for documentation generation
		Temperature: 0.7,
		Stream:      false,
	}

	// Add tools if any are provided
	if len(openaiTools) > 0 {
		requestPayload.Tools = openaiTools
		requestPayload.ToolChoice = "auto"
	}

	jsonPayload, err := json.Marshal(requestPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	url := fmt.Sprintf("%s/v1/chat/completions", l.endpoint)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	if l.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+l.apiKey)
	}

	// Send request
	resp, err := l.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errBody bytes.Buffer
		io.Copy(&errBody, resp.Body)
		return nil, fmt.Errorf("local LLM API returned status %d: %s", resp.StatusCode, errBody.String())
	}

	// Parse response
	var openaiResp openaiResponse
	if err := json.NewDecoder(resp.Body).Decode(&openaiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Debug logging for the full response
	logger.Debugf("Local LLM API response - Choices count: %d", len(openaiResp.Choices))
	if len(openaiResp.Choices) > 0 {
		choice := openaiResp.Choices[0]
		logger.Debugf("Local LLM API response - FinishReason: %s", choice.FinishReason)
		logger.Debugf("Local LLM API response - Content: %s", choice.Message.Content)
		if len(choice.Message.ToolCalls) > 0 {
			logger.Debugf("Local LLM API response - ToolCalls count: %d", len(choice.Message.ToolCalls))
			for i, toolCall := range choice.Message.ToolCalls {
				logger.Debugf("Local LLM API response - ToolCall[%d]: name=%s, id=%s, args=%s",
					i, toolCall.Function.Name, toolCall.ID, toolCall.Function.Arguments)
			}
		}
	}

	// Convert to our format
	response := &LLMResponse{
		ToolCalls: []ToolCall{},
		Finished:  false,
	}

	if len(openaiResp.Choices) > 0 {
		choice := openaiResp.Choices[0]
		response.Content = choice.Message.Content
		response.Finished = choice.FinishReason == "stop"

		// Convert tool calls
		for i, toolCall := range choice.Message.ToolCalls {
			response.ToolCalls = append(response.ToolCalls, ToolCall{
				ID:        toolCall.ID,
				Name:      toolCall.Function.Name,
				Arguments: toolCall.Function.Arguments,
			})
			logger.Debugf("Converted ToolCall[%d]: ID=%s, Name=%s", i, toolCall.ID, toolCall.Function.Name)
		}
	}

	return response, nil
}
