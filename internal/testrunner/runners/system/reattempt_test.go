// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package system

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/elastic-package/internal/testrunner"
)

// attemptOutcome describes what a single test attempt should produce in the
// stub used by these tests.
type attemptOutcome struct {
	result testrunner.TestResult
	runErr error
	tdErr  error
}

func stubAttempts(t *testing.T, outcomes []attemptOutcome, calls *int) func() ([]testrunner.TestResult, error, error) {
	t.Helper()
	return func() ([]testrunner.TestResult, error, error) {
		require.Less(t, *calls, len(outcomes), "more attempts than expected")
		outcome := outcomes[*calls]
		*calls++
		return []testrunner.TestResult{outcome.result}, outcome.runErr, outcome.tdErr
	}
}

func TestRunWithSetupReattempts(t *testing.T) {
	setupErr := errSetupFailed{err: errors.New("service is unhealthy: container exited with code 143")}
	setupResult := testrunner.TestResult{Name: "test", ErrorMsg: setupErr.Error()}
	passResult := testrunner.TestResult{Name: "test"}
	failedResult := testrunner.TestResult{Name: "test", FailureMsg: "could not find the expected hits"}

	cases := []struct {
		title           string
		reattempts      int
		outcomes        []attemptOutcome
		expectedCalls   int
		expectedErr     string
		expectedFlaky   bool
		expectedResults func(t *testing.T, results []testrunner.TestResult)
	}{
		{
			title:         "pass on first attempt",
			reattempts:    1,
			outcomes:      []attemptOutcome{{result: passResult}},
			expectedCalls: 1,
		},
		{
			title:      "setup failure then pass is marked flaky",
			reattempts: 1,
			outcomes: []attemptOutcome{
				{result: setupResult, runErr: setupErr},
				{result: passResult},
			},
			expectedCalls: 2,
			expectedFlaky: true,
		},
		{
			title:      "setup failures until attempts are exhausted",
			reattempts: 2,
			outcomes: []attemptOutcome{
				{result: setupResult, runErr: setupErr},
				{result: setupResult, runErr: setupErr},
				{result: setupResult, runErr: setupErr},
			},
			expectedCalls: 3,
			expectedResults: func(t *testing.T, results []testrunner.TestResult) {
				// The last failure is reported as an error entry, without
				// aborting the run, and it is not marked as flaky.
				require.Len(t, results, 1)
				assert.NotEmpty(t, results[0].ErrorMsg)
				assert.Empty(t, results[0].FlakyMsg)
			},
		},
		{
			title:      "validation failure is never re-attempted",
			reattempts: 3,
			outcomes: []attemptOutcome{
				{result: failedResult},
			},
			expectedCalls: 1,
		},
		{
			title:      "re-attempts disabled",
			reattempts: 0,
			outcomes: []attemptOutcome{
				{result: setupResult, runErr: setupErr},
			},
			expectedCalls: 1,
		},
		{
			title:      "hard errors are returned without re-attempt",
			reattempts: 3,
			outcomes: []attemptOutcome{
				{result: passResult, runErr: errors.New("cannot load config")},
			},
			expectedCalls: 1,
			expectedErr:   "cannot load config",
		},
		{
			title:      "no re-attempt if teardown of the failed attempt failed",
			reattempts: 3,
			outcomes: []attemptOutcome{
				{result: setupResult, runErr: setupErr, tdErr: errors.New("could not remove policy")},
			},
			expectedCalls: 1,
			expectedErr:   "failed to tear down runner",
		},
		{
			title:      "teardown failure after passing test is returned",
			reattempts: 1,
			outcomes: []attemptOutcome{
				{result: passResult, tdErr: errors.New("could not remove policy")},
			},
			expectedCalls: 1,
			expectedErr:   "failed to tear down runner",
		},
	}

	for _, c := range cases {
		t.Run(c.title, func(t *testing.T) {
			var calls int
			results, err := runWithSetupReattempts(context.Background(), c.reattempts, stubAttempts(t, c.outcomes, &calls))

			assert.Equal(t, c.expectedCalls, calls)
			if c.expectedErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), c.expectedErr)
			} else {
				assert.NoError(t, err)
			}

			if c.expectedResults != nil {
				c.expectedResults(t, results)
				return
			}

			require.Len(t, results, 1)
			if c.expectedFlaky {
				assert.NotEmpty(t, results[0].FlakyMsg, "test passing after re-attempts should be marked as flaky")
			} else {
				assert.Empty(t, results[0].FlakyMsg)
			}
		})
	}
}

func TestRunWithSetupReattemptsCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	setupErr := errSetupFailed{err: errors.New("context deadline exceeded")}
	var calls int
	attempt := func() ([]testrunner.TestResult, error, error) {
		calls++
		cancel() // The context is cancelled while the attempt runs.
		return []testrunner.TestResult{{Name: "test", ErrorMsg: setupErr.Error()}}, setupErr, nil
	}

	results, err := runWithSetupReattempts(ctx, 3, attempt)
	assert.NoError(t, err)
	assert.Equal(t, 1, calls, "cancelled context should not be re-attempted")
	require.Len(t, results, 1)
	assert.Empty(t, results[0].FlakyMsg)
}

func TestErrSetupFailedWrapping(t *testing.T) {
	cause := errors.New("service is unhealthy")
	err := fmt.Errorf("attempt failed: %w", errSetupFailed{err: cause})

	var setupErr errSetupFailed
	require.ErrorAs(t, err, &setupErr)
	assert.ErrorIs(t, err, cause)
	assert.Equal(t, cause, setupErr.Unwrap())
}
