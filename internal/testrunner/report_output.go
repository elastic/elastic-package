// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package testrunner

import (
	"fmt"
)

// TestReportOutput represents an output for a test report
type TestReportOutput string

// ReportOutputFunc defines the report writer function.
type ReportOutputFunc func(pkg, report string, format TestReportFormat) error

var reportOutputs = map[TestReportOutput]ReportOutputFunc{}

// RegisterReporterOutput registers a test report output.
func RegisterReporterOutput(name TestReportOutput, outputFunc ReportOutputFunc) {
	reportOutputs[name] = outputFunc
}

// WriteReport delegates writing of test results to the registered test report output
func WriteReport(pkg string, name TestReportOutput, report string, format TestReportFormat) error {
	outputFunc, defined := reportOutputs[name]
	if !defined {
		return fmt.Errorf("unregistered test report output: %s", name)
	}

	return outputFunc(pkg, report, format)
}
