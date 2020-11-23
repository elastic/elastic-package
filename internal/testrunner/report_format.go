// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package testrunner

import "fmt"

// TestReportFormat represents a test report format
type TestReportFormat string

// ReportFormatFunc defines the report formatter function.
type ReportFormatFunc func(results []TestResult) (string, error)

var reportFormatters = map[TestReportFormat]ReportFormatFunc{}

// RegisterReporterFormat registers a test report formatter.
func RegisterReporterFormat(name TestReportFormat, formatFunc ReportFormatFunc) {
	reportFormatters[name] = formatFunc
}

// FormatReport delegates formatting of test results to the registered test report formatter
func FormatReport(name TestReportFormat, results []TestResult) (string, error) {
	reportFunc, defined := reportFormatters[name]
	if !defined {
		return "", fmt.Errorf("unregistered test report format: %s", name)
	}

	return reportFunc(results)
}
