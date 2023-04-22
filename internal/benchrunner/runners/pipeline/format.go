// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package pipeline

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"strings"
	"time"

	"github.com/jedib0t/go-pretty/table"
	"github.com/jedib0t/go-pretty/text"
)

const (
	// ReportFormatHuman reports benchmark results in a human-readable format
	ReportFormatHuman Format = "human"
	// ReportFormatJSON reports benchmark results in the json format
	ReportFormatJSON Format = "json"
	// ReportFormatXUnit reports benchmark results in the xUnit format
	ReportFormatXUnit Format = "xUnit"
)

// Format represents a benchmark report format
type Format string

// FormatResult delegates formatting of benchmark results to the registered benchmark report formatter.
func formatResult(name Format, result *BenchmarkResult) (report []byte, err error) {
	switch name {
	case ReportFormatHuman:
		return reportHumanFormat(result), nil
	case ReportFormatJSON:
		return reportJSONFormat(result)
	case ReportFormatXUnit:
		return reportXUnitFormat(result)
	}
	return nil, fmt.Errorf("unknown format: %s", name)
}

func reportHumanFormat(b *BenchmarkResult) []byte {
	var report strings.Builder
	if len(b.Parameters) > 0 {
		report.WriteString(renderBenchmarkTable("parameters", b.Parameters) + "\n")
	}
	for _, t := range b.Tests {
		report.WriteString(renderBenchmarkTable(t.Name, t.Results) + "\n")
	}
	return []byte(report.String())
}

func renderBenchmarkTable(title string, values []BenchmarkValue) string {
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

func reportJSONFormat(b *BenchmarkResult) ([]byte, error) {
	// Filter out detailed benchmarks. These add too much information for the
	// aggregated nature of the reports, creating a lot of noise in Jenkins.
	var benchmarks []BenchmarkTest
	for _, t := range b.Tests {
		if !t.Detailed {
			benchmarks = append(benchmarks, t)
		}
	}
	b.Tests = benchmarks
	out, err := json.MarshalIndent(b, "", " ")
	if err != nil {
		return nil, fmt.Errorf("unable to format benchmark results as json: %w", err)
	}
	return out, nil
}

func reportXUnitFormat(b *BenchmarkResult) ([]byte, error) {
	// Filter out detailed benchmarks. These add too much information for the
	// aggregated nature of xUnit reports, creating a lot of noise in Jenkins.
	var benchmarks []BenchmarkTest
	for _, t := range b.Tests {
		if !t.Detailed {
			benchmarks = append(benchmarks, t)
		}
	}
	b.Tests = benchmarks
	out, err := xml.MarshalIndent(b, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("unable to format benchmark results as xUnit: %w", err)
	}
	return out, nil
}

func filenameByFormat(pkg string, format Format) string {
	var ext string
	switch format {
	default:
		fallthrough
	case ReportFormatJSON:
		ext = "json"
	case ReportFormatXUnit:
		ext = "xml"
	}
	fileName := fmt.Sprintf(
		"%s_%d.%s",
		pkg,
		time.Now().UnixNano(),
		ext,
	)
	return fileName
}
