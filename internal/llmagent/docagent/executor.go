// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package docagent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model"
	"google.golang.org/adk/model/gemini"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"
	"google.golang.org/adk/tool"
	"google.golang.org/genai"

	"github.com/elastic/elastic-package/internal/llmagent/tracing"
	"github.com/elastic/elastic-package/internal/logger"
)

const (
	maxIterations = 15
	appName       = "elastic-package"
	defaultUserID = "default-user"
)

// ExecutorConfig holds configuration for creating an Executor.
type ExecutorConfig struct {
	APIKey      string
	ModelID     string
	Instruction string
}

// TaskResult represents the result of a task execution.
type TaskResult struct {
	Success      bool
	FinalContent string
	Conversation []ConversationEntry
}

// ConversationEntry represents an entry in the conversation.
type ConversationEntry struct {
	Type    string // "user", "assistant", "tool_result"
	Content string
}

// Executor wraps an ADK LLM agent for documentation generation.
type Executor struct {
	llmModel       model.LLM
	modelID        string
	tools          []tool.Tool
	toolsets       []tool.Toolset
	instruction    string
	sessionService session.Service
}

// NewExecutor creates a new ADK-based executor with tools only.
func NewExecutor(ctx context.Context, cfg ExecutorConfig, tools []tool.Tool) (*Executor, error) {
	return NewExecutorWithToolsets(ctx, cfg, tools, nil)
}

// NewExecutorWithToolsets creates a new ADK-based executor with tools and optional toolsets.
func NewExecutorWithToolsets(ctx context.Context, cfg ExecutorConfig, tools []tool.Tool, toolsets []tool.Toolset) (*Executor, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("API key is required")
	}

	modelID := cfg.ModelID
	if modelID == "" {
		modelID = "gemini-2.5-pro"
	}

	// Initialize Phoenix tracing (no-op if PHOENIX_COLLECTOR_ENDPOINT not set)
	if err := tracing.Init(ctx); err != nil {
		logger.Debugf("Failed to initialize Phoenix tracing: %v", err)
	}

	// Create Gemini model
	llmModel, err := gemini.NewModel(ctx, modelID, &genai.ClientConfig{
		APIKey: cfg.APIKey,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini model: %w", err)
	}

	logger.Debugf("Created ADK executor with model: %s", modelID)

	return &Executor{
		llmModel:       llmModel,
		modelID:        modelID,
		tools:          tools,
		toolsets:       toolsets,
		instruction:    cfg.Instruction,
		sessionService: session.InMemoryService(),
	}, nil
}

// ExecuteTask runs the executor to complete a task.
func (e *Executor) ExecuteTask(ctx context.Context, prompt string) (*TaskResult, error) {
	// Start agent span for the entire task
	ctx, agentSpan := tracing.StartAgentSpan(ctx, "executor:execute_task", e.modelID)
	defer agentSpan.End()

	// Record input prompt
	tracing.RecordInput(agentSpan, prompt)

	var conversation []ConversationEntry

	// Add initial prompt to conversation history
	conversation = append(conversation, ConversationEntry{
		Type:    "user",
		Content: prompt,
	})

	// Create the LLM agent
	adkAgent, err := llmagent.New(llmagent.Config{
		Name:        "doc-agent",
		Description: "Documentation generation agent for Elastic packages",
		Model:       e.llmModel,
		Instruction: e.instruction,
		Tools:       e.tools,
		Toolsets:    e.toolsets,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create LLM agent: %w", err)
	}

	// Create runner
	r, err := runner.New(runner.Config{
		AppName:        appName,
		Agent:          adkAgent,
		SessionService: e.sessionService,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create runner: %w", err)
	}

	// Create a new session for this task
	sessionResp, err := e.sessionService.Create(ctx, &session.CreateRequest{
		AppName: appName,
		UserID:  defaultUserID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	// Create user content
	userContent := genai.NewContentFromText(prompt, genai.RoleUser)

	// Run the agent
	var finalContent strings.Builder
	var lastEventContent string
	iterationCount := 0

	// Track input messages for LLM spans
	inputMessages := []tracing.Message{{Role: "user", Content: prompt}}

	for event, err := range r.Run(ctx, defaultUserID, sessionResp.Session.ID(), userContent, agent.RunConfig{}) {
		if err != nil {
			logger.Debugf("Executor iteration error: %v", err)
			return nil, fmt.Errorf("executor execution error: %w", err)
		}

		iterationCount++
		if iterationCount > maxIterations {
			logger.Debugf("Max iterations reached")
			break
		}

		if event == nil {
			continue
		}

		// Extract content from the event
		if event.Content != nil {
			for _, part := range event.Content.Parts {
				if part.Text != "" {
					lastEventContent = part.Text
					finalContent.WriteString(part.Text)

					// Add to conversation
					conversation = append(conversation, ConversationEntry{
						Type:    "assistant",
						Content: part.Text,
					})

					// Create LLM span for this response
					_, llmSpan := tracing.StartLLMSpan(ctx, "llm:response", e.modelID, inputMessages)
					outputMessages := []tracing.Message{{Role: "assistant", Content: part.Text}}
					tracing.EndLLMSpan(llmSpan, outputMessages, 0, 0)
				}

				// Track function calls
				if part.FunctionCall != nil {
					logger.Debugf("Function call: %s", part.FunctionCall.Name)

					// Convert Args to map[string]any for tracing
					var argsMap map[string]any
					if part.FunctionCall.Args != nil {
						argsMap = part.FunctionCall.Args
					}

					// Start tool span (will be ended when we get the response)
					_, toolSpan := tracing.StartToolSpan(ctx, part.FunctionCall.Name, argsMap)
					// End immediately since we don't have async tracking
					// In a real implementation, we'd match up call/response pairs
					toolSpan.End()

					conversation = append(conversation, ConversationEntry{
						Type:    "tool_result",
						Content: fmt.Sprintf("Called: %s", part.FunctionCall.Name),
					})
				}

				// Track function responses
				if part.FunctionResponse != nil {
					logger.Debugf("Function response for: %s", part.FunctionResponse.Name)

					// Create tool response span
					_, toolSpan := tracing.StartToolSpan(ctx, part.FunctionResponse.Name+"_response", nil)

					// Format the response content
					var responseContent string
					if content, exists := part.FunctionResponse.Response["content"]; exists {
						responseContent = fmt.Sprintf("%v", content)
					} else if errContent, exists := part.FunctionResponse.Response["error"]; exists {
						responseContent = fmt.Sprintf("Error: %v", errContent)
					} else {
						// Marshal the entire response
						if respJSON, err := json.Marshal(part.FunctionResponse.Response); err == nil {
							responseContent = string(respJSON)
						}
					}

					tracing.EndToolSpan(toolSpan, responseContent, nil)

					conversation = append(conversation, ConversationEntry{
						Type:    "tool_result",
						Content: fmt.Sprintf("✅ SUCCESS: %s completed.\nResult: %s", part.FunctionResponse.Name, responseContent),
					})
				}
			}
		}

		// Check if this is a final response
		if event.IsFinalResponse() {
			logger.Debugf("Final response received")
			break
		}
	}

	// Use the last meaningful content if available
	resultContent := finalContent.String()
	if resultContent == "" {
		resultContent = lastEventContent
	}

	// Record final output on agent span
	tracing.RecordOutput(agentSpan, strings.TrimSpace(resultContent))

	return &TaskResult{
		Success:      resultContent != "",
		FinalContent: strings.TrimSpace(resultContent),
		Conversation: conversation,
	}, nil
}
