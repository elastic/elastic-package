// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package tracing

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

// PhoenixClient fetches traces from Arize Phoenix
type PhoenixClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewPhoenixClient creates a new Phoenix client
func NewPhoenixClient(baseURL string) *PhoenixClient {
	if baseURL == "" {
		baseURL = DefaultEndpoint
	}
	// Remove /v1/traces suffix if present (we need the base URL for GraphQL)
	baseURL = strings.TrimSuffix(baseURL, "/v1/traces")

	return &PhoenixClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// SessionTraces represents trace data for a session
type SessionTraces struct {
	SessionID string        `json:"session_id"`
	NumTraces int           `json:"num_traces"`
	StartTime time.Time     `json:"start_time"`
	EndTime   time.Time     `json:"end_time"`
	Traces    []TraceData   `json:"traces"`
	Summary   *TraceSummary `json:"summary,omitempty"`
}

// TraceData represents a single trace
type TraceData struct {
	TraceID   string     `json:"trace_id"`
	StartTime time.Time  `json:"start_time"`
	EndTime   time.Time  `json:"end_time"`
	LatencyMs float64    `json:"latency_ms"`
	Spans     []SpanData `json:"spans"`
}

// SpanData represents a single span
type SpanData struct {
	SpanID               string                 `json:"span_id"`
	Name                 string                 `json:"name"`
	SpanKind             string                 `json:"span_kind"`
	StatusCode           string                 `json:"status_code"`
	StatusMessage        string                 `json:"status_message,omitempty"`
	StartTime            time.Time              `json:"start_time"`
	EndTime              time.Time              `json:"end_time"`
	LatencyMs            float64                `json:"latency_ms"`
	ParentID             string                 `json:"parent_id,omitempty"`
	TokenCountTotal      int                    `json:"token_count_total,omitempty"`
	TokenCountPrompt     int                    `json:"token_count_prompt,omitempty"`
	TokenCountCompletion int                    `json:"token_count_completion,omitempty"`
	Input                string                 `json:"input,omitempty"`
	Output               string                 `json:"output,omitempty"`
	Attributes           map[string]interface{} `json:"attributes,omitempty"`
}

// TraceSummary provides aggregated trace statistics
type TraceSummary struct {
	TotalSpans            int                `json:"total_spans"`
	TotalLatencyMs        float64            `json:"total_latency_ms"`
	TotalPromptTokens     int                `json:"total_prompt_tokens"`
	TotalCompletionTokens int                `json:"total_completion_tokens"`
	TotalTokens           int                `json:"total_tokens"`
	LLMCalls              int                `json:"llm_calls"`
	AgentCalls            []AgentCallSummary `json:"agent_calls"`
	ValidationResults     []ValidationResult `json:"validation_results,omitempty"`
	LLMCallDetails        []LLMCallDetail    `json:"llm_call_details,omitempty"`
	SignificantEvents     []SignificantEvent `json:"significant_events"`
	Errors                []TraceError       `json:"errors,omitempty"`
}

// AgentCallSummary summarizes an agent's activity
type AgentCallSummary struct {
	AgentName      string  `json:"agent_name"`
	CallCount      int     `json:"call_count"`
	TotalLatencyMs float64 `json:"total_latency_ms"`
	TotalTokens    int     `json:"total_tokens"`
	Approved       *bool   `json:"approved,omitempty"`
	Score          *int    `json:"score,omitempty"`
}

// ValidationResult captures validation stage results
type ValidationResult struct {
	Stage      string   `json:"stage"`
	Validator  string   `json:"validator,omitempty"`
	Valid      bool     `json:"valid"`
	Score      int      `json:"score,omitempty"`
	IssueCount int      `json:"issue_count"`
	Issues     []string `json:"issues,omitempty"`
	Iteration  int      `json:"iteration,omitempty"`
	LatencyMs  float64  `json:"latency_ms,omitempty"`
	Tokens     int      `json:"tokens,omitempty"`
	Source     string   `json:"source,omitempty"` // "static" or "llm"
}

// LLMCallDetail captures details of an LLM call
type LLMCallDetail struct {
	Timestamp        time.Time `json:"timestamp"`
	SpanName         string    `json:"span_name"`
	Model            string    `json:"model,omitempty"`
	PromptTokens     int       `json:"prompt_tokens"`
	CompletionTokens int       `json:"completion_tokens"`
	LatencyMs        float64   `json:"latency_ms"`
	InputPreview     string    `json:"input_preview,omitempty"`  // First 500 chars
	OutputPreview    string    `json:"output_preview,omitempty"` // First 500 chars
	Purpose          string    `json:"purpose,omitempty"`        // e.g., "generation", "validation", "critic"
}

// TraceError captures error details from spans
type TraceError struct {
	Timestamp  time.Time `json:"timestamp"`
	SpanName   string    `json:"span_name"`
	Message    string    `json:"message"`
	StatusCode string    `json:"status_code,omitempty"`
	StackTrace string    `json:"stack_trace,omitempty"`
}

// SignificantEvent represents an important event during documentation generation
type SignificantEvent struct {
	Timestamp   time.Time `json:"timestamp"`
	Type        string    `json:"type"` // "llm_call", "validation", "iteration", "error", "agent"
	Agent       string    `json:"agent,omitempty"`
	Description string    `json:"description"`
	LatencyMs   float64   `json:"latency_ms,omitempty"`
	Tokens      int       `json:"tokens,omitempty"`
	Severity    string    `json:"severity,omitempty"` // "info", "warning", "error"
	Details     string    `json:"details,omitempty"`
}

// graphqlQuery is the GraphQL query to fetch session traces
const graphqlQuery = `
query GetSessionTraces($sessionId: String!) {
  getProjectSessionById(sessionId: $sessionId) {
    sessionId
    numTraces
    startTime
    endTime
    traces(first: 100) {
      edges {
        node {
          traceId
          startTime
          endTime
          latencyMs
          spans(first: 1000) {
            edges {
              node {
                spanId
                name
                spanKind
                statusCode
                statusMessage
                startTime
                endTime
                latencyMs
                parentId
                tokenCountTotal
                tokenCountPrompt
                tokenCountCompletion
                input {
                  value
                }
                output {
                  value
                }
                attributes
              }
            }
          }
        }
      }
    }
  }
}
`

// FetchSessionTraces fetches all traces for a given session ID
func (c *PhoenixClient) FetchSessionTraces(ctx context.Context, sessionID string) (*SessionTraces, error) {
	graphqlURL := c.baseURL + "/graphql"

	reqBody := map[string]interface{}{
		"query": graphqlQuery,
		"variables": map[string]string{
			"sessionId": sessionID,
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", graphqlURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Data struct {
			GetProjectSessionById *struct {
				SessionID string `json:"sessionId"`
				NumTraces int    `json:"numTraces"`
				StartTime string `json:"startTime"`
				EndTime   string `json:"endTime"`
				Traces    struct {
					Edges []struct {
						Node struct {
							TraceID   string  `json:"traceId"`
							StartTime string  `json:"startTime"`
							EndTime   string  `json:"endTime"`
							LatencyMs float64 `json:"latencyMs"`
							Spans     struct {
								Edges []struct {
									Node struct {
										SpanID               string  `json:"spanId"`
										Name                 string  `json:"name"`
										SpanKind             string  `json:"spanKind"`
										StatusCode           string  `json:"statusCode"`
										StatusMessage        string  `json:"statusMessage"`
										StartTime            string  `json:"startTime"`
										EndTime              string  `json:"endTime"`
										LatencyMs            float64 `json:"latencyMs"`
										ParentID             string  `json:"parentId"`
										TokenCountTotal      int     `json:"tokenCountTotal"`
										TokenCountPrompt     int     `json:"tokenCountPrompt"`
										TokenCountCompletion int     `json:"tokenCountCompletion"`
										Input                *struct {
											Value string `json:"value"`
										} `json:"input"`
										Output *struct {
											Value string `json:"value"`
										} `json:"output"`
										Attributes json.RawMessage `json:"attributes"`
									} `json:"node"`
								} `json:"edges"`
							} `json:"spans"`
						} `json:"node"`
					} `json:"edges"`
				} `json:"traces"`
			} `json:"getProjectSessionById"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(result.Errors) > 0 {
		return nil, fmt.Errorf("GraphQL errors: %v", result.Errors)
	}

	sessionData := result.Data.GetProjectSessionById
	if sessionData == nil {
		return nil, nil // No session found
	}

	// Parse into our structures
	traces := &SessionTraces{
		SessionID: sessionData.SessionID,
		NumTraces: sessionData.NumTraces,
		StartTime: parseTime(sessionData.StartTime),
		EndTime:   parseTime(sessionData.EndTime),
	}

	for _, traceEdge := range sessionData.Traces.Edges {
		t := traceEdge.Node
		traceData := TraceData{
			TraceID:   t.TraceID,
			StartTime: parseTime(t.StartTime),
			EndTime:   parseTime(t.EndTime),
			LatencyMs: t.LatencyMs,
		}

		for _, spanEdge := range t.Spans.Edges {
			s := spanEdge.Node
			span := SpanData{
				SpanID:               s.SpanID,
				Name:                 s.Name,
				SpanKind:             s.SpanKind,
				StatusCode:           s.StatusCode,
				StatusMessage:        s.StatusMessage,
				StartTime:            parseTime(s.StartTime),
				EndTime:              parseTime(s.EndTime),
				LatencyMs:            s.LatencyMs,
				ParentID:             s.ParentID,
				TokenCountTotal:      s.TokenCountTotal,
				TokenCountPrompt:     s.TokenCountPrompt,
				TokenCountCompletion: s.TokenCountCompletion,
			}

			if s.Input != nil {
				span.Input = s.Input.Value
			}
			if s.Output != nil {
				span.Output = s.Output.Value
			}

			if len(s.Attributes) > 0 {
				var attrs map[string]interface{}
				if err := json.Unmarshal(s.Attributes, &attrs); err == nil {
					span.Attributes = attrs
				}
			}

			traceData.Spans = append(traceData.Spans, span)
		}

		traces.Traces = append(traces.Traces, traceData)
	}

	// Generate summary
	traces.Summary = c.generateSummary(traces)

	return traces, nil
}

// generateSummary creates a comprehensive summary of the trace data
func (c *PhoenixClient) generateSummary(traces *SessionTraces) *TraceSummary {
	summary := &TraceSummary{
		AgentCalls:        []AgentCallSummary{},
		ValidationResults: []ValidationResult{},
		LLMCallDetails:    []LLMCallDetail{},
		SignificantEvents: []SignificantEvent{},
		Errors:            []TraceError{},
	}
	agentStats := make(map[string]*AgentCallSummary)
	for _, trace := range traces.Traces {
		summary.TotalLatencyMs += trace.LatencyMs
		for _, span := range trace.Spans {
			processSpan(summary, agentStats, span)
		}
	}
	finalizeSummary(summary, agentStats)
	return summary
}

// processSpan categorizes a span and updates the summary accordingly.
func processSpan(summary *TraceSummary, agentStats map[string]*AgentCallSummary, span SpanData) {
	summary.TotalSpans++
	summary.TotalPromptTokens += span.TokenCountPrompt
	summary.TotalCompletionTokens += span.TokenCountCompletion
	summary.TotalTokens += span.TokenCountTotal

	if span.Name == "call_llm" || strings.HasPrefix(span.Name, "llm:") {
		summary.LLMCalls++
	}
	if strings.HasPrefix(span.Name, "llm:") || (span.Name == "call_llm" && span.TokenCountTotal > 0) {
		if detail, ok := buildLLMCallDetail(span); ok {
			summary.LLMCallDetails = append(summary.LLMCallDetails, detail)
		}
	}
	if strings.HasPrefix(span.Name, "validation:") ||
		strings.HasPrefix(span.Name, "llm_validation:") ||
		strings.HasPrefix(span.Name, "static_validation:") {
		valResult := parseValidationSpan(span)
		summary.ValidationResults = append(summary.ValidationResults, valResult)
		summary.SignificantEvents = append(summary.SignificantEvents, buildValidationEvent(valResult, span))
	}
	if strings.HasPrefix(span.Name, "agent:") {
		summary.SignificantEvents = append(summary.SignificantEvents, processAgentSpan(agentStats, span))
	}
	if span.Name == "generation_iteration" {
		summary.SignificantEvents = append(summary.SignificantEvents, buildIterationEvent(span))
	}
	if strings.HasPrefix(span.Name, "workflow:") {
		summary.SignificantEvents = append(summary.SignificantEvents, buildWorkflowEvent(span))
	}
	if strings.HasPrefix(span.Name, "test:") || strings.HasPrefix(span.Name, "doc:") {
		summary.SignificantEvents = append(summary.SignificantEvents, SignificantEvent{
			Timestamp:   span.StartTime,
			Type:        "session",
			Description: fmt.Sprintf("Session: %s", span.Name),
			LatencyMs:   span.LatencyMs,
			Tokens:      span.TokenCountTotal,
			Severity:    "info",
		})
	}
	if span.StatusCode == "ERROR" || span.StatusCode == "error" {
		traceErr, event := buildErrorSpanInfo(span)
		summary.Errors = append(summary.Errors, traceErr)
		summary.SignificantEvents = append(summary.SignificantEvents, event)
	}
	if strings.HasPrefix(span.Name, "tool:") {
		event := SignificantEvent{
			Timestamp:   span.StartTime,
			Type:        "tool",
			Description: fmt.Sprintf("Tool: %s", strings.TrimPrefix(span.Name, "tool:")),
			LatencyMs:   span.LatencyMs,
			Severity:    "info",
		}
		if span.Output != "" {
			event.Details = truncateString(span.Output, 200)
		}
		summary.SignificantEvents = append(summary.SignificantEvents, event)
	}
	if strings.HasPrefix(span.Name, "subagent:") {
		subagentName := strings.TrimPrefix(span.Name, "subagent:")
		summary.SignificantEvents = append(summary.SignificantEvents, SignificantEvent{
			Timestamp:   span.StartTime,
			Type:        "subagent",
			Agent:       subagentName,
			Description: fmt.Sprintf("Subagent: %s", subagentName),
			LatencyMs:   span.LatencyMs,
			Tokens:      span.TokenCountTotal,
			Severity:    "info",
		})
	}
}

// extractLLMModel extracts the model name from span attributes.
func extractLLMModel(attrs map[string]interface{}) string {
	if attrs == nil {
		return ""
	}
	if genAI, ok := attrs["gen_ai"].(map[string]interface{}); ok {
		if req, ok := genAI["request"].(map[string]interface{}); ok {
			if model, ok := req["model"].(string); ok {
				return model
			}
		}
		if system, ok := genAI["system"].(string); ok {
			return system
		}
	}
	if llm, ok := attrs["llm"].(map[string]interface{}); ok {
		if model, ok := llm["model_name"].(string); ok {
			return model
		}
	}
	return ""
}

// buildLLMCallDetail builds an LLMCallDetail from a span. Returns ok=false if no meaningful data.
func buildLLMCallDetail(span SpanData) (LLMCallDetail, bool) {
	detail := LLMCallDetail{
		Timestamp:        span.StartTime,
		SpanName:         span.Name,
		PromptTokens:     span.TokenCountPrompt,
		CompletionTokens: span.TokenCountCompletion,
		LatencyMs:        span.LatencyMs,
		Model:            extractLLMModel(span.Attributes),
		Purpose:          determineLLMPurpose(span.Name, span.Attributes),
	}
	if span.Input != "" {
		detail.InputPreview = truncateString(span.Input, 500)
	}
	if span.Output != "" {
		detail.OutputPreview = truncateString(span.Output, 500)
	}
	ok := detail.PromptTokens > 0 || detail.CompletionTokens > 0 ||
		detail.InputPreview != "" || detail.OutputPreview != ""
	return detail, ok
}

// buildValidationEvent creates a SignificantEvent for a validation span.
func buildValidationEvent(valResult ValidationResult, span SpanData) SignificantEvent {
	event := SignificantEvent{
		Timestamp:   span.StartTime,
		Type:        "validation",
		Description: fmt.Sprintf("Validation: %s", span.Name),
		LatencyMs:   span.LatencyMs,
		Tokens:      span.TokenCountTotal,
	}
	if valResult.Valid {
		event.Severity = "info"
		event.Details = fmt.Sprintf("Passed (score: %d)", valResult.Score)
	} else {
		event.Severity = "warning"
		event.Details = fmt.Sprintf("Failed with %d issues", valResult.IssueCount)
		if len(valResult.Issues) > 0 {
			event.Details += ": " + strings.Join(valResult.Issues[:min(3, len(valResult.Issues))], "; ")
		}
	}
	return event
}

// processAgentSpan updates agent stats and returns a SignificantEvent for the agent span.
func processAgentSpan(agentStats map[string]*AgentCallSummary, span SpanData) SignificantEvent {
	agentName := strings.TrimPrefix(span.Name, "agent:")
	if _, ok := agentStats[agentName]; !ok {
		agentStats[agentName] = &AgentCallSummary{AgentName: agentName}
	}
	agentStats[agentName].CallCount++
	agentStats[agentName].TotalLatencyMs += span.LatencyMs
	agentStats[agentName].TotalTokens += span.TokenCountTotal
	event := SignificantEvent{
		Timestamp:   span.StartTime,
		Type:        "agent",
		Agent:       agentName,
		Description: fmt.Sprintf("Agent: %s", agentName),
		LatencyMs:   span.LatencyMs,
		Tokens:      span.TokenCountTotal,
		Severity:    "info",
	}
	if span.Output != "" {
		parseAgentOutput(span.Output, agentStats[agentName], &event)
	}
	return event
}

// buildIterationEvent creates a SignificantEvent for a generation_iteration span.
func buildIterationEvent(span SpanData) SignificantEvent {
	event := SignificantEvent{
		Timestamp:   span.StartTime,
		Type:        "iteration",
		Description: "Generation iteration",
		LatencyMs:   span.LatencyMs,
		Severity:    "info",
	}
	if span.Attributes == nil {
		return event
	}
	gen, ok := span.Attributes["generation"].(map[string]interface{})
	if !ok {
		return event
	}
	if iter, ok := gen["iteration"].(float64); ok {
		event.Description = fmt.Sprintf("Iteration %d", int(iter))
	}
	if feedback, ok := gen["has_feedback"].(bool); ok && feedback {
		event.Details = "Has validation feedback"
	}
	return event
}

// buildWorkflowEvent creates a SignificantEvent for a workflow: span.
func buildWorkflowEvent(span SpanData) SignificantEvent {
	event := SignificantEvent{
		Timestamp:   span.StartTime,
		Type:        "workflow",
		Description: fmt.Sprintf("Workflow: %s", span.Name),
		LatencyMs:   span.LatencyMs,
		Severity:    "info",
	}
	if span.Attributes == nil {
		return event
	}
	workflow, ok := span.Attributes["workflow"].(map[string]interface{})
	if !ok {
		return event
	}
	if iterations, ok := workflow["total_iterations"].(float64); ok {
		event.Description = fmt.Sprintf("Workflow completed in %.0f iterations", iterations)
	}
	if approved, ok := workflow["approved"].(bool); ok {
		if approved {
			event.Details = "Approved"
		} else {
			event.Severity = "warning"
			event.Details = "Not approved after max iterations"
		}
	}
	if maxIter, ok := workflow["max_iterations"].(float64); ok && event.Details != "" {
		event.Details += fmt.Sprintf(" (max: %.0f)", maxIter)
	}
	return event
}

// buildErrorSpanInfo creates a TraceError and SignificantEvent for an error span.
func buildErrorSpanInfo(span SpanData) (TraceError, SignificantEvent) {
	traceErr := TraceError{
		Timestamp:  span.StartTime,
		SpanName:   span.Name,
		Message:    span.StatusMessage,
		StatusCode: span.StatusCode,
	}
	if span.Attributes != nil {
		if stackTrace, ok := span.Attributes["exception.stacktrace"].(string); ok {
			traceErr.StackTrace = truncateString(stackTrace, 1000)
		}
	}
	event := SignificantEvent{
		Timestamp:   span.StartTime,
		Type:        "error",
		Agent:       span.Name,
		Description: fmt.Sprintf("Error in %s", span.Name),
		Severity:    "error",
		Details:     span.StatusMessage,
	}
	return traceErr, event
}

// finalizeSummary converts maps to slices and sorts all summary collections.
func finalizeSummary(summary *TraceSummary, agentStats map[string]*AgentCallSummary) {
	for _, stats := range agentStats {
		summary.AgentCalls = append(summary.AgentCalls, *stats)
	}
	sort.Slice(summary.AgentCalls, func(i, j int) bool {
		return summary.AgentCalls[i].AgentName < summary.AgentCalls[j].AgentName
	})
	sort.Slice(summary.SignificantEvents, func(i, j int) bool {
		return summary.SignificantEvents[i].Timestamp.Before(summary.SignificantEvents[j].Timestamp)
	})
	sort.Slice(summary.ValidationResults, func(i, j int) bool {
		return summary.ValidationResults[i].Iteration < summary.ValidationResults[j].Iteration
	})
	sort.Slice(summary.LLMCallDetails, func(i, j int) bool {
		return summary.LLMCallDetails[i].Timestamp.Before(summary.LLMCallDetails[j].Timestamp)
	})
}

// parseValidationSpan extracts validation details from a span
func parseValidationSpan(span SpanData) ValidationResult {
	result := ValidationResult{
		Stage:     strings.TrimPrefix(strings.TrimPrefix(strings.TrimPrefix(span.Name, "validation:"), "llm_validation:"), "static_validation:"),
		Source:    validationSource(span.Name),
		LatencyMs: span.LatencyMs,
		Tokens:    span.TokenCountTotal,
	}
	if span.Attributes != nil {
		parseValidationAttributes(span.Attributes, &result)
	}
	if span.Output != "" {
		parseValidationOutput(span.Output, &result)
	}
	return result
}

// validationSource returns the validation source ("static", "llm", or "combined") from the span name.
func validationSource(name string) string {
	switch {
	case strings.HasPrefix(name, "static_validation:"):
		return "static"
	case strings.HasPrefix(name, "llm_validation:"):
		return "llm"
	default:
		return "combined"
	}
}

// parseValidationAttributes extracts validation fields from span attributes (nested and flat formats).
func parseValidationAttributes(attributes map[string]interface{}, result *ValidationResult) {
	if validation, ok := attributes["validation"].(map[string]interface{}); ok {
		if stage, ok := validation["stage"].(string); ok {
			result.Stage = stage
		}
		if iter, ok := validation["iteration"].(float64); ok {
			result.Iteration = int(iter)
		}
		if passed, ok := validation["passed"].(bool); ok {
			result.Valid = passed
		}
		if score, ok := validation["score"].(float64); ok {
			result.Score = int(score)
		}
		if issueCount, ok := validation["issues"].(float64); ok {
			result.IssueCount = int(issueCount)
		}
	}
	if passed, ok := attributes["validation.passed"].(bool); ok {
		result.Valid = passed
	}
	if score, ok := attributes["validation.score"].(float64); ok {
		result.Score = int(score)
	}
	if issueCount, ok := attributes["validation.issues"].(float64); ok {
		result.IssueCount = int(issueCount)
	}
	if stage, ok := attributes["validation.stage"].(string); ok && result.Stage == "" {
		result.Stage = stage
	}
	if validator, ok := attributes["validator"].(map[string]interface{}); ok {
		if name, ok := validator["name"].(string); ok {
			result.Validator = name
		}
	}
	if issuesSample, ok := attributes["validation.issues_sample"].(string); ok {
		var issues []string
		if err := json.Unmarshal([]byte(issuesSample), &issues); err == nil {
			result.Issues = issues
			if result.IssueCount == 0 {
				result.IssueCount = len(issues)
			}
		}
	}
}

// parseValidationOutput extracts validation fields from the span's JSON output (fallback).
func parseValidationOutput(output string, result *ValidationResult) {
	var out map[string]interface{}
	if err := json.Unmarshal([]byte(output), &out); err != nil {
		return
	}
	if valid, ok := out["valid"].(bool); ok && !result.Valid {
		result.Valid = valid
	}
	if score, ok := out["score"].(float64); ok && result.Score == 0 {
		result.Score = int(score)
	}
	if issues, ok := out["issues"].([]interface{}); ok && len(result.Issues) == 0 {
		result.IssueCount = len(issues)
		for _, issue := range issues {
			if issueStr, ok := issue.(string); ok {
				result.Issues = append(result.Issues, issueStr)
			}
		}
	}
	if count, ok := out["issue_count"].(float64); ok && result.IssueCount == 0 {
		result.IssueCount = int(count)
	}
	if feedback, ok := out["feedback"].(string); ok && feedback != "" && len(result.Issues) == 0 {
		result.Issues = append(result.Issues, feedback)
	}
}

// parseAgentOutput extracts results from agent output
func parseAgentOutput(output string, stats *AgentCallSummary, event *SignificantEvent) {
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		return
	}

	if approved, ok := result["approved"].(bool); ok {
		stats.Approved = &approved
		if approved {
			event.Details = "approved"
		} else {
			event.Details = "rejected"
			event.Severity = "warning"
		}
	}
	if valid, ok := result["valid"].(bool); ok {
		stats.Approved = &valid
		if valid {
			event.Details = "valid"
		} else {
			event.Details = "invalid"
			event.Severity = "warning"
		}
	}
	if score, ok := result["score"].(float64); ok {
		scoreInt := int(score)
		stats.Score = &scoreInt
		if event.Details != "" {
			event.Details += fmt.Sprintf(" (score: %d)", scoreInt)
		} else {
			event.Details = fmt.Sprintf("score: %d", scoreInt)
		}
	}
	if feedback, ok := result["feedback"].(string); ok && feedback != "" {
		if event.Details != "" {
			event.Details += " - "
		}
		event.Details += truncateString(feedback, 200)
	}
	if issues, ok := result["issues"].([]interface{}); ok && len(issues) > 0 {
		event.Severity = "warning"
		if event.Details != "" {
			event.Details += " - "
		}
		event.Details += fmt.Sprintf("%d issues found", len(issues))
	}
}

// determineLLMPurpose determines the purpose of an LLM call from span info
func determineLLMPurpose(spanName string, attrs map[string]interface{}) string {
	if strings.Contains(spanName, "generator") {
		return "generation"
	}
	if strings.Contains(spanName, "critic") {
		return "critique"
	}
	if strings.Contains(spanName, "validator") {
		return "validation"
	}
	if strings.Contains(spanName, "response") {
		return "response"
	}

	// Check attributes for more context
	if attrs != nil {
		if openinf, ok := attrs["openinference"].(map[string]interface{}); ok {
			if span, ok := openinf["span"].(map[string]interface{}); ok {
				if kind, ok := span["kind"].(string); ok {
					return strings.ToLower(kind)
				}
			}
		}
	}

	return "llm_call"
}

// truncateString truncates a string to maxLen and adds "..." if truncated
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// parseTime parses an ISO timestamp
func parseTime(ts string) time.Time {
	ts = strings.Replace(ts, "+00:00", "Z", 1)
	t, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		t, _ = time.Parse(time.RFC3339, ts)
	}
	return t
}

// IsPhoenixAvailable checks if Phoenix is reachable
func (c *PhoenixClient) IsPhoenixAvailable(ctx context.Context) bool {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/health", nil)
	if err != nil {
		return false
	}

	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}
