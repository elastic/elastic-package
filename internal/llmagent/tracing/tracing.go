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

	"github.com/google/uuid"
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
	EnvPhoenixEnabled     = "LLM_TRACING_ENABLED"
	EnvPhoenixEndpoint    = "LLM_TRACING_ENDPOINT"
	EnvPhoenixAPIKey      = "LLM_TRACING_API_KEY"
	EnvPhoenixProjectName = "LLM_TRACING_PROJECT_NAME"
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

	// Session attributes
	AttrSessionID = "session.id"

	// LLM attributes
	AttrGenAISystem        = "gen_ai.system"
	AttrGenAIRequestModel  = "gen_ai.request.model"
	AttrGenAIResponseModel = "gen_ai.response.model"
	AttrLLMModelName       = "llm.model_name" // Phoenix uses this for cost calculation

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
	SpanKindLLM      = "LLM"
	SpanKindTool     = "TOOL"
	SpanKindChain    = "CHAIN"
	SpanKindAgent    = "AGENT"
	SpanKindWorkflow = "CHAIN" // Workflows are represented as chains in OpenInference
)

// Workflow-specific attribute keys
const (
	AttrWorkflowName      = "workflow.name"
	AttrWorkflowIteration = "workflow.iteration"
	AttrWorkflowMaxIter   = "workflow.max_iterations"
	AttrSubAgentName      = "sub_agent.name"
	AttrSubAgentRole      = "sub_agent.role"
)

// Context keys for session tracking
type sessionIDKey struct{}
type sessionTokensKey struct{}

// SessionTokens tracks cumulative token usage for a session
type SessionTokens struct {
	mu               sync.Mutex
	PromptTokens     int
	CompletionTokens int
}

// Add adds token counts to the session totals
func (st *SessionTokens) Add(prompt, completion int) {
	st.mu.Lock()
	defer st.mu.Unlock()
	st.PromptTokens += prompt
	st.CompletionTokens += completion
}

// Total returns the total token count
func (st *SessionTokens) Total() int {
	st.mu.Lock()
	defer st.mu.Unlock()
	return st.PromptTokens + st.CompletionTokens
}

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
	Enabled     bool
	Endpoint    string
	APIKey      string
	ProjectName string
}

// ConfigFromEnv creates a Config from environment variables
func ConfigFromEnv() Config {
	cfg := Config{
		Enabled:     true, // Enabled by default
		Endpoint:    os.Getenv(EnvPhoenixEndpoint),
		APIKey:      os.Getenv(EnvPhoenixAPIKey),
		ProjectName: os.Getenv(EnvPhoenixProjectName),
	}

	// Check if explicitly disabled
	if enabledStr := os.Getenv(EnvPhoenixEnabled); enabledStr != "" {
		cfg.Enabled = enabledStr == "true" || enabledStr == "1"
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
	return InitWithConfig(ctx, ConfigFromEnv())
}

// InitWithConfig initializes the OpenTelemetry tracer with the provided configuration.
// This function is safe to call multiple times; subsequent calls are no-ops.
func InitWithConfig(ctx context.Context, cfg Config) error {
	initOnce.Do(func() {
		if !cfg.Enabled {
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

	// Build headers map - Phoenix requires project name as header
	headers := map[string]string{
		"phoenix-project-name": cfg.ProjectName,
	}
	if cfg.APIKey != "" {
		headers["api_key"] = cfg.APIKey
	}
	opts = append(opts, otlptracehttp.WithHeaders(headers))

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
			attribute.String("project.name", cfg.ProjectName),
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

// WithSessionID returns a new context with the given session ID stored.
// Child spans created from this context will inherit the session ID.
func WithSessionID(ctx context.Context, sessionID string) context.Context {
	return context.WithValue(ctx, sessionIDKey{}, sessionID)
}

// withSessionTokens returns a new context with session token tracking.
func withSessionTokens(ctx context.Context, tokens *SessionTokens) context.Context {
	return context.WithValue(ctx, sessionTokensKey{}, tokens)
}

// SessionIDFromContext retrieves the session ID from the context, if present.
func SessionIDFromContext(ctx context.Context) (string, bool) {
	sessionID, ok := ctx.Value(sessionIDKey{}).(string)
	return sessionID, ok
}

// SessionTokensFromContext retrieves the session token tracker from context.
func SessionTokensFromContext(ctx context.Context) *SessionTokens {
	tokens, _ := ctx.Value(sessionTokensKey{}).(*SessionTokens)
	return tokens
}

// sessionAttributes returns session ID attribute if present in context
func sessionAttributes(ctx context.Context) []attribute.KeyValue {
	if sessionID, ok := SessionIDFromContext(ctx); ok {
		return []attribute.KeyValue{attribute.String(AttrSessionID, sessionID)}
	}
	return nil
}

// StartSessionSpan starts a root span for an entire session/conversation.
// It generates a unique session ID and stores it in the context for propagation to child spans.
// It also initializes token tracking for the session.
func StartSessionSpan(ctx context.Context, sessionName string, modelID string) (context.Context, trace.Span) {
	sessionID := uuid.New().String()

	// Store session ID and token tracker in context for child spans
	ctx = WithSessionID(ctx, sessionID)
	ctx = withSessionTokens(ctx, &SessionTokens{})

	ctx, span := Tracer().Start(ctx, sessionName,
		trace.WithAttributes(
			attribute.String(AttrOpenInferenceSpanKind, SpanKindChain),
			attribute.String(AttrSessionID, sessionID),
			attribute.String(AttrLLMModelName, modelID),
			attribute.String(AttrGenAIRequestModel, modelID),
		),
		trace.WithSpanKind(trace.SpanKindServer),
	)
	return ctx, span
}

// EndSessionSpan records the final session output and token counts, then ends the span.
func EndSessionSpan(ctx context.Context, span trace.Span, output string) {
	// Record output
	if output != "" {
		span.SetAttributes(attribute.String(AttrOutputValue, output))
	}

	// Record cumulative token counts from session
	if tokens := SessionTokensFromContext(ctx); tokens != nil {
		tokens.mu.Lock()
		span.SetAttributes(
			attribute.Int(AttrLLMTokenCountPrompt, tokens.PromptTokens),
			attribute.Int(AttrLLMTokenCountCompletion, tokens.CompletionTokens),
			attribute.Int(AttrLLMTokenCountTotal, tokens.PromptTokens+tokens.CompletionTokens),
		)
		tokens.mu.Unlock()
	}

	span.End()
}

// RecordSessionInput records the input value on a session span.
func RecordSessionInput(span trace.Span, input string) {
	span.SetAttributes(attribute.String(AttrInputValue, input))
}

// StartChainSpan starts a new span for a chain of operations (e.g., document generation)
func StartChainSpan(ctx context.Context, name string) (context.Context, trace.Span) {
	attrs := []attribute.KeyValue{
		attribute.String(AttrOpenInferenceSpanKind, SpanKindChain),
	}
	attrs = append(attrs, sessionAttributes(ctx)...)
	return Tracer().Start(ctx, name, trace.WithAttributes(attrs...))
}

// StartAgentSpan starts a new span for an agent task execution
func StartAgentSpan(ctx context.Context, name string, modelID string) (context.Context, trace.Span) {
	attrs := []attribute.KeyValue{
		attribute.String(AttrOpenInferenceSpanKind, SpanKindAgent),
		attribute.String(AttrGenAISystem, "gemini"),
		attribute.String(AttrGenAIRequestModel, modelID),
		attribute.String(AttrLLMModelName, modelID), // Phoenix uses this for cost calculation
	}
	attrs = append(attrs, sessionAttributes(ctx)...)
	return Tracer().Start(ctx, name, trace.WithAttributes(attrs...))
}

// StartLLMSpan starts a new span for an LLM call
func StartLLMSpan(ctx context.Context, name string, modelID string, inputMessages []Message) (context.Context, trace.Span) {
	attrs := []attribute.KeyValue{
		attribute.String(AttrOpenInferenceSpanKind, SpanKindLLM),
		attribute.String(AttrGenAISystem, "gemini"),
		attribute.String(AttrGenAIRequestModel, modelID),
		attribute.String(AttrLLMModelName, modelID), // Phoenix uses this for cost calculation
	}
	attrs = append(attrs, sessionAttributes(ctx)...)

	if len(inputMessages) > 0 {
		if msgJSON, err := json.Marshal(inputMessages); err == nil {
			attrs = append(attrs, attribute.String(AttrLLMInputMessages, string(msgJSON)))
		}
	}

	return Tracer().Start(ctx, name, trace.WithAttributes(attrs...))
}

// EndLLMSpan records the LLM response and ends the span.
// It also accumulates token counts to the session tracker if present in context.
func EndLLMSpan(ctx context.Context, span trace.Span, outputMessages []Message, promptTokens, completionTokens int) {
	if len(outputMessages) > 0 {
		if msgJSON, err := json.Marshal(outputMessages); err == nil {
			span.SetAttributes(attribute.String(AttrLLMOutputMessages, string(msgJSON)))
		}
		// Also set output.value to the last message content for simpler viewing
		span.SetAttributes(attribute.String(AttrOutputValue, outputMessages[len(outputMessages)-1].Content))
	}

	// Always set token counts (even if 0) for Phoenix cost calculation
	span.SetAttributes(
		attribute.Int(AttrLLMTokenCountPrompt, promptTokens),
		attribute.Int(AttrLLMTokenCountCompletion, completionTokens),
		attribute.Int(AttrLLMTokenCountTotal, promptTokens+completionTokens),
	)

	// Accumulate tokens to session tracker
	if tokens := SessionTokensFromContext(ctx); tokens != nil {
		tokens.Add(promptTokens, completionTokens)
	}

	span.End()
}

// StartToolSpan starts a new span for a tool call
func StartToolSpan(ctx context.Context, toolName string, parameters map[string]any) (context.Context, trace.Span) {
	attrs := []attribute.KeyValue{
		attribute.String(AttrOpenInferenceSpanKind, SpanKindTool),
		attribute.String(AttrToolName, toolName),
	}
	attrs = append(attrs, sessionAttributes(ctx)...)

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

// StartWorkflowSpan starts a new span for a multi-agent workflow execution
func StartWorkflowSpan(ctx context.Context, name string) (context.Context, trace.Span) {
	attrs := []attribute.KeyValue{
		attribute.String(AttrOpenInferenceSpanKind, SpanKindWorkflow),
		attribute.String(AttrWorkflowName, name),
	}
	attrs = append(attrs, sessionAttributes(ctx)...)
	return Tracer().Start(ctx, name, trace.WithAttributes(attrs...))
}

// StartWorkflowSpanWithConfig starts a workflow span with iteration configuration
func StartWorkflowSpanWithConfig(ctx context.Context, name string, maxIterations uint) (context.Context, trace.Span) {
	attrs := []attribute.KeyValue{
		attribute.String(AttrOpenInferenceSpanKind, SpanKindWorkflow),
		attribute.String(AttrWorkflowName, name),
		attribute.Int(AttrWorkflowMaxIter, int(maxIterations)),
	}
	attrs = append(attrs, sessionAttributes(ctx)...)
	return Tracer().Start(ctx, name, trace.WithAttributes(attrs...))
}

// StartSubAgentSpan starts a new span for a sub-agent within a workflow
func StartSubAgentSpan(ctx context.Context, agentName string, role string) (context.Context, trace.Span) {
	attrs := []attribute.KeyValue{
		attribute.String(AttrOpenInferenceSpanKind, SpanKindAgent),
		attribute.String(AttrSubAgentName, agentName),
		attribute.String(AttrSubAgentRole, role),
	}
	attrs = append(attrs, sessionAttributes(ctx)...)
	return Tracer().Start(ctx, "subagent:"+agentName, trace.WithAttributes(attrs...))
}

// RecordWorkflowIteration records the current iteration number on a workflow span
func RecordWorkflowIteration(span trace.Span, iteration int) {
	span.SetAttributes(attribute.Int(AttrWorkflowIteration, iteration))
}

// RecordWorkflowResult records the final workflow result on a span
func RecordWorkflowResult(span trace.Span, approved bool, iterations int, content string) {
	span.SetAttributes(
		attribute.Bool("workflow.approved", approved),
		attribute.Int("workflow.total_iterations", iterations),
	)
	if content != "" {
		// Truncate content if too long for tracing
		if len(content) > 1000 {
			content = content[:1000] + "..."
		}
		span.SetAttributes(attribute.String(AttrOutputValue, content))
	}
}
