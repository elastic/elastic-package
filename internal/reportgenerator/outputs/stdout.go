// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package outputs

import (
	"fmt"

	"github.com/elastic/elastic-package/internal/reportgenerator"
)

func init() {
	reportgenerator.RegisterReportOutput(OutputSTDOUT, writeToSTDOUT)
}

const (
	// OutputSTDOUT reports to STDOUT
	OutputSTDOUT reportgenerator.ReportOutput = "stdout"
)

func writeToSTDOUT(report []byte, _ string) error {
	fmt.Println(string(report))
	return nil
}
