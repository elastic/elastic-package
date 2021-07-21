// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package testrunner

// WriteCoverage function calculates test coverage for the given package.
// It requires to execute tests for all data streams (same test type), so the coverage can be calculated properly.
// The function includes following test types in the coverage report - pipeline and system.
func WriteCoverage(pkg string, testType TestType, results []TestResult) error {

	return nil
}
