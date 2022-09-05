// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package benchrunner

import (
	"fmt"
)

// BenchReportOutput represents an output for a test report
type BenchReportOutput string

// ReportOutputFunc defines the report writer function.
type ReportOutputFunc func(pkg, report string, format BenchReportFormat) error

var reportOutputs = map[BenchReportOutput]ReportOutputFunc{}

// RegisterReporterOutput registers a test report output.
func RegisterReporterOutput(name BenchReportOutput, outputFunc ReportOutputFunc) {
	reportOutputs[name] = outputFunc
}

// WriteReport delegates writing of test results to the registered test report output
func WriteReport(pkg string, name BenchReportOutput, report string, format BenchReportFormat) error {
	outputFunc, defined := reportOutputs[name]
	if !defined {
		return fmt.Errorf("unregistered test report output: %s", name)
	}
	return outputFunc(pkg, report, format)
}
