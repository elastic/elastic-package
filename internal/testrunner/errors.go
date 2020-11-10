// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package testrunner

import "fmt"

// ErrTestCaseFailed represents a test case failure result
type ErrTestCaseFailed struct {
	Reason  string
	Details string
}

// Error returns the message detailing the test case failure.
func (e ErrTestCaseFailed) Error() string {
	return fmt.Sprintf("test case failed: %s", e.Reason)
}
