// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

// Package tracing provides OpenTelemetry-based tracing for LLM conversations
// and tool calls, using OpenInference semantic conventions.
package tracing

import (
	"context"
	"encoding/json"
	"strings"
	"sync"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
	"go.opentelemetry.io/otel/trace"

	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/version"
)

// Default values
const (
	DefaultEndpoint    = "http://localhost:6006/v1/traces"
	DefaultProjectName = "elastic-package"
	TracerName         = "elastic-package-llmagent"
	// DefaultModelID is used when no model ID is provided to ensure Phoenix can calculate costs
	DefaultModelID = "gemini-3-flash-preview"
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
	AttrLLMModelName       = "llm.model_name" // Used for cost calculation in tracing UIs
	AttrLLMProvider        = "llm.provider"   // Used for cost calculation in tracing UIs

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

	// Gen AI tool attributes (OpenInference)
	AttrGenAIOperationName = "gen_ai.operation.name"
	AttrGenAIToolType      = "gen_ai.tool.type"
	AttrGenAIToolName      = "gen_ai.tool.name"
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

	// Section context attributes (for visualization)
	AttrSectionTitle = "section.title"
	AttrSectionLevel = "section.level"

	// Span identification (for tree building in visualizers)
	AttrSpanID = "span.id"

	// Build info attributes (for version tracking)
	AttrBuildCommit  = "build.commit"
	AttrBuildTime    = "build.time"
	AttrBuildVersion = "build.version"
)

// Context keys for session tracking
type (
	sessionIDKey     struct{}
	sessionTokensKey struct{}
)

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
	globalTracer        trace.Tracer
	globalProvider      *sdktrace.TracerProvider
	initOnce            sync.Once
	shutdownOnce        sync.Once
	tracingEnabled      bool
	tracingInitError    error
	currentSessionID    string
	currentSessionMutex sync.RWMutex
)

// suppressingErrorHandler logs trace export errors at debug level, and only once.
type suppressingErrorHandler struct {
	loggedOnce sync.Once
}

func (h *suppressingErrorHandler) Handle(err error) {
	// Only log trace export errors once, at debug level
	if strings.Contains(err.Error(), "traces export") {
		h.loggedOnce.Do(func() {
			logger.Debugf("Tracing endpoint unavailable (further errors suppressed): %v", err)
		})
		return
	}
	// Log other OTel errors at debug level
	logger.Debugf("OpenTelemetry error: %v", err)
}

// Config holds LLM tracing configuration
type Config struct {
	Enabled     bool
	Endpoint    string
	APIKey      string
	ProjectName string
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

		// Apply defaults if not set
		if cfg.Endpoint == "" {
			cfg.Endpoint = DefaultEndpoint
		}
		if cfg.ProjectName == "" {
			cfg.ProjectName = DefaultProjectName
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

	// Build headers map with project name for collectors that support it
	headers := map[string]string{
		"phoenix-project-name": cfg.ProjectName, // Phoenix-specific header
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

	// Set custom error handler to suppress repeated trace export errors
	otel.SetErrorHandler(&suppressingErrorHandler{})

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

// ForceFlush forces any pending spans to be exported.
// This is useful before fetching trace data from Phoenix.
func ForceFlush(ctx context.Context) error {
	if globalProvider != nil {
		return globalProvider.ForceFlush(ctx)
	}
	return nil
}

// EndChainSpan flushes pending spans and ends the chain span. Use in defer after StartChainSpan.
func EndChainSpan(ctx context.Context, span trace.Span) {
	if err := ForceFlush(ctx); err != nil {
		logger.Debugf("Failed to flush traces before ending chain span: %v", err)
	}
	span.End()
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

// providerAttrs returns GenAISystem and LLMProvider attributes for the given provider (empty = gemini).
func providerAttrs(provider string) (genAISystem, llmProvider string) {
	if provider == "" {
		return "gemini", "google"
	}
	return provider, provider
}

// StartSessionSpan starts a root span for an entire session/conversation.
// It generates a unique session ID and stores it in the context for propagation to child spans.
// It also initializes token tracking for the session.
// provider is the LLM provider name (e.g. gemini).
func StartSessionSpan(ctx context.Context, sessionName string, modelID string, provider string) (context.Context, trace.Span) {
	sessionID := uuid.New().String()
	genAISystem, llmProvider := providerAttrs(provider)

	// Store session ID at package level for retrieval
	setCurrentSessionID(sessionID)

	// Store session ID and token tracker in context for child spans
	ctx = WithSessionID(ctx, sessionID)
	ctx = withSessionTokens(ctx, &SessionTokens{})

	// Create the session span - this will be a root span since the incoming
	// context from cmd.Context() should not have any parent span
	ctx, span := Tracer().Start(ctx, sessionName,
		trace.WithAttributes(
			attribute.String(AttrOpenInferenceSpanKind, SpanKindChain),
			attribute.String(AttrSessionID, sessionID),
			attribute.String(AttrLLMModelName, modelID),
			attribute.String(AttrLLMProvider, llmProvider),
			attribute.String(AttrGenAIRequestModel, modelID),
			attribute.String(AttrGenAISystem, genAISystem),
			// Build info for version tracking
			attribute.String(AttrBuildCommit, version.CommitHash),
			attribute.String(AttrBuildTime, version.BuildTime),
			attribute.String(AttrBuildVersion, version.Tag),
		),
		trace.WithSpanKind(trace.SpanKindServer),
	)
	recordSpanID(span)

	// Log trace info for debugging
	if tracingEnabled {
		sc := span.SpanContext()
		logger.Debugf("Started session span: name=%s, traceID=%s, spanID=%s, sessionID=%s",
			sessionName, sc.TraceID().String(), sc.SpanID().String(), sessionID)
	}

	return ctx, span
}

// setCurrentSessionID sets the current session ID (thread-safe)
func setCurrentSessionID(sessionID string) {
	currentSessionMutex.Lock()
	defer currentSessionMutex.Unlock()
	currentSessionID = sessionID
}

// GetSessionID returns the current session ID (thread-safe)
// Returns empty string if no session is active
func GetSessionID() string {
	currentSessionMutex.RLock()
	defer currentSessionMutex.RUnlock()
	return currentSessionID
}

// ClearSessionID clears the current session ID
func ClearSessionID() {
	currentSessionMutex.Lock()
	defer currentSessionMutex.Unlock()
	currentSessionID = ""
}

// EndSessionSpan records the final session output and token counts, then ends the span.
// It flushes all pending spans before ending to ensure child spans are exported first.
func EndSessionSpan(ctx context.Context, span trace.Span, output string) {
	// Force flush all pending spans before ending session to ensure child spans are exported
	if globalProvider != nil {
		_ = globalProvider.ForceFlush(ctx)
	}

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

	span.SetStatus(codes.Ok, "")
	span.End()
}

// EndSessionSpanWithError records an error and ends the session span.
// It flushes all pending spans before ending to ensure child spans are exported first.
func EndSessionSpanWithError(ctx context.Context, span trace.Span, err error) {
	// Force flush all pending spans before ending session to ensure child spans are exported
	if globalProvider != nil {
		_ = globalProvider.ForceFlush(ctx)
	}

	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
	span.End()
}

// RecordSessionInput records the input value on a session span.
func RecordSessionInput(span trace.Span, input string) {
	span.SetAttributes(attribute.String(AttrInputValue, input))
}

// StartChainSpan starts a new span for a chain of operations (e.g., document generation)
func StartChainSpan(ctx context.Context, name string) (context.Context, trace.Span) {
	// Get parent span for debug logging
	parentSpan := trace.SpanFromContext(ctx)
	parentSpanCtx := parentSpan.SpanContext()

	attrs := []attribute.KeyValue{
		attribute.String(AttrOpenInferenceSpanKind, SpanKindChain),
	}
	attrs = append(attrs, sessionAttributes(ctx)...)
	ctx, span := Tracer().Start(ctx, name, trace.WithAttributes(attrs...))
	recordSpanID(span)

	// Log span hierarchy for debugging
	if tracingEnabled {
		sc := span.SpanContext()
		sessionID, hasSession := SessionIDFromContext(ctx)
		logger.Debugf("Started chain span: name=%s, traceID=%s, spanID=%s, parentSpanID=%s, sessionID=%s (found=%v)",
			name, sc.TraceID().String(), sc.SpanID().String(),
			parentSpanCtx.SpanID().String(), sessionID, hasSession)
	}

	return ctx, span
}

// StartAgentSpan starts a new span for an agent task execution.
// provider is the LLM provider name (e.g. gemini); empty defaults to gemini/google.
func StartAgentSpan(ctx context.Context, name string, modelID string, provider string) (context.Context, trace.Span) {
	parentSpan := trace.SpanFromContext(ctx)
	parentSpanCtx := parentSpan.SpanContext()
	genAISystem, llmProvider := providerAttrs(provider)

	attrs := []attribute.KeyValue{
		attribute.String(AttrOpenInferenceSpanKind, SpanKindAgent),
		attribute.String(AttrGenAISystem, genAISystem),
		attribute.String(AttrGenAIRequestModel, modelID),
		attribute.String(AttrLLMModelName, modelID),
		attribute.String(AttrLLMProvider, llmProvider),
	}
	attrs = append(attrs, sessionAttributes(ctx)...)
	ctx, span := Tracer().Start(ctx, name, trace.WithAttributes(attrs...))
	recordSpanID(span)

	// Log span hierarchy for debugging
	sc := span.SpanContext()
	if tracingEnabled {
		sessionID, hasSession := SessionIDFromContext(ctx)
		logger.Debugf("Started agent span: name=%s, traceID=%s, spanID=%s, parentSpanID=%s, sessionID=%s (found=%v)",
			name, sc.TraceID().String(), sc.SpanID().String(),
			parentSpanCtx.SpanID().String(), sessionID, hasSession)
	}

	return ctx, span
}

// StartLLMSpan starts a new span for an LLM call.
// provider is the LLM provider name (e.g. gemini); empty defaults to gemini/google.
func StartLLMSpan(ctx context.Context, name string, modelID string, provider string, inputMessages []Message) (context.Context, trace.Span) {
	genAISystem, llmProvider := providerAttrs(provider)
	attrs := []attribute.KeyValue{
		attribute.String(AttrOpenInferenceSpanKind, SpanKindLLM),
		attribute.String(AttrGenAISystem, genAISystem),
		attribute.String(AttrGenAIRequestModel, modelID),
		attribute.String(AttrLLMModelName, modelID),
		attribute.String(AttrLLMProvider, llmProvider),
	}
	attrs = append(attrs, sessionAttributes(ctx)...)

	if len(inputMessages) > 0 {
		if msgJSON, err := json.Marshal(inputMessages); err == nil {
			attrs = append(attrs, attribute.String(AttrLLMInputMessages, string(msgJSON)))
		}
		// Set input.value for Phoenix display and cost calculation
		attrs = append(attrs, attribute.String(AttrInputValue, inputMessages[0].Content))
	}

	ctx, span := Tracer().Start(ctx, name, trace.WithAttributes(attrs...))
	recordSpanID(span)
	return ctx, span
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

	// Always set token counts (even if 0) for cost calculation
	span.SetAttributes(
		attribute.Int(AttrLLMTokenCountPrompt, promptTokens),
		attribute.Int(AttrLLMTokenCountCompletion, completionTokens),
		attribute.Int(AttrLLMTokenCountTotal, promptTokens+completionTokens),
	)

	// Accumulate tokens to session tracker
	if tokens := SessionTokensFromContext(ctx); tokens != nil {
		tokens.Add(promptTokens, completionTokens)
	}

	span.SetStatus(codes.Ok, "")
	span.End()
}

// StartToolSpan starts a new span for a tool call
func StartToolSpan(ctx context.Context, toolName string, parameters map[string]any) (context.Context, trace.Span) {
	attrs := []attribute.KeyValue{
		// OpenInference span kind
		attribute.String(AttrOpenInferenceSpanKind, SpanKindTool),
		// Standard tool attributes
		attribute.String(AttrToolName, toolName),
		// Gen AI tool attributes for Phoenix
		attribute.String(AttrGenAIOperationName, "execute_tool"),
		attribute.String(AttrGenAIToolType, "FunctionTool"),
		attribute.String(AttrGenAIToolName, toolName),
	}
	attrs = append(attrs, sessionAttributes(ctx)...)

	if parameters != nil {
		if paramJSON, err := json.Marshal(parameters); err == nil {
			attrs = append(attrs, attribute.String(AttrToolParameters, string(paramJSON)))
		}
	}

	ctx, span := Tracer().Start(ctx, "execute_tool "+toolName, trace.WithAttributes(attrs...))
	recordSpanID(span)
	return ctx, span
}

// EndToolSpan records the tool output and ends the span
func EndToolSpan(span trace.Span, output string, err error) {
	if output != "" {
		span.SetAttributes(attribute.String(AttrToolOutput, output))
		span.SetAttributes(attribute.String(AttrOutputValue, output))
	}
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	} else {
		span.SetStatus(codes.Ok, "")
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

// recordSpanID records the span's own ID as an attribute for visualization tools.
// This allows trace visualizers to build proper parent-child trees.
func recordSpanID(span trace.Span) {
	spanID := span.SpanContext().SpanID().String()
	span.SetAttributes(attribute.String(AttrSpanID, spanID))
}

// startWorkflowSpanInternal is a helper that creates workflow spans with common attributes and logging
func startWorkflowSpanInternal(ctx context.Context, name string, extraAttrs []attribute.KeyValue) (context.Context, trace.Span) {
	parentSpanCtx := trace.SpanFromContext(ctx).SpanContext()

	attrs := []attribute.KeyValue{
		attribute.String(AttrOpenInferenceSpanKind, SpanKindWorkflow),
		attribute.String(AttrWorkflowName, name),
	}
	attrs = append(attrs, extraAttrs...)
	attrs = append(attrs, sessionAttributes(ctx)...)

	ctx, span := Tracer().Start(ctx, name, trace.WithAttributes(attrs...))
	recordSpanID(span)

	if tracingEnabled {
		sc := span.SpanContext()
		sessionID, hasSession := SessionIDFromContext(ctx)
		logger.Debugf("Started workflow span: name=%s, traceID=%s, spanID=%s, parentSpanID=%s, sessionID=%s (found=%v)",
			name, sc.TraceID().String(), sc.SpanID().String(),
			parentSpanCtx.SpanID().String(), sessionID, hasSession)
	}

	return ctx, span
}

// StartWorkflowSpan starts a new span for a multi-agent workflow execution
func StartWorkflowSpan(ctx context.Context, name string) (context.Context, trace.Span) {
	return startWorkflowSpanInternal(ctx, name, nil)
}

// StartWorkflowSpanWithConfig starts a workflow span with iteration configuration
func StartWorkflowSpanWithConfig(ctx context.Context, name string, maxIterations uint) (context.Context, trace.Span) {
	return startWorkflowSpanInternal(ctx, name, []attribute.KeyValue{
		attribute.Int(AttrWorkflowMaxIter, int(maxIterations)),
	})
}

// StartSectionWorkflowSpan starts a workflow span for a specific documentation section
func StartSectionWorkflowSpan(ctx context.Context, name string, maxIterations uint, sectionTitle string, sectionLevel int) (context.Context, trace.Span) {
	return startWorkflowSpanInternal(ctx, name, []attribute.KeyValue{
		attribute.Int(AttrWorkflowMaxIter, int(maxIterations)),
		attribute.String(AttrSectionTitle, sectionTitle),
		attribute.Int(AttrSectionLevel, sectionLevel),
	})
}

// StartSubAgentSpan starts a new span for a sub-agent within a workflow
func StartSubAgentSpan(ctx context.Context, agentName string, role string) (context.Context, trace.Span) {
	attrs := []attribute.KeyValue{
		attribute.String(AttrOpenInferenceSpanKind, SpanKindAgent),
		attribute.String(AttrSubAgentName, agentName),
		attribute.String(AttrSubAgentRole, role),
	}
	attrs = append(attrs, sessionAttributes(ctx)...)
	ctx, span := Tracer().Start(ctx, "subagent:"+agentName, trace.WithAttributes(attrs...))
	recordSpanID(span)
	return ctx, span
}

// RecordWorkflowIteration records the current iteration number on a workflow span
func RecordWorkflowIteration(span trace.Span, iteration int) {
	span.SetAttributes(attribute.Int(AttrWorkflowIteration, iteration))
}

// RecordWorkflowResult records the final workflow result on a span and sets status
func RecordWorkflowResult(span trace.Span, approved bool, iterations int, content string) {
	span.SetAttributes(
		attribute.Bool("workflow.approved", approved),
		attribute.Int("workflow.total_iterations", iterations),
	)
	if content != "" {
		span.SetAttributes(attribute.String(AttrOutputValue, content))
	}
	span.SetStatus(codes.Ok, "")
}

// SetSpanOk marks a span as successfully completed
func SetSpanOk(span trace.Span) {
	span.SetStatus(codes.Ok, "")
}

// SetSpanError marks a span as failed with an error
func SetSpanError(span trace.Span, err error) {
	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
}
