// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package tracing

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func preserveTracingGlobals(t *testing.T) {
	t.Helper()

	previousTracer := globalTracer
	previousProvider := globalProvider
	previousConfiguredOTelProvider := previousOTelProvider
	previousConfiguredErrorHandler := previousErrorHandler
	previousInitialized := tracingInitialized
	previousEnabled := tracingEnabled.Load()
	previousGlobalOTelProvider := otel.GetTracerProvider()
	previousGlobalErrorHandler := otel.GetErrorHandler()
	previousSessionID := GetSessionID()

	t.Cleanup(func() {
		if globalProvider != nil && globalProvider != previousProvider {
			_ = globalProvider.Shutdown(context.Background())
		}
		globalTracer = previousTracer
		globalProvider = previousProvider
		previousOTelProvider = previousConfiguredOTelProvider
		previousErrorHandler = previousConfiguredErrorHandler
		tracingInitialized = previousInitialized
		tracingEnabled.Store(previousEnabled)
		otel.SetTracerProvider(previousGlobalOTelProvider)
		otel.SetErrorHandler(previousGlobalErrorHandler)
		setCurrentSessionID(previousSessionID)
	})
}

func useSpanRecorder(t *testing.T) *tracetest.SpanRecorder {
	t.Helper()
	preserveTracingGlobals(t)

	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	globalProvider = provider
	globalTracer = provider.Tracer(TracerName)
	return recorder
}

func TestNewResourceUsesOpenInferenceProjectName(t *testing.T) {
	res, err := newResource(Config{ProjectName: "documentation-agent"})
	require.NoError(t, err)

	attrs := make(map[string]string)
	for _, attr := range res.Attributes() {
		attrs[string(attr.Key)] = attr.Value.AsString()
	}
	assert.Equal(t, DefaultProjectName, attrs["service.name"])
	assert.Equal(t, "documentation-agent", attrs[AttrOpenInferenceProject])
	assert.NotContains(t, attrs, "project.name")
}

func TestEndSessionSpanExportsEndedRoot(t *testing.T) {
	recorder := useSpanRecorder(t)

	ctx, span := StartSessionSpan(context.Background(), "session-root", "gemini-test", "gemini")
	EndSessionSpan(ctx, span, "complete")

	ended := recorder.Ended()
	require.Len(t, ended, 1)
	assert.Equal(t, "session-root", ended[0].Name())
	assert.Equal(t, codes.Ok, ended[0].Status().Code)
}

func TestEndSessionSpanWithErrorRecordsFailure(t *testing.T) {
	recorder := useSpanRecorder(t)

	ctx, span := StartSessionSpan(context.Background(), "session-root", "gemini-test", "gemini")
	_, llmSpan := StartLLMSpan(ctx, "llm:response", "gemini-test", "gemini", nil)
	EndLLMSpan(ctx, llmSpan, nil, 3, 5)
	EndSessionSpanWithError(ctx, span, errors.New("generation failed"))

	ended := recorder.Ended()
	require.Len(t, ended, 2)
	root := ended[1]
	assert.Equal(t, "session-root", root.Name())
	assert.Equal(t, codes.Error, root.Status().Code)
	assert.Equal(t, "generation failed", root.Status().Description)

	attrs := make(map[string]any)
	for _, attr := range root.Attributes() {
		attrs[string(attr.Key)] = attr.Value.AsInterface()
	}
	assert.Equal(t, int64(3), attrs[AttrLLMTokenCountPrompt])
	assert.Equal(t, int64(5), attrs[AttrLLMTokenCountCompletion])
	assert.Equal(t, int64(8), attrs[AttrLLMTokenCountTotal])
}

func TestToolSpanRecordsParametersAndFailure(t *testing.T) {
	recorder := useSpanRecorder(t)

	_, span := StartToolSpan(context.Background(), "read_file", map[string]any{"path": "README.md"})
	EndToolSpan(span, "permission denied", errors.New("tool failed"))

	ended := recorder.Ended()
	require.Len(t, ended, 1)
	assert.Equal(t, "tool:read_file", ended[0].Name())
	assert.Equal(t, codes.Error, ended[0].Status().Code)

	attrs := make(map[string]any)
	for _, attr := range ended[0].Attributes() {
		attrs[string(attr.Key)] = attr.Value.AsInterface()
	}
	parameters, ok := attrs[AttrToolParameters].(string)
	require.True(t, ok)
	assert.JSONEq(t, `{"path":"README.md"}`, parameters)
	assert.Equal(t, "permission denied", attrs[AttrToolOutput])
}

func TestToolSpanTrackerMatchesResponseWithoutID(t *testing.T) {
	recorder := useSpanRecorder(t)
	tracker := NewToolSpanTracker(context.Background())

	tracker.Start("call-id", "read_file", map[string]any{"path": "README.md"})
	output, err := tracker.End("", "read_file", map[string]any{"output": "contents"})
	require.NoError(t, err)
	assert.Equal(t, "contents", output)
	assert.Empty(t, tracker.pending)

	ended := recorder.Ended()
	require.Len(t, ended, 1)
	assert.Equal(t, "tool:read_file", ended[0].Name())
	assert.Equal(t, codes.Ok, ended[0].Status().Code)
}

func TestToolSpanTrackerMarksResponseError(t *testing.T) {
	recorder := useSpanRecorder(t)
	tracker := NewToolSpanTracker(context.Background())

	tracker.Start("", "read_file", nil)
	output, err := tracker.End("", "read_file", map[string]any{"error": "permission denied"})
	require.EqualError(t, err, "tool read_file failed: permission denied")
	assert.Equal(t, "Error: permission denied", output)

	ended := recorder.Ended()
	require.Len(t, ended, 1)
	assert.Equal(t, codes.Error, ended[0].Status().Code)
}

func TestToolSpanTrackerMarksEncodingError(t *testing.T) {
	recorder := useSpanRecorder(t)
	tracker := NewToolSpanTracker(context.Background())

	tracker.Start("", "read_file", nil)
	_, err := tracker.End("", "read_file", map[string]any{"invalid": func() {}})
	require.ErrorContains(t, err, "encoding response from tool read_file")

	ended := recorder.Ended()
	require.Len(t, ended, 1)
	assert.Equal(t, codes.Error, ended[0].Status().Code)
}

func TestToolSpanTrackerEndsUnmatchedCalls(t *testing.T) {
	recorder := useSpanRecorder(t)
	tracker := NewToolSpanTracker(context.Background())

	tracker.Start("", "read_file", nil)
	tracker.EndPending(nil)

	assert.Empty(t, tracker.pending)
	ended := recorder.Ended()
	require.Len(t, ended, 1)
	assert.Equal(t, codes.Error, ended[0].Status().Code)
	assert.Equal(t, "tool call ended without a response", ended[0].Status().Description)
}

func TestInitWithConfigCanRetryAfterFailure(t *testing.T) {
	preserveTracingGlobals(t)
	tracingInitialized = false
	tracingEnabled.Store(false)

	err := InitWithConfig(context.Background(), Config{
		Enabled:  true,
		Endpoint: "http://[::1",
	})
	require.Error(t, err)
	assert.False(t, IsEnabled())
	assert.False(t, tracingInitialized)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	err = InitWithConfig(context.Background(), Config{
		Enabled:     true,
		Endpoint:    server.URL + "/v1/traces",
		ProjectName: "retry-test",
	})
	require.NoError(t, err)
	assert.True(t, IsEnabled())
	assert.True(t, tracingInitialized)
}

func TestInitWithConfigCanReinitializeAfterShutdown(t *testing.T) {
	preserveTracingGlobals(t)
	tracingInitialized = false
	tracingEnabled.Store(false)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	cfg := Config{
		Enabled:     true,
		Endpoint:    server.URL + "/v1/traces",
		ProjectName: "reinitialize-test",
	}

	require.NoError(t, InitWithConfig(context.Background(), cfg))
	assert.True(t, IsEnabled())
	require.NoError(t, Shutdown(context.Background()))
	assert.False(t, IsEnabled())
	assert.False(t, tracingInitialized)

	require.NoError(t, InitWithConfig(context.Background(), cfg))
	assert.True(t, IsEnabled())
	assert.True(t, tracingInitialized)
}

func TestOTLPExporterUsesHeadersAndExportsRootSpan(t *testing.T) {
	preserveTracingGlobals(t)

	type receivedRequest struct {
		path          string
		authorization string
		apiKey        string
		body          []byte
		err           error
	}
	received := make(chan receivedRequest, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		received <- receivedRequest{
			path:          r.URL.Path,
			authorization: r.Header.Get("Authorization"),
			apiKey:        r.Header.Get("api_key"),
			body:          body,
			err:           err,
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	err := initTracer(context.Background(), Config{
		Endpoint:    server.URL + "/v1/traces",
		APIKey:      "api-secret",
		ProjectName: "documentation-agent",
		Headers: map[string]string{
			"Authorization": "Bearer token",
		},
	})
	require.NoError(t, err)

	ctx, span := StartSessionSpan(context.Background(), "session-root", "gemini-test", "gemini")
	EndSessionSpan(ctx, span, "complete")

	select {
	case request := <-received:
		require.NoError(t, request.err)
		assert.Equal(t, "/v1/traces", request.path)
		assert.Equal(t, "Bearer token", request.authorization)
		assert.Equal(t, "api-secret", request.apiKey)
		assert.True(t, bytes.Contains(request.body, []byte("session-root")))
		assert.True(t, bytes.Contains(request.body, []byte(AttrOpenInferenceProject)))
		assert.True(t, bytes.Contains(request.body, []byte("documentation-agent")))
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for OTLP export")
	}
}
