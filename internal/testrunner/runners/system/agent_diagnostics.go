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
	"time"

	"github.com/elastic/elastic-package/internal/agentdeployer"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/multierror"
	"github.com/elastic/elastic-package/internal/stack"
	"github.com/elastic/elastic-package/internal/testrunner"
)

// maxAgentDiagnosticErrors caps unique indexing/agent errors attached to a
// missing-hits failure so Buildkite junit annotations stay within size limits.
const maxAgentDiagnosticErrors = 5

// indexingErrorPatterns matches Elasticsearch indexing failures in agent logs.
// These are a subset of errorPatterns focused on explaining zero-hit failures.
var indexingErrorPatterns = []logsRegexp{
	{
		includes: regexp.MustCompile(`^Cannot index event publisher.Event`),
		excludes: []*regexp.Regexp{
			regexp.MustCompile(`action \[indices:data\/write\/bulk\[s\]\] is unauthorized for API key id \[.*\] of user \[.*\] on indices \[.*\], this action is granted by the index privileges \[.*\]`),
		},
	},
	{
		includes: regexp.MustCompile(`^failed to index document`),
	},
}

// enrichWithAgentDiagnostics attaches agent indexing-failure details to an
// ErrTestCaseFailed when an independent agent is still available. Best-effort:
// collection failures leave the original error unchanged.
func (r *tester) enrichWithAgentDiagnostics(ctx context.Context, scenario *scenarioTest, err error) error {
	var testErr testrunner.ErrTestCaseFailed
	if !errors.As(err, &testErr) {
		return err
	}
	if scenario == nil || scenario.agent == nil {
		return err
	}

	details := r.collectAgentIndexingDiagnostics(ctx, scenario.agent, scenario.startTestTime)
	if details == "" {
		return err
	}
	if testErr.Details != "" {
		testErr.Details = testErr.Details + "\n" + details
	} else {
		testErr.Details = details
	}
	return testErr
}

// collectAgentIndexingDiagnostics scrapes independent-agent logs for indexing
// failures. Prefers internal NDJSON logs; falls back to docker-compose stdout.
func (r *tester) collectAgentIndexingDiagnostics(ctx context.Context, agent agentdeployer.DeployedAgent, startTime time.Time) string {
	var matched []stack.LogLine

	collect := func(log stack.LogLine) error {
		if matchLogPatterns(log, indexingErrorPatterns) {
			matched = append(matched, log)
		}
		return nil
	}

	internalErr := r.collectFromInternalLogs(agent, startTime, collect)
	if internalErr != nil {
		logger.Debugf("agent internal log diagnostics unavailable: %v", internalErr)
	}
	if internalErr != nil || len(matched) == 0 {
		if err := r.collectFromComposeLogs(ctx, agent, startTime, collect); err != nil {
			logger.Debugf("agent compose log diagnostics unavailable: %v", err)
		}
	}

	return formatDiagnosticErrors(matched, maxAgentDiagnosticErrors)
}

func (r *tester) collectFromInternalLogs(agent agentdeployer.DeployedAgent, startTime time.Time, process func(stack.LogLine) error) error {
	tmpParent, err := os.MkdirTemp("", "elastic-agent-internal-logs-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpParent)

	destDir := filepath.Join(tmpParent, "logs")
	if err := agent.CopyInternalLogs(destDir); err != nil {
		return err
	}

	// docker cp of a directory may nest an extra folder when the destination
	// parent exists, or place files directly in destDir.
	logDir := destDir
	if entries, err := os.ReadDir(destDir); err == nil {
		if len(entries) == 1 && entries[0].IsDir() {
			logDir = filepath.Join(destDir, entries[0].Name())
		}
	}

	return stack.ParseNDJSONLogsDir(logDir, startTime, process)
}

func (r *tester) collectFromComposeLogs(ctx context.Context, agent agentdeployer.DeployedAgent, startTime time.Time, process func(stack.LogLine) error) error {
	output, err := agent.Logs(ctx, startTime)
	if err != nil {
		return err
	}
	if len(output) == 0 {
		return nil
	}

	f, err := os.CreateTemp("", "elastic-agent.compose.logs")
	if err != nil {
		return err
	}
	path := f.Name()
	defer os.Remove(path)

	if _, err := f.Write(output); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}

	return stack.ParseLogs(stack.ParseLogsOptions{
		LogsFilePath: path,
		StartTime:    startTime,
	}, process)
}

func matchLogPatterns(log stack.LogLine, patterns []logsRegexp) bool {
	for _, pattern := range patterns {
		if !pattern.includes.MatchString(log.Message) {
			continue
		}
		excluded := false
		for _, excludes := range pattern.excludes {
			if excludes.MatchString(log.Message) {
				excluded = true
				break
			}
		}
		if !excluded {
			return true
		}
	}
	return false
}

func formatDiagnosticErrors(logs []stack.LogLine, maxUnique int) string {
	if len(logs) == 0 {
		return ""
	}

	var multiErr multierror.Error
	seen := make(map[string]struct{})
	for _, log := range logs {
		formatted := log.FormatError()
		dedupeKey := log.ErrorReason
		if dedupeKey == "" {
			dedupeKey = formatted
		}
		if _, ok := seen[dedupeKey]; ok {
			continue
		}
		seen[dedupeKey] = struct{}{}
		multiErr = append(multiErr, fmt.Errorf("%s", formatted))
		if len(multiErr) >= maxUnique {
			break
		}
	}
	return multiErr.Error()
}
