// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

// Package agent provides an ADK-based LLM agent wrapper for elastic-package.
package agent

import (
	"context"
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

	"github.com/elastic/elastic-package/internal/logger"
)

const (
	maxIterations = 15
	appName       = "elastic-package"
	defaultUserID = "default-user"
)

// Config holds configuration for creating an Agent.
type Config struct {
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

// Agent wraps an ADK LLM agent for documentation generation.
type Agent struct {
	llmModel       model.LLM
	tools          []tool.Tool
	toolsets       []tool.Toolset
	instruction    string
	sessionService session.Service
}

// NewAgent creates a new ADK-based agent with tools only.
func NewAgent(ctx context.Context, cfg Config, tools []tool.Tool) (*Agent, error) {
	return NewAgentWithToolsets(ctx, cfg, tools, nil)
}

// NewAgentWithToolsets creates a new ADK-based agent with tools and optional toolsets.
func NewAgentWithToolsets(ctx context.Context, cfg Config, tools []tool.Tool, toolsets []tool.Toolset) (*Agent, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("API key is required")
	}

	modelID := cfg.ModelID
	if modelID == "" {
		modelID = "gemini-2.5-pro"
	}

	// Create Gemini model
	llmModel, err := gemini.NewModel(ctx, modelID, &genai.ClientConfig{
		APIKey: cfg.APIKey,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini model: %w", err)
	}

	logger.Debugf("Created ADK agent with model: %s", modelID)

	return &Agent{
		llmModel:       llmModel,
		tools:          tools,
		toolsets:       toolsets,
		instruction:    cfg.Instruction,
		sessionService: session.InMemoryService(),
	}, nil
}

// ExecuteTask runs the agent to complete a task.
func (a *Agent) ExecuteTask(ctx context.Context, prompt string) (*TaskResult, error) {
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
		Model:       a.llmModel,
		Instruction: a.instruction,
		Tools:       a.tools,
		Toolsets:    a.toolsets,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create LLM agent: %w", err)
	}

	// Create runner
	r, err := runner.New(runner.Config{
		AppName:        appName,
		Agent:          adkAgent,
		SessionService: a.sessionService,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create runner: %w", err)
	}

	// Create a new session for this task
	sessionResp, err := a.sessionService.Create(ctx, &session.CreateRequest{
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

	for event, err := range r.Run(ctx, defaultUserID, sessionResp.Session.ID(), userContent, agent.RunConfig{}) {
		if err != nil {
			logger.Debugf("Agent iteration error: %v", err)
			return nil, fmt.Errorf("agent execution error: %w", err)
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
				}

				// Track function calls
				if part.FunctionCall != nil {
					logger.Debugf("Function call: %s", part.FunctionCall.Name)
					conversation = append(conversation, ConversationEntry{
						Type:    "tool_result",
						Content: fmt.Sprintf("Called: %s", part.FunctionCall.Name),
					})
				}

				// Track function responses
				if part.FunctionResponse != nil {
					logger.Debugf("Function response for: %s", part.FunctionResponse.Name)
					// Format the response content
					var responseContent string
					if content, exists := part.FunctionResponse.Response["content"]; exists {
						responseContent = fmt.Sprintf("%v", content)
					} else if errContent, exists := part.FunctionResponse.Response["error"]; exists {
						responseContent = fmt.Sprintf("Error: %v", errContent)
					}
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

	return &TaskResult{
		Success:      resultContent != "",
		FinalContent: strings.TrimSpace(resultContent),
		Conversation: conversation,
	}, nil
}

