// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package system

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/elastic-package/internal/agentdeployer"
	"github.com/elastic/elastic-package/internal/stack"
	"github.com/elastic/elastic-package/internal/testrunner"
)

const sampleOTelIndexingFailure = `{"log.level":"error","@timestamp":"2026-07-01T17:34:35.220Z","message":"failed to index document","http.response.status_code":400,"error.type":"illegal_argument_exception","error.reason":"field [event.original] cannot reconstruct _source from doc values; every field must be reconstructable from doc values in index using [logsdb_columnar] index mode","ecs.version":"1.6.0"}`

type stubAgent struct {
	internalLogs map[string]string
	composeLogs  []byte
	copyErr      error
	logsErr      error
}

func (s *stubAgent) TearDown(context.Context) error  { return nil }
func (s *stubAgent) Info() agentdeployer.AgentInfo   { return agentdeployer.AgentInfo{} }
func (s *stubAgent) SetInfo(agentdeployer.AgentInfo) {}
func (s *stubAgent) ExitCode(context.Context) (bool, int, error) {
	return false, 0, agentdeployer.ErrNotSupported
}
func (s *stubAgent) Logs(context.Context, time.Time) ([]byte, error) {
	if s.logsErr != nil {
		return nil, s.logsErr
	}
	return s.composeLogs, nil
}
func (s *stubAgent) CopyInternalLogs(destDir string) error {
	if s.copyErr != nil {
		return s.copyErr
	}
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return err
	}
	for name, content := range s.internalLogs {
		if err := os.WriteFile(filepath.Join(destDir, name), []byte(content), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func TestFormatDiagnosticErrors(t *testing.T) {
	logs := []stack.LogLine{
		{
			Message:     "failed to index document",
			ErrorReason: "field [event.original] cannot reconstruct _source from doc values; every field must be reconstructable from doc values in index using [logsdb_columnar] index mode",
			ErrorType:   "illegal_argument_exception",
		},
		{
			Message:     "failed to index document",
			ErrorReason: "field [event.original] cannot reconstruct _source from doc values; every field must be reconstructable from doc values in index using [logsdb_columnar] index mode",
			ErrorType:   "illegal_argument_exception",
		},
		{
			Message:     "failed to index document",
			ErrorReason: "field [other.field] cannot reconstruct _source from doc values",
			ErrorType:   "illegal_argument_exception",
		},
	}

	got := formatDiagnosticErrors(logs, 5)
	assert.Contains(t, got, "[0] failed to index document: field [event.original]", "first unique reason")
	assert.Contains(t, got, "[1] failed to index document: field [other.field]", "second unique reason")
	assert.NotContains(t, got, "[2]", "duplicates should be capped out of listing")
}

func TestFormatDiagnosticErrors_CapsUnique(t *testing.T) {
	var logs []stack.LogLine
	for i := 0; i < 10; i++ {
		logs = append(logs, stack.LogLine{
			Message:     "failed to index document",
			ErrorReason: fmt.Sprintf("reason-%d", i),
			ErrorType:   "illegal_argument_exception",
		})
	}
	got := formatDiagnosticErrors(logs, 3)
	assert.Contains(t, got, "[0]", "first entry")
	assert.Contains(t, got, "[2]", "third entry")
	assert.NotContains(t, got, "[3]", "should cap at maxUnique")
}

func TestMatchLogPatterns_FailedToIndexDocument(t *testing.T) {
	log := stack.LogLine{Message: "failed to index document"}
	assert.True(t, matchLogPatterns(log, indexingErrorPatterns), "should match failed to index document")
	assert.True(t, matchLogPatterns(log, errorPatterns[0].patterns), "should match shared errorPatterns")
}

func TestMatchLogPatterns_InputFailed(t *testing.T) {
	log := stack.LogLine{Message: "Input 'azure-blob-storage' failed with: GET http://svc:10000/... RESPONSE 400"}
	assert.True(t, matchLogPatterns(log, indexingErrorPatterns), "should match permanent input failures")

	failed := stack.LogLine{Message: "Component state changed azure-blob-storage-default (STARTING->FAILED): Permanent: failed to fetch next page"}
	assert.True(t, matchLogPatterns(failed, indexingErrorPatterns), "should match STARTING->FAILED")
}

func TestCollectAgentIndexingDiagnostics_FromInternalLogs(t *testing.T) {
	r := &tester{}
	agent := &stubAgent{
		internalLogs: map[string]string{
			"elastic-agent-20260701.ndjson": sampleOTelIndexingFailure + "\n",
		},
	}
	start, err := time.Parse(time.RFC3339Nano, "2026-07-01T17:00:00.000Z")
	require.NoError(t, err, "parse start time")

	details := r.collectAgentIndexingDiagnostics(context.Background(), agent, start)
	require.NotEmpty(t, details, "expected diagnostics from internal logs")
	assert.Contains(t, details, "failed to index document", "message")
	assert.Contains(t, details, "logsdb_columnar", "error.reason")
	assert.Contains(t, details, "illegal_argument_exception", "error.type")
}

func TestCollectAgentIndexingDiagnostics_FallsBackToComposeLogs(t *testing.T) {
	r := &tester{}
	composeLine := "elastic-agent-1  | " + sampleOTelIndexingFailure + "\n"
	agent := &stubAgent{
		copyErr:     errors.New("docker cp failed"),
		composeLogs: []byte(composeLine),
	}
	start, err := time.Parse(time.RFC3339Nano, "2026-07-01T17:00:00.000Z")
	require.NoError(t, err, "parse start time")

	details := r.collectAgentIndexingDiagnostics(context.Background(), agent, start)
	require.NotEmpty(t, details, "expected diagnostics from compose logs fallback")
	assert.Contains(t, details, "event.original", "error.reason from compose logs")
}

func TestEnrichWithAgentDiagnostics(t *testing.T) {
	r := &tester{}
	agent := &stubAgent{
		internalLogs: map[string]string{
			"elastic-agent-20260701.ndjson": sampleOTelIndexingFailure + "\n",
		},
	}
	start, err := time.Parse(time.RFC3339Nano, "2026-07-01T17:00:00.000Z")
	require.NoError(t, err, "parse start time")

	scenario := &scenarioTest{
		agent:         agent,
		startTestTime: start,
	}
	orig := testrunner.ErrTestCaseFailed{
		Reason: "could not find the expected hits in logs-hashicorp_vault.audit-43464 data stream",
	}

	got := r.enrichWithAgentDiagnostics(context.Background(), scenario, orig)
	var testErr testrunner.ErrTestCaseFailed
	require.True(t, errors.As(got, &testErr), "should remain ErrTestCaseFailed")
	assert.Equal(t, orig.Reason, testErr.Reason, "reason preserved")
	assert.Contains(t, testErr.Details, "logsdb_columnar", "details include indexing reason")
	assert.Equal(t, "test case failed: "+orig.Reason, testErr.Error(), "Error() uses Reason only")
}

func TestEnrichWithAgentDiagnostics_NonTestFailureUnchanged(t *testing.T) {
	r := &tester{}
	orig := errors.New("setup exploded")
	got := r.enrichWithAgentDiagnostics(context.Background(), &scenarioTest{}, orig)
	assert.Equal(t, orig, got, "non-test failures must pass through")
}

func TestXUnitFailureBodyIncludesAgentDiagnostics(t *testing.T) {
	r := &tester{}
	agent := &stubAgent{
		internalLogs: map[string]string{
			"elastic-agent-20260701.ndjson": sampleOTelIndexingFailure + "\n",
		},
	}
	start, err := time.Parse(time.RFC3339Nano, "2026-07-01T17:00:00.000Z")
	require.NoError(t, err, "parse start time")

	enriched := r.enrichWithAgentDiagnostics(context.Background(), &scenarioTest{
		agent:         agent,
		startTestTime: start,
	}, testrunner.ErrTestCaseFailed{
		Reason: "could not find the expected hits in logs-zeek.intel-68376 data stream",
	})

	composer := testrunner.NewResultComposer(testrunner.TestResult{
		TestType:   "system",
		Name:       "default",
		Package:    "zeek",
		DataStream: "intel",
	})
	results, err := composer.WithError(enriched)
	require.NoError(t, err, "WithError should treat ErrTestCaseFailed as a failure, not a runner error")
	require.Len(t, results, 1, "one test result")

	// Mirrors reporters/formats/xunit.go failure body construction.
	failure := results[0].FailureMsg
	if results[0].FailureDetails != "" {
		failure += ": " + results[0].FailureDetails
	}

	assert.Contains(t, failure, "could not find the expected hits in logs-zeek.intel-68376 data stream", "reason in body")
	assert.Contains(t, failure, "failed to index document", "indexing message in body")
	assert.Contains(t, failure, "logsdb_columnar", "rejection reason in body for Buildkite annotation")
}

func TestIndexingErrorPatterns_ExcludesUnauthorizedBulk(t *testing.T) {
	log := stack.LogLine{
		Message: `Cannot index event publisher.Event..., error: fail to execute the bulk action: {...} action [indices:data/write/bulk[s]] is unauthorized for API key id [abc] of user [xyz] on indices [logs-system.auth-default], this action is granted by the index privileges [create_doc,create,index,all]`,
	}
	assert.False(t, matchLogPatterns(log, indexingErrorPatterns), "unauthorized bulk noise should be excluded")
}

func TestFailedToIndexDocumentPatternInErrorPatterns(t *testing.T) {
	found := false
	for _, p := range errorPatterns[0].patterns {
		if p.includes.String() == regexp.MustCompile(`^failed to index document`).String() {
			found = true
			break
		}
	}
	assert.True(t, found, "errorPatterns must include failed to index document")
}
