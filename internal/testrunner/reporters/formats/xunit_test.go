// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package formats

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/elastic-package/internal/testrunner"
)

func TestReportXUnitFormatFlaky(t *testing.T) {
	results := []testrunner.TestResult{
		{
			Name:        "default",
			Package:     "network_traffic",
			DataStream:  "dns",
			TestType:    "system",
			TimeElapsed: 5 * time.Second,
			FlakyMsg:    "attempt 1 of 2 failed during setup: service is unhealthy",
		},
		{
			Name:       "other",
			Package:    "network_traffic",
			DataStream: "http",
			TestType:   "system",
		},
	}

	report, err := reportXUnitFormat(results)
	require.NoError(t, err)

	assert.Contains(t, report, `<flakyFailure message="attempt 1 of 2 failed during setup: service is unhealthy"`)
	// The flaky test is still reported as passed, not as failed or errored.
	assert.NotContains(t, report, "failures=")
	assert.NotContains(t, report, "errors=")
}
