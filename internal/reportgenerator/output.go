// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package reportgenerator

import (
	"fmt"
)

// ReportOutput represents an output for a benchmark report
type ReportOutput string

// ReportOutputFunc defines the writer function.
type ReportOutputFunc func(result []byte, format string) error

var reportOutputs = map[ReportOutput]ReportOutputFunc{}

// RegisterReportOutput registers a benchmark output.
func RegisterReportOutput(name ReportOutput, outputFunc ReportOutputFunc) {
	reportOutputs[name] = outputFunc
}

// WriteReport delegates writing of benchmark reports to the registered benchmark output
func WriteReport(name ReportOutput, report []byte, format string) error {
	outputFunc, defined := reportOutputs[name]
	if !defined {
		return fmt.Errorf("unregistered benchmark output: %s", name)
	}
	return outputFunc(report, format)
}
