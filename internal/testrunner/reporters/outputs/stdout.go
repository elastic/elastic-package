// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package outputs

import (
	"fmt"

	"github.com/elastic/elastic-package/internal/testrunner"
)

func init() {
	testrunner.RegisterReporterOutput(ReportOutputSTDOUT, reportToSTDOUT)
}

const (
	// ReportOutputSTDOUT reports test results to STDOUT
	ReportOutputSTDOUT testrunner.TestReportOutput = "stdout"
)

func reportToSTDOUT(pkg, report string, _ testrunner.TestReportFormat, ttype testrunner.TestReportType) error {
	reportType := "Test"
	if ttype == testrunner.ReportTypeBench {
		reportType = "Benchmark"
	}
	fmt.Printf("--- %s results for package: %s - START ---\n", reportType, pkg)
	fmt.Println(report)
	fmt.Printf("--- %s results for package: %s - END   ---\n", reportType, pkg)
	fmt.Println("Done")

	return nil
}
