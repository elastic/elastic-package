// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

// Package tracing provides OpenTelemetry tracing and Phoenix integration.
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
	TotalSpans           int                `json:"total_spans"`
	TotalLatencyMs       float64            `json:"total_latency_ms"`
	TotalPromptTokens    int                `json:"total_prompt_tokens"`
	TotalCompletionTokens int               `json:"total_completion_tokens"`
	TotalTokens          int                `json:"total_tokens"`
	LLMCalls             int                `json:"llm_calls"`
	AgentCalls           []AgentCallSummary `json:"agent_calls"`
	SignificantEvents    []SignificantEvent `json:"significant_events"`
}

// AgentCallSummary summarizes an agent's activity
type AgentCallSummary struct {
	AgentName     string  `json:"agent_name"`
	CallCount     int     `json:"call_count"`
	TotalLatencyMs float64 `json:"total_latency_ms"`
	TotalTokens   int     `json:"total_tokens"`
	Approved      *bool   `json:"approved,omitempty"`
	Score         *int    `json:"score,omitempty"`
}

// SignificantEvent represents an important event during documentation generation
type SignificantEvent struct {
	Timestamp   time.Time `json:"timestamp"`
	Type        string    `json:"type"` // "llm_call", "validation", "iteration", "error"
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

// generateSummary creates a summary of the trace data
func (c *PhoenixClient) generateSummary(traces *SessionTraces) *TraceSummary {
	summary := &TraceSummary{
		AgentCalls:        []AgentCallSummary{},
		SignificantEvents: []SignificantEvent{},
	}

	agentStats := make(map[string]*AgentCallSummary)

	for _, trace := range traces.Traces {
		summary.TotalLatencyMs += trace.LatencyMs

		for _, span := range trace.Spans {
			summary.TotalSpans++
			summary.TotalPromptTokens += span.TokenCountPrompt
			summary.TotalCompletionTokens += span.TokenCountCompletion
			summary.TotalTokens += span.TokenCountTotal

			// Track LLM calls
			if span.Name == "call_llm" || strings.Contains(span.Name, "llm") {
				summary.LLMCalls++
			}

			// Track agent calls and add as significant events
			if strings.HasPrefix(span.Name, "agent:") {
				agentName := strings.TrimPrefix(span.Name, "agent:")
				if _, ok := agentStats[agentName]; !ok {
					agentStats[agentName] = &AgentCallSummary{
						AgentName: agentName,
					}
				}
				agentStats[agentName].CallCount++
				agentStats[agentName].TotalLatencyMs += span.LatencyMs
				agentStats[agentName].TotalTokens += span.TokenCountTotal

				// Create event for agent call
				event := SignificantEvent{
					Timestamp:   span.StartTime,
					Type:        "agent",
					Agent:       agentName,
					Description: fmt.Sprintf("Agent: %s", agentName),
					LatencyMs:   span.LatencyMs,
					Tokens:      span.TokenCountTotal,
					Severity:    "info",
				}

				// Check for validation results in output
				if span.Output != "" {
					var result map[string]interface{}
					if err := json.Unmarshal([]byte(span.Output), &result); err == nil {
						if approved, ok := result["approved"].(bool); ok {
							agentStats[agentName].Approved = &approved
							if approved {
								event.Details = "approved"
							} else {
								event.Details = "rejected"
								event.Severity = "warning"
							}
						}
						if valid, ok := result["valid"].(bool); ok {
							agentStats[agentName].Approved = &valid
							if valid {
								event.Details = "valid"
							} else {
								event.Details = "invalid"
								event.Severity = "warning"
							}
						}
						if score, ok := result["score"].(float64); ok {
							scoreInt := int(score)
							agentStats[agentName].Score = &scoreInt
							event.Details = fmt.Sprintf("score: %d", scoreInt)
						}
						if feedback, ok := result["feedback"].(string); ok && feedback != "" {
							if event.Details != "" {
								event.Details += " - "
							}
							// Truncate long feedback
							if len(feedback) > 100 {
								feedback = feedback[:100] + "..."
							}
							event.Details += feedback
						}
					}
				}

				summary.SignificantEvents = append(summary.SignificantEvents, event)
			}

			// Track workflow iterations
			if span.Name == "workflow:section" || span.Name == "workflow:staged" {
				event := SignificantEvent{
					Timestamp:   span.StartTime,
					Type:        "workflow",
					Description: fmt.Sprintf("Workflow: %s", span.Name),
					LatencyMs:   span.LatencyMs,
					Severity:    "info",
				}

				// Check attributes for iteration info
				if span.Attributes != nil {
					if workflow, ok := span.Attributes["workflow"].(map[string]interface{}); ok {
						if iterations, ok := workflow["total_iterations"].(float64); ok {
							event.Description = fmt.Sprintf("Workflow completed in %.0f iterations", iterations)
						}
						if approved, ok := workflow["approved"].(bool); ok {
							if approved {
								event.Severity = "info"
								event.Details = "Approved"
							} else {
								event.Severity = "warning"
								event.Details = "Not approved after max iterations"
							}
						}
					}
				}

				summary.SignificantEvents = append(summary.SignificantEvents, event)
			}

			// Track root session span
			if strings.HasPrefix(span.Name, "test:") || strings.HasPrefix(span.Name, "doc:") {
				event := SignificantEvent{
					Timestamp:   span.StartTime,
					Type:        "session",
					Description: fmt.Sprintf("Session: %s", span.Name),
					LatencyMs:   span.LatencyMs,
					Tokens:      span.TokenCountTotal,
					Severity:    "info",
				}
				summary.SignificantEvents = append(summary.SignificantEvents, event)
			}

			// Track errors
			if span.StatusCode == "ERROR" {
				event := SignificantEvent{
					Timestamp:   span.StartTime,
					Type:        "error",
					Agent:       span.Name,
					Description: fmt.Sprintf("Error in %s", span.Name),
					Severity:    "error",
					Details:     span.StatusMessage,
				}
				summary.SignificantEvents = append(summary.SignificantEvents, event)
			}
		}
	}

	// Convert agent stats to slice
	for _, stats := range agentStats {
		summary.AgentCalls = append(summary.AgentCalls, *stats)
	}

	// Sort agent calls by name
	sort.Slice(summary.AgentCalls, func(i, j int) bool {
		return summary.AgentCalls[i].AgentName < summary.AgentCalls[j].AgentName
	})

	// Sort events by timestamp
	sort.Slice(summary.SignificantEvents, func(i, j int) bool {
		return summary.SignificantEvents[i].Timestamp.Before(summary.SignificantEvents[j].Timestamp)
	})

	return summary
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

