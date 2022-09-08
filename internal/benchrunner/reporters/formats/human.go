// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package formats

import (
	"strings"

	"github.com/jedib0t/go-pretty/table"
	"github.com/jedib0t/go-pretty/text"

	"github.com/elastic/elastic-package/internal/benchrunner"
)

func init() {
	benchrunner.RegisterReporterFormat(ReportFormatHuman, reportHumanFormat)
}

const (
	// ReportFormatHuman reports benchmark results in a human-readable format
	ReportFormatHuman benchrunner.BenchReportFormat = "human"
)

func reportHumanFormat(results []*benchrunner.Result) ([]string, error) {
	if len(results) == 0 {
		return nil, nil
	}

	var benchmarks []benchrunner.BenchmarkResult
	for _, r := range results {
		if r.Benchmark != nil {
			benchmarks = append(benchmarks, *r.Benchmark)
		}
	}

	benchFormatted, err := reportHumanFormatBenchmark(benchmarks)
	if err != nil {
		return nil, err
	}
	return benchFormatted, nil
}

func reportHumanFormatBenchmark(benchmarks []benchrunner.BenchmarkResult) ([]string, error) {
	var textReports []string
	for _, b := range benchmarks {
		var report strings.Builder
		if len(b.Parameters) > 0 {
			report.WriteString(renderBenchmarkTable("parameters", b.Parameters) + "\n")
		}
		for _, t := range b.Tests {
			report.WriteString(renderBenchmarkTable(t.Name, t.Results) + "\n")
		}
		textReports = append(textReports, report.String())
	}
	return textReports, nil
}

func renderBenchmarkTable(title string, values []benchrunner.BenchmarkValue) string {
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
		t.AppendRow(table.Row{r.Name, r.String()})
	}
	return t.Render()
}
