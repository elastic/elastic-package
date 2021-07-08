// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package formats

import (
	"fmt"
	"strings"

	"github.com/jedib0t/go-pretty/table"

	"github.com/elastic/elastic-package/internal/testrunner"
)

func init() {
	testrunner.RegisterReporterFormat(ReportFormatHuman, reportHumanFormat)
}

const (
	// ReportFormatHuman reports test results in a human-readable format
	ReportFormatHuman testrunner.TestReportFormat = "human"
)

func reportHumanFormat(results []testrunner.TestResult) (string, error) {
	if len(results) == 0 {
		return "No test results", nil
	}

	var report strings.Builder

	headerPrinted := false
	for _, r := range results {
		if r.FailureMsg == "" {
			continue
		}

		if !headerPrinted {
			report.WriteString("FAILURE DETAILS:\n")
			headerPrinted = true
		}

		detail := fmt.Sprintf("%s/%s %s:\n%s\n", r.Package, r.DataStream, r.Name, r.FailureDetails)
		report.WriteString(detail)
	}
	if headerPrinted {
		report.WriteString("\n\n")
	}

	t := table.NewWriter()
	t.AppendHeader(table.Row{"Package", "Data stream", "Test type", "Test name", "Result", "Time elapsed"})

	for _, r := range results {
		var result string
		if r.ErrorMsg != "" {
			result = fmt.Sprintf("ERROR: %s", r.ErrorMsg)
		} else if r.FailureMsg != "" {
			result = fmt.Sprintf("FAIL: %s", r.FailureMsg)
		} else if r.Skipped != nil {
			result = r.Skipped.String()
		} else {
			result = "PASS"
		}

		t.AppendRow(table.Row{r.Package, r.DataStream, r.TestType, r.Name, result, r.TimeElapsed})
	}

	t.SetStyle(table.StyleRounded)

	report.WriteString(t.Render())

	return report.String(), nil
}
