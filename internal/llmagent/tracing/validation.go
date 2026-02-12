// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package tracing

import (
	"context"
	"encoding/json"
	"fmt"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// Validation-specific attribute keys for staged validation workflow tracing
const (
	// Validation stage attributes
	AttrValidationStage    = "validation.stage"
	AttrValidationPassed   = "validation.passed"
	AttrValidationScore    = "validation.score"
	AttrValidationIssues   = "validation.issues_count"
	AttrValidationWarnings = "validation.warnings_count"
	AttrValidationSource   = "validation.source" // "static" or "llm"

	// Generation iteration attributes
	AttrGenerationIteration  = "generation.iteration"
	AttrGenerationFeedback   = "generation.feedback_applied"
	AttrGenerationContentLen = "generation.content_length"

	// Package context attributes
	AttrPackageName      = "package.name"
	AttrPackageTitle     = "package.title"
	AttrPackageVersion   = "package.version"
	AttrDataStreamsCount = "package.data_streams_count"
	AttrFieldsCount      = "package.fields_count"
)

// ValidationStageResult holds the result of a validation stage for tracing
type ValidationStageResult struct {
	Stage         string   `json:"stage"`
	Passed        bool     `json:"passed"`
	Score         int      `json:"score"`
	IssuesCount   int      `json:"issues_count"`
	WarningsCount int      `json:"warnings_count"`
	Source        string   `json:"source"` // "static", "llm", or "merged"
	Issues        []string `json:"issues,omitempty"`
}

// StartValidationStageSpan starts a span for a validation stage
func StartValidationStageSpan(ctx context.Context, stageName string, iteration int) (context.Context, trace.Span) {
	attrs := []attribute.KeyValue{
		attribute.String(AttrOpenInferenceSpanKind, SpanKindChain),
		attribute.String(AttrValidationStage, stageName),
		attribute.Int(AttrGenerationIteration, iteration),
	}
	attrs = append(attrs, sessionAttributes(ctx)...)
	return Tracer().Start(ctx, "validation:"+stageName, trace.WithAttributes(attrs...))
}

// EndValidationStageSpan records the validation result and ends the span
func EndValidationStageSpan(span trace.Span, result *ValidationStageResult) {
	if result == nil {
		span.SetStatus(codes.Error, "nil validation result")
		span.End()
		return
	}

	span.SetAttributes(
		attribute.Bool(AttrValidationPassed, result.Passed),
		attribute.Int(AttrValidationScore, result.Score),
		attribute.Int(AttrValidationIssues, result.IssuesCount),
		attribute.Int(AttrValidationWarnings, result.WarningsCount),
		attribute.String(AttrValidationSource, result.Source),
	)

	// Record issues as JSON if present
	if len(result.Issues) > 0 {
		if issuesJSON, err := json.Marshal(result.Issues); err == nil {
			span.SetAttributes(attribute.String("validation.issues", string(issuesJSON)))
		}
	}

	if result.Passed {
		span.SetStatus(codes.Ok, "validation passed")
	} else {
		span.SetStatus(codes.Ok, "validation failed") // Still OK status, just not passed
	}

	span.End()
}

// StartStaticValidationSpan starts a span for static (non-LLM) validation
func StartStaticValidationSpan(ctx context.Context, stageName string) (context.Context, trace.Span) {
	attrs := []attribute.KeyValue{
		attribute.String(AttrOpenInferenceSpanKind, SpanKindTool),
		attribute.String(AttrValidationStage, stageName),
		attribute.String(AttrValidationSource, "static"),
	}
	attrs = append(attrs, sessionAttributes(ctx)...)
	return Tracer().Start(ctx, "static_validation:"+stageName, trace.WithAttributes(attrs...))
}

// StartLLMValidationSpan starts a span for LLM-based validation
func StartLLMValidationSpan(ctx context.Context, stageName string, modelID string) (context.Context, trace.Span) {
	attrs := []attribute.KeyValue{
		attribute.String(AttrOpenInferenceSpanKind, SpanKindLLM),
		attribute.String(AttrValidationStage, stageName),
		attribute.String(AttrValidationSource, "llm"),
		attribute.String(AttrGenAIRequestModel, modelID),
		attribute.String(AttrLLMModelName, modelID),
	}
	attrs = append(attrs, sessionAttributes(ctx)...)
	return Tracer().Start(ctx, "llm_validation:"+stageName, trace.WithAttributes(attrs...))
}

// EndValidationSpan ends a validation span with the result details.
// Nil spans are safely ignored (occurs when tracing is disabled).
func EndValidationSpan(span trace.Span, passed bool, score int, issueCount int, issues []string) {
	if span == nil {
		return
	}

	span.SetAttributes(
		attribute.Bool(AttrValidationPassed, passed),
		attribute.Int(AttrValidationScore, score),
		attribute.Int(AttrValidationIssues, issueCount),
	)

	// Record issues as JSON if present (limited to first 10 to avoid large spans)
	maxIssues := 10
	if len(issues) > maxIssues {
		issues = issues[:maxIssues]
	}

	// Store validation results in the output field (Phoenix captures this)
	output := map[string]interface{}{
		"valid":       passed,
		"score":       score,
		"issue_count": issueCount,
		"issues":      issues,
	}
	if outputJSON, err := json.Marshal(output); err == nil {
		span.SetAttributes(attribute.String("output.value", string(outputJSON)))
	}

	if passed {
		span.SetStatus(codes.Ok, "validation passed")
	} else {
		span.SetStatus(codes.Ok, fmt.Sprintf("validation failed with %d issues", issueCount))
	}

	span.End()
}

// EndValidationSpanWithError ends a validation span with an error.
// Nil spans are safely ignored (occurs when tracing is disabled).
func EndValidationSpanWithError(span trace.Span, err error) {
	if span == nil {
		return
	}

	span.SetAttributes(
		attribute.Bool(AttrValidationPassed, false),
		attribute.String("error.message", err.Error()),
	)
	span.SetStatus(codes.Error, err.Error())
	span.RecordError(err)
	span.End()
}

// StartGenerationIterationSpan starts a span for a generation iteration
func StartGenerationIterationSpan(ctx context.Context, iteration int, hasFeedback bool) (context.Context, trace.Span) {
	attrs := []attribute.KeyValue{
		attribute.String(AttrOpenInferenceSpanKind, SpanKindAgent),
		attribute.Int(AttrGenerationIteration, iteration),
		attribute.Bool(AttrGenerationFeedback, hasFeedback),
	}
	attrs = append(attrs, sessionAttributes(ctx)...)
	return Tracer().Start(ctx, "generation_iteration", trace.WithAttributes(attrs...))
}

// RecordGenerationContent records the generated content length on a span
func RecordGenerationContent(span trace.Span, contentLen int) {
	span.SetAttributes(attribute.Int(AttrGenerationContentLen, contentLen))
}

// RecordPackageContext records package metadata on a span
func RecordPackageContext(span trace.Span, name, title, version string, dataStreams, fields int) {
	span.SetAttributes(
		attribute.String(AttrPackageName, name),
		attribute.String(AttrPackageTitle, title),
		attribute.String(AttrPackageVersion, version),
		attribute.Int(AttrDataStreamsCount, dataStreams),
		attribute.Int(AttrFieldsCount, fields),
	)
}
