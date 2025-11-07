// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package framework

import (
	"context"
	"fmt"
	"strings"

	"github.com/elastic/elastic-package/internal/llmagent/providers"
	"github.com/elastic/elastic-package/internal/logger"
)

const (
	maxIterations        = 15
	maxRecentToolHistory = 5
)

// Agent represents a generic LLM agent that can use tools
type Agent struct {
	provider providers.LLMProvider
	tools    []providers.Tool
}

// ToolExecutionInfo tracks information about recent tool executions for error analysis
type ToolExecutionInfo struct {
	ToolName   string
	Success    bool
	ResultType string // "success", "error", "failed"
	Result     string
	Iteration  int
}

// TaskResult represents the result of a task execution
type TaskResult struct {
	Success      bool
	FinalContent string
	Conversation []ConversationEntry
}

// ConversationEntry represents an entry in the conversation
type ConversationEntry struct {
	Type    string // "user", "assistant", "tool_result"
	Content string
}

// NewAgent creates a new LLM agent
func NewAgent(provider providers.LLMProvider, tools []providers.Tool) *Agent {
	return &Agent{
		provider: provider,
		tools:    tools,
	}
}

// ExecuteTask runs the agent to complete a task with enhanced error handling
func (a *Agent) ExecuteTask(ctx context.Context, prompt string) (*TaskResult, error) {
	var conversation []ConversationEntry
	var recentTools []ToolExecutionInfo

	// Add initial prompt
	conversation = append(conversation, ConversationEntry{
		Type:    "user",
		Content: prompt,
	})

	for i := 0; i < maxIterations; i++ {
		// Build the full prompt with conversation history
		fullPrompt := a.buildPrompt(conversation)

		logger.Debugf("iterating number %d: we have %d tools\n", i, len(a.tools))
		// Get response from LLM
		response, err := a.provider.GenerateResponse(ctx, fullPrompt, a.tools)
		if err != nil {
			return nil, fmt.Errorf("failed to get LLM response: %w", err)
		}

		// Add LLM response to conversation
		conversation = append(conversation, ConversationEntry{
			Type:    "assistant",
			Content: response.Content,
		})

		// Check for false tool error reports after successful tool executions
		if len(response.ToolCalls) == 0 && a.detectFalseToolError(response.Content, recentTools) {
			// LLM incorrectly thinks tools failed - provide clarification
			clarification := a.buildToolClarificationPrompt(recentTools)
			conversation = append(conversation, ConversationEntry{
				Type:    "user",
				Content: clarification,
			})
			continue
		}

		// If there are tool calls, execute them
		if len(response.ToolCalls) > 0 {
			for _, toolCall := range response.ToolCalls {
				result, err := a.executeTool(ctx, toolCall)
				var toolInfo ToolExecutionInfo

				if err != nil {
					toolResultMsg := a.formatToolError(toolCall.Name, err)
					conversation = append(conversation, ConversationEntry{
						Type:    "tool_result",
						Content: toolResultMsg,
					})
					toolInfo = ToolExecutionInfo{
						ToolName:   toolCall.Name,
						Success:    false,
						ResultType: "failed",
						Result:     err.Error(),
						Iteration:  i,
					}
				} else {
					if result.Error != "" {
						toolResultMsg := a.formatToolError(toolCall.Name, fmt.Errorf("%s", result.Error))
						conversation = append(conversation, ConversationEntry{
							Type:    "tool_result",
							Content: toolResultMsg,
						})
						toolInfo = ToolExecutionInfo{
							ToolName:   toolCall.Name,
							Success:    false,
							ResultType: "error",
							Result:     result.Error,
							Iteration:  i,
						}
					} else {
						toolResultMsg := a.formatToolSuccess(toolCall.Name, result.Content)
						conversation = append(conversation, ConversationEntry{
							Type:    "tool_result",
							Content: toolResultMsg,
						})
						toolInfo = ToolExecutionInfo{
							ToolName:   toolCall.Name,
							Success:    true,
							ResultType: "success",
							Result:     result.Content,
							Iteration:  i,
						}
					}
				}

				// Track recent tool executions
				recentTools = append(recentTools, toolInfo)
				if len(recentTools) > maxRecentToolHistory {
					recentTools = recentTools[1:]
				}
			}
		} else if response.Finished {
			// No tool calls and LLM indicated it's finished
			return &TaskResult{
				Success:      true,
				FinalContent: response.Content,
				Conversation: conversation,
			}, nil
		} else {
			// No tool calls and not finished - this can happen with unstable models
			// Add a prompt to encourage the LLM to complete the task or use tools
			conversation = append(conversation, ConversationEntry{
				Type:    "user",
				Content: "Please complete the task or use the available tools to gather the information you need. If the task is complete, please indicate that you are finished.",
			})
		}
	}

	return &TaskResult{
		Success:      false,
		FinalContent: "Task did not complete within maximum iterations",
		Conversation: conversation,
	}, nil
}

// executeTool executes a specific tool call
func (a *Agent) executeTool(ctx context.Context, toolCall providers.ToolCall) (*providers.ToolResult, error) {
	// Find the tool
	for _, tool := range a.tools {
		if tool.Name == toolCall.Name {
			return tool.Handler(ctx, toolCall.Arguments)
		}
	}

	return nil, fmt.Errorf("tool not found: %s", toolCall.Name)
}

// detectFalseToolError determines if LLM incorrectly thinks tools failed after they succeeded
func (a *Agent) detectFalseToolError(content string, recentTools []ToolExecutionInfo) bool {
	if len(recentTools) == 0 {
		return false
	}

	// Check if LLM reports an error after recent successful tool executions
	errorIndicators := []string{
		"I encountered an error",
		"I'm experiencing an error",
		"error while trying to call",
		"function call failed",
		"tool call failed",
		"I'm having trouble",
		"something went wrong",
	}

	contentLower := strings.ToLower(content)
	hasErrorIndicator := false
	for _, indicator := range errorIndicators {
		if strings.Contains(contentLower, strings.ToLower(indicator)) {
			hasErrorIndicator = true
			break
		}
	}

	if !hasErrorIndicator {
		return false
	}

	// Check if we have recent successful tool executions
	for i := len(recentTools) - 1; i >= 0; i-- {
		tool := recentTools[i]
		// If the most recent tools were successful, this is likely a false error
		if tool.Success && tool.ResultType == "success" {
			return true
		}
		// If we hit an actual error, stop checking
		if !tool.Success {
			break
		}
	}

	return false
}

// buildToolClarificationPrompt creates a clarifying prompt when LLM incorrectly reports tool errors
func (a *Agent) buildToolClarificationPrompt(recentTools []ToolExecutionInfo) string {
	var builder strings.Builder

	builder.WriteString("IMPORTANT CLARIFICATION: You mentioned encountering an error, but please review the recent tool execution results:\n\n")

	// Show recent tool results
	for i := len(recentTools) - 1; i >= 0 && i >= len(recentTools)-3; i-- {
		tool := recentTools[i]
		if tool.Success {
			builder.WriteString(fmt.Sprintf("✅ %s: SUCCEEDED - %s\n", tool.ToolName, tool.Result))
		} else {
			builder.WriteString(fmt.Sprintf("❌ %s: FAILED - %s\n", tool.ToolName, tool.Result))
		}
	}

	builder.WriteString("\nGuidance for interpreting tool results:\n")
	builder.WriteString("- Messages starting with 'Successfully' indicate success\n")
	builder.WriteString("- Messages containing 'bytes written', 'file created', or similar indicate success\n")
	builder.WriteString("- Only messages explicitly stating 'error', 'failed', or 'denied' indicate actual failures\n\n")
	builder.WriteString("Please continue with your task based on the ACTUAL tool results shown above, not any perceived errors.")

	return builder.String()
}

// formatToolSuccess formats successful tool results in a clear, LLM-friendly way
func (a *Agent) formatToolSuccess(toolName, result string) string {
	return fmt.Sprintf("✅ SUCCESS: %s completed successfully.\nResult: %s", toolName, result)
}

// formatToolError formats tool errors in a clear, LLM-friendly way
func (a *Agent) formatToolError(toolName string, err error) string {
	return fmt.Sprintf("❌ ERROR: %s failed.\nError: %s", toolName, err.Error())
}

// buildPrompt creates the full prompt with conversation history
func (a *Agent) buildPrompt(conversation []ConversationEntry) string {
	var builder strings.Builder

	for _, entry := range conversation {
		switch entry.Type {
		case "user":
			builder.WriteString("Human: ")
			builder.WriteString(entry.Content)
			builder.WriteString("\n\n")
		case "assistant":
			builder.WriteString("Assistant: ")
			builder.WriteString(entry.Content)
			builder.WriteString("\n\n")
		case "tool_result":
			builder.WriteString("Tool Result: ")
			builder.WriteString(entry.Content)
			builder.WriteString("\n\n")
		}
	}

	return builder.String()
}
