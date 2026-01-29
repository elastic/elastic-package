// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package executor

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTaskResult(t *testing.T) {
	result := TaskResult{
		Success:      true,
		FinalContent: "Test content",
		Conversation: []ConversationEntry{
			{Type: "user", Content: "Hello"},
			{Type: "assistant", Content: "Hi there"},
		},
	}

	assert.True(t, result.Success)
	assert.Equal(t, "Test content", result.FinalContent)
	assert.Len(t, result.Conversation, 2)
	assert.Equal(t, "user", result.Conversation[0].Type)
	assert.Equal(t, "assistant", result.Conversation[1].Type)
}

func TestConversationEntry(t *testing.T) {
	entries := []ConversationEntry{
		{Type: "user", Content: "User message"},
		{Type: "assistant", Content: "Assistant response"},
		{Type: "tool_result", Content: "Tool output"},
	}

	assert.Len(t, entries, 3)
	assert.Equal(t, "user", entries[0].Type)
	assert.Equal(t, "assistant", entries[1].Type)
	assert.Equal(t, "tool_result", entries[2].Type)
}
