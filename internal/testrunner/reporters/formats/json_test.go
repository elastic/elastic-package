// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package formats

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/elastic-package/internal/testrunner"
)

func TestReportJSONFormatFlaky(t *testing.T) {
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

	report, err := reportJSONFormat(results)
	require.NoError(t, err)

	var parsed []map[string]any
	require.NoError(t, json.Unmarshal([]byte(report), &parsed))
	require.Len(t, parsed, 2)

	// The flaky test is still reported as a plain PASS, with the previous
	// failures available in a separate field, mirroring the xUnit format.
	assert.Equal(t, "PASS", parsed[0]["result"])
	assert.Equal(t, "attempt 1 of 2 failed during setup: service is unhealthy", parsed[0]["flaky_details"])

	// A stable pass carries no flaky details at all.
	assert.Equal(t, "PASS", parsed[1]["result"])
	assert.NotContains(t, parsed[1], "flaky_details")
}
