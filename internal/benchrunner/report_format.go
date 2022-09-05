// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package benchrunner

import "fmt"

// BenchReportFormat represents a benchmark report format
type BenchReportFormat string

// ReportFormatFunc defines the report formatter function.
type ReportFormatFunc func(results []*Result) ([]string, error)

var reportFormatters = map[BenchReportFormat]ReportFormatFunc{}

// RegisterReporterFormat registers a benchmark report formatter.
func RegisterReporterFormat(name BenchReportFormat, formatFunc ReportFormatFunc) {
	reportFormatters[name] = formatFunc
}

// FormatReport delegates formatting of benchmark results to the registered benchmark report formatter.
func FormatReport(name BenchReportFormat, results []*Result) (benchmarkReports []string, err error) {
	reportFunc, defined := reportFormatters[name]
	if !defined {
		return nil, fmt.Errorf("unregistered benchmark report format: %s", name)
	}

	return reportFunc(results)
}
