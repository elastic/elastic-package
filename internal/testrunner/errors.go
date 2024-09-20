// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package testrunner

import (
	"fmt"
)

// ErrTestCaseFailed represents a test case failure result
type ErrTestCaseFailed struct {
	Reason  string
	Details string
}

// Error returns the message detailing the test case failure.
func (e ErrTestCaseFailed) Error() string {
	return fmt.Sprintf("test case failed: %s", e.Reason)
}

// ErrTestCaseConstraintsSkip represents a test case skipped due to version constraints specified in the config
type ErrTestCaseConstraintsSkip struct {
}

// Error returns the message detailing the test case failure.
func (e ErrTestCaseConstraintsSkip) Error() string {
	return fmt.Sprintf("test case skipped validation against expected output due to non matching constraints")
}
