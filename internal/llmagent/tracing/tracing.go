// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

// Package tracing provides OpenTelemetry-based tracing for LLM conversations
// and tool calls, compatible with Arize Phoenix via OpenInference semantic conventions.
package tracing

import (
	"context"
	"encoding/json"
	"os"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
	"go.opentelemetry.io/otel/trace"
)

// Environment variable names for Phoenix configuration
const (
	EnvPhoenixEndpoint    = "PHOENIX_COLLECTOR_ENDPOINT"
	EnvPhoenixAPIKey      = "PHOENIX_API_KEY"
	EnvPhoenixProjectName = "PHOENIX_PROJECT_NAME"
)

// Default values
const (
	DefaultEndpoint    = "http://localhost:6006/v1/traces"
	DefaultProjectName = "elastic-package"
	TracerName         = "elastic-package-llmagent"
)

// OpenInference semantic convention attribute keys
// See: https://github.com/Arize-ai/openinference/blob/main/spec/semantic_conventions.md
const (
	// Span kind attributes
	AttrOpenInferenceSpanKind = "openinference.span.kind"

	// LLM attributes
	AttrGenAISystem        = "gen_ai.system"
	AttrGenAIRequestModel  = "gen_ai.request.model"
	AttrGenAIResponseModel = "gen_ai.response.model"

	// Input/Output attributes
	AttrInputValue  = "input.value"
	AttrOutputValue = "output.value"

	// Message attributes (JSON arrays)
	AttrLLMInputMessages  = "llm.input_messages"
	AttrLLMOutputMessages = "llm.output_messages"

	// Token usage attributes
	AttrLLMTokenCountPrompt     = "llm.token_count.prompt"
	AttrLLMTokenCountCompletion = "llm.token_count.completion"
	AttrLLMTokenCountTotal      = "llm.token_count.total"

	// Tool attributes
	AttrToolName       = "tool.name"
	AttrToolParameters = "tool.parameters"
	AttrToolOutput     = "tool.output"
)

// SpanKind values for OpenInference
const (
	SpanKindLLM   = "LLM"
	SpanKindTool  = "TOOL"
	SpanKindChain = "CHAIN"
	SpanKindAgent = "AGENT"
)

var (
	globalTracer     trace.Tracer
	globalProvider   *sdktrace.TracerProvider
	initOnce         sync.Once
	shutdownOnce     sync.Once
	tracingEnabled   bool
	tracingInitError error
)

// Config holds Phoenix tracing configuration
type Config struct {
	Endpoint    string
	APIKey      string
	ProjectName string
}

// ConfigFromEnv creates a Config from environment variables
func ConfigFromEnv() Config {
	cfg := Config{
		Endpoint:    os.Getenv(EnvPhoenixEndpoint),
		APIKey:      os.Getenv(EnvPhoenixAPIKey),
		ProjectName: os.Getenv(EnvPhoenixProjectName),
	}

	if cfg.Endpoint == "" {
		cfg.Endpoint = DefaultEndpoint
	}
	if cfg.ProjectName == "" {
		cfg.ProjectName = DefaultProjectName
	}

	return cfg
}

// Init initializes the OpenTelemetry tracer with OTLP exporter for Phoenix.
// It reads configuration from environment variables.
// This function is safe to call multiple times; subsequent calls are no-ops.
func Init(ctx context.Context) error {
	initOnce.Do(func() {
		cfg := ConfigFromEnv()

		// Only enable tracing if endpoint is explicitly set
		if os.Getenv(EnvPhoenixEndpoint) == "" {
			tracingEnabled = false
			globalTracer = otel.Tracer(TracerName)
			return
		}

		tracingEnabled = true
		tracingInitError = initTracer(ctx, cfg)
	})

	return tracingInitError
}

func initTracer(ctx context.Context, cfg Config) error {
	// Configure OTLP exporter options
	opts := []otlptracehttp.Option{
		otlptracehttp.WithEndpointURL(cfg.Endpoint),
	}

	// Add API key header if provided
	if cfg.APIKey != "" {
		opts = append(opts, otlptracehttp.WithHeaders(map[string]string{
			"api_key": cfg.APIKey,
		}))
	}

	// Create OTLP exporter
	exporter, err := otlptracehttp.New(ctx, opts...)
	if err != nil {
		return err
	}

	// Create resource with service info
	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(cfg.ProjectName),
			attribute.String("phoenix.project.name", cfg.ProjectName),
		),
	)
	if err != nil {
		return err
	}

	// Create tracer provider
	globalProvider = sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)

	// Set as global provider
	otel.SetTracerProvider(globalProvider)
	globalTracer = globalProvider.Tracer(TracerName)

	return nil
}

// Shutdown flushes and shuts down the tracer provider.
// It should be called when the application exits.
func Shutdown(ctx context.Context) error {
	var err error
	shutdownOnce.Do(func() {
		if globalProvider != nil {
			err = globalProvider.Shutdown(ctx)
		}
	})
	return err
}

// IsEnabled returns true if tracing is enabled
func IsEnabled() bool {
	return tracingEnabled
}

// Tracer returns the global tracer instance
func Tracer() trace.Tracer {
	if globalTracer == nil {
		return otel.Tracer(TracerName)
	}
	return globalTracer
}

// Message represents a chat message for tracing
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// StartAgentSpan starts a new span for an agent task execution
func StartAgentSpan(ctx context.Context, name string, modelID string) (context.Context, trace.Span) {
	return Tracer().Start(ctx, name,
		trace.WithAttributes(
			attribute.String(AttrOpenInferenceSpanKind, SpanKindAgent),
			attribute.String(AttrGenAISystem, "gemini"),
			attribute.String(AttrGenAIRequestModel, modelID),
		),
	)
}

// StartLLMSpan starts a new span for an LLM call
func StartLLMSpan(ctx context.Context, name string, modelID string, inputMessages []Message) (context.Context, trace.Span) {
	attrs := []attribute.KeyValue{
		attribute.String(AttrOpenInferenceSpanKind, SpanKindLLM),
		attribute.String(AttrGenAISystem, "gemini"),
		attribute.String(AttrGenAIRequestModel, modelID),
	}

	if len(inputMessages) > 0 {
		if msgJSON, err := json.Marshal(inputMessages); err == nil {
			attrs = append(attrs, attribute.String(AttrLLMInputMessages, string(msgJSON)))
		}
	}

	return Tracer().Start(ctx, name, trace.WithAttributes(attrs...))
}

// EndLLMSpan records the LLM response and ends the span
func EndLLMSpan(span trace.Span, outputMessages []Message, promptTokens, completionTokens int) {
	if len(outputMessages) > 0 {
		if msgJSON, err := json.Marshal(outputMessages); err == nil {
			span.SetAttributes(attribute.String(AttrLLMOutputMessages, string(msgJSON)))
		}
		// Also set output.value to the last message content for simpler viewing
		if len(outputMessages) > 0 {
			span.SetAttributes(attribute.String(AttrOutputValue, outputMessages[len(outputMessages)-1].Content))
		}
	}

	if promptTokens > 0 {
		span.SetAttributes(attribute.Int(AttrLLMTokenCountPrompt, promptTokens))
	}
	if completionTokens > 0 {
		span.SetAttributes(attribute.Int(AttrLLMTokenCountCompletion, completionTokens))
	}
	if promptTokens > 0 || completionTokens > 0 {
		span.SetAttributes(attribute.Int(AttrLLMTokenCountTotal, promptTokens+completionTokens))
	}

	span.End()
}

// StartToolSpan starts a new span for a tool call
func StartToolSpan(ctx context.Context, toolName string, parameters map[string]any) (context.Context, trace.Span) {
	attrs := []attribute.KeyValue{
		attribute.String(AttrOpenInferenceSpanKind, SpanKindTool),
		attribute.String(AttrToolName, toolName),
	}

	if parameters != nil {
		if paramJSON, err := json.Marshal(parameters); err == nil {
			attrs = append(attrs, attribute.String(AttrToolParameters, string(paramJSON)))
		}
	}

	return Tracer().Start(ctx, "tool:"+toolName, trace.WithAttributes(attrs...))
}

// EndToolSpan records the tool output and ends the span
func EndToolSpan(span trace.Span, output string, err error) {
	if output != "" {
		span.SetAttributes(attribute.String(AttrToolOutput, output))
		span.SetAttributes(attribute.String(AttrOutputValue, output))
	}
	if err != nil {
		span.RecordError(err)
	}
	span.End()
}

// RecordInput records the input value on a span
func RecordInput(span trace.Span, input string) {
	span.SetAttributes(attribute.String(AttrInputValue, input))
}

// RecordOutput records the output value on a span
func RecordOutput(span trace.Span, output string) {
	span.SetAttributes(attribute.String(AttrOutputValue, output))
}
