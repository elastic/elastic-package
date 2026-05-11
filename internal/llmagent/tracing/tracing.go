// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

// Package tracing provides no-op stubs for LLM tracing.
// The real implementation (OpenTelemetry + Phoenix) can be swapped in later.
package tracing

import (
	"context"

	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

const (
	DefaultEndpoint    = "http://localhost:6006/v1/traces"
	DefaultProjectName = "elastic-package"
	TracerName         = "elastic-package-llmagent"
	DefaultModelID     = "gemini-3-flash-preview"
)

// Config holds LLM tracing configuration.
type Config struct {
	Enabled     bool
	Endpoint    string
	APIKey      string
	ProjectName string
}

// Message represents a chat message for tracing.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

var noopTracer = noop.NewTracerProvider().Tracer(TracerName)

func InitWithConfig(_ context.Context, _ Config) error { return nil }
func Shutdown(_ context.Context) error                 { return nil }
func ForceFlush(_ context.Context) error               { return nil }
func IsEnabled() bool                                  { return false }

func SessionIDFromContext(_ context.Context) (string, bool) { return "", false }

func StartSessionSpan(ctx context.Context, _ string, _ string, _ string) (context.Context, trace.Span) {
	return noopTracer.Start(ctx, "noop")
}

func EndSessionSpan(_ context.Context, span trace.Span, _ string) { span.End() }

func RecordSessionInput(_ trace.Span, _ string) {}

func StartChainSpan(ctx context.Context, _ string) (context.Context, trace.Span) {
	return noopTracer.Start(ctx, "noop")
}

func EndChainSpan(_ context.Context, span trace.Span) { span.End() }

func StartAgentSpan(ctx context.Context, _ string, _ string, _ string) (context.Context, trace.Span) {
	return noopTracer.Start(ctx, "noop")
}

func SetSpanOk(_ trace.Span)              {}
func SetSpanError(_ trace.Span, _ error)  {}
func RecordInput(_ trace.Span, _ string)  {}
func RecordOutput(_ trace.Span, _ string) {}

func StartLLMSpan(ctx context.Context, _ string, _ string, _ string, _ []Message) (context.Context, trace.Span) {
	return noopTracer.Start(ctx, "noop")
}

func EndLLMSpan(_ context.Context, span trace.Span, _ []Message, _, _ int) { span.End() }

func StartToolSpan(ctx context.Context, _ string, _ map[string]any) (context.Context, trace.Span) {
	return noopTracer.Start(ctx, "noop")
}

func EndToolSpan(span trace.Span, _ string, _ error) { span.End() }

func StartWorkflowSpanWithConfig(ctx context.Context, _ string, _ uint) (context.Context, trace.Span) {
	return noopTracer.Start(ctx, "noop")
}

func RecordWorkflowResult(_ trace.Span, _ bool, _ int, _ string) {}
