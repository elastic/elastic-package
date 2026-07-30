// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package tracing

import (
	"context"
	"encoding/json"
	"fmt"

	"go.opentelemetry.io/otel/trace"
)

type pendingToolSpan struct {
	id   string
	name string
	span trace.Span
}

// ToolSpanTracker pairs tool call and response events so the resulting span
// covers the tool's execution rather than only response processing.
type ToolSpanTracker struct {
	ctx     context.Context
	pending []pendingToolSpan
}

// NewToolSpanTracker creates a tracker whose tool spans inherit from ctx.
func NewToolSpanTracker(ctx context.Context) *ToolSpanTracker {
	return &ToolSpanTracker{ctx: ctx}
}

// Start records a tool call and keeps its span open until the matching response.
func (t *ToolSpanTracker) Start(id, name string, parameters map[string]any) {
	_, span := StartToolSpan(t.ctx, name, parameters)
	t.pending = append(t.pending, pendingToolSpan{
		id:   id,
		name: name,
		span: span,
	})
}

// End records a tool response, closes the matching span, and returns the
// formatted response and any tool-reported error.
func (t *ToolSpanTracker) End(id, name string, response map[string]any) (string, error) {
	span, ok := t.take(id, name)
	if !ok {
		_, span = StartToolSpan(t.ctx, name, nil)
	}

	output, err := formatToolResponse(name, response)
	EndToolSpan(span, output, err)
	return output, err
}

// EndPending marks calls without matching responses as failed.
func (t *ToolSpanTracker) EndPending(err error) {
	if len(t.pending) == 0 {
		return
	}
	if err == nil {
		err = fmt.Errorf("tool call ended without a response")
	}
	pending := t.pending
	t.pending = nil
	for _, call := range pending {
		EndToolSpan(call.span, "", err)
	}
}

func (t *ToolSpanTracker) take(id, name string) (trace.Span, bool) {
	if id != "" {
		for i, call := range t.pending {
			if call.id == id {
				return t.remove(i), true
			}
		}
	}
	for i, call := range t.pending {
		if call.name == name {
			return t.remove(i), true
		}
	}
	return nil, false
}

func (t *ToolSpanTracker) remove(index int) trace.Span {
	span := t.pending[index].span
	t.pending = append(t.pending[:index], t.pending[index+1:]...)
	return span
}

func formatToolResponse(name string, response map[string]any) (string, error) {
	if errContent, exists := response["error"]; exists {
		return fmt.Sprintf("Error: %v", errContent), fmt.Errorf("tool %s failed: %v", name, errContent)
	}
	if content, exists := response["content"]; exists {
		return fmt.Sprintf("%v", content), nil
	}
	if output, exists := response["output"]; exists {
		return fmt.Sprintf("%v", output), nil
	}
	respJSON, err := json.Marshal(response)
	if err != nil {
		return fmt.Sprintf("%v", response), fmt.Errorf("encoding response from tool %s: %w", name, err)
	}
	return string(respJSON), nil
}
