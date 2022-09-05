// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package outputs

import (
	"fmt"

	"github.com/elastic/elastic-package/internal/benchrunner"
)

func init() {
	benchrunner.RegisterReporterOutput(ReportOutputSTDOUT, reportToSTDOUT)
}

const (
	// ReportOutputSTDOUT reports benchmark results to STDOUT
	ReportOutputSTDOUT benchrunner.BenchReportOutput = "stdout"
)

func reportToSTDOUT(pkg, report string, _ benchrunner.BenchReportFormat) error {
	fmt.Printf("--- Benchmark results for package: %s - START ---\n", pkg)
	fmt.Println(report)
	fmt.Printf("--- Benchmark results for package: %s - END   ---\n", pkg)
	fmt.Println("Done")
	return nil
}
