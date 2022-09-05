// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package benchrunner

import "fmt"

// BenchReportFormat represents a test report format
type BenchReportFormat string

// ReportFormatFunc defines the report formatter function.
type ReportFormatFunc func(results []BenchResult) ([]string, error)

var reportFormatters = map[BenchReportFormat]ReportFormatFunc{}

// RegisterReporterFormat registers a test report formatter.
func RegisterReporterFormat(name BenchReportFormat, formatFunc ReportFormatFunc) {
	reportFormatters[name] = formatFunc
}

// FormatReport delegates formatting of test results to the registered test report formatter.
func FormatReport(name BenchReportFormat, results []BenchResult) (benchmarkReports []string, err error) {
	reportFunc, defined := reportFormatters[name]
	if !defined {
		return nil, fmt.Errorf("unregistered test report format: %s", name)
	}

	return reportFunc(results)
}
