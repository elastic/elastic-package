// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.
package formats

import (
	"encoding/xml"
	"fmt"

	"github.com/elastic/elastic-package/internal/benchrunner"
)

func init() {
	benchrunner.RegisterReporterFormat(ReportFormatXML, reportXMLFormat)
}

const (
	// ReportFormatXML reports benchmark results in the xml format
	ReportFormatXML benchrunner.BenchReportFormat = "xml"
)

func reportXMLFormat(results []*benchrunner.Result) ([]string, error) {
	var benchmarks []*benchrunner.BenchmarkResult
	for _, r := range results {
		if r.Benchmark != nil {
			benchmarks = append(benchmarks, r.Benchmark)
		}
	}

	benchFormatted, err := reportXMLFormatBenchmark(benchmarks)
	if err != nil {
		return nil, err
	}
	return benchFormatted, nil
}

func reportXMLFormatBenchmark(benchmarks []*benchrunner.BenchmarkResult) ([]string, error) {
	var reports []string
	for _, b := range benchmarks {
		// Filter out detailed benchmarks. These add too much information for the
		// aggregated nature of XML reports, creating a lot of noise in Jenkins.
		var benchmarks []benchrunner.BenchmarkTest
		for _, t := range b.Tests {
			if !t.Detailed {
				benchmarks = append(benchmarks, t)
			}
		}
		b.Tests = benchmarks
		out, err := xml.MarshalIndent(b, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("unable to format benchmark results as XML: %w", err)
		}
		reports = append(reports, xml.Header+string(out))
	}
	return reports, nil
}
