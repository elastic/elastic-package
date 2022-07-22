// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package formats

import (
	"fmt"
	"strings"

	"github.com/jedib0t/go-pretty/table"
	"github.com/jedib0t/go-pretty/text"

	"github.com/elastic/elastic-package/internal/testrunner"
)

func init() {
	testrunner.RegisterReporterFormat(ReportFormatHuman, reportHumanFormat)
}

const (
	// ReportFormatHuman reports test results in a human-readable format
	ReportFormatHuman testrunner.TestReportFormat = "human"
)

func reportHumanFormat(results []testrunner.TestResult) (string, []string, error) {
	if len(results) == 0 {
		return "No test results", nil, nil
	}

	var benchmarks []testrunner.BenchmarkResult
	for _, r := range results {
		if r.Benchmark != nil {
			benchmarks = append(benchmarks, *r.Benchmark)
		}
	}

	testFmtd, err := reportHumanFormatTest(results)
	if err != nil {
		return "", nil, err
	}
	benchFmtd, err := reportHumanFormatBenchmark(benchmarks)
	if err != nil {
		return "", nil, err
	}
	return testFmtd, benchFmtd, nil
}

func reportHumanFormatTest(results []testrunner.TestResult) (string, error) {
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

func reportHumanFormatBenchmark(benchmarks []testrunner.BenchmarkResult) ([]string, error) {
	var textReports []string
	for _, b := range benchmarks {
		var report strings.Builder
		if len(b.Parameters) > 0 {
			report.WriteString(renderBenchmarkTable("parameters", b.Parameters) + "\n")
		}
		for _, test := range b.Tests {
			report.WriteString(renderBenchmarkTable(test.Name, test.Results) + "\n")
		}
		textReports = append(textReports, report.String())
	}
	return textReports, nil
}

func renderBenchmarkTable(title string, values []testrunner.BenchmarkValue) string {
	t := table.NewWriter()
	t.SetStyle(table.StyleRounded)
	t.SetTitle(title)
	t.SetColumnConfigs([]table.ColumnConfig{
		{
			Number: 2,
			Align:  text.AlignRight,
		},
	})
	for _, r := range values {
		t.AppendRow(table.Row{r.Name, r.PrettyValue()})
	}
	return t.Render()
}
