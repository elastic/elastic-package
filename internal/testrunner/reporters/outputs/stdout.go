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

func reportToSTDOUT(pkg, report string, _ testrunner.TestReportFormat) error {
	fmt.Printf("--- Test results for package: %s - START ---\n", pkg)
	fmt.Println(report)
	fmt.Printf("--- Test results for package: %s - END   ---\n", pkg)
	fmt.Println("Done")

	return nil
}
