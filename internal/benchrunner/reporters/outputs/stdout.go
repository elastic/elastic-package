// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package outputs

import (
	"fmt"

	"github.com/elastic/elastic-package/internal/benchrunner/reporters"
)

func init() {
	reporters.RegisterOutput(ReportOutputSTDOUT, reportToSTDOUT)
}

const (
	// ReportOutputSTDOUT reports benchmark results to STDOUT
	ReportOutputSTDOUT reporters.Output = "stdout"
)

func reportToSTDOUT(report reporters.Reportable) error {
	fmt.Printf("--- Benchmark results for package: %s - START ---\n", report.Package())
	fmt.Println(string(report.Report()))
	fmt.Printf("--- Benchmark results for package: %s - END   ---\n", report.Package())
	fmt.Println("Done")
	return nil
}
