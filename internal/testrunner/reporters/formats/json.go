// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package formats

import (
	"encoding/json"
	"fmt"

	"github.com/elastic/elastic-package/internal/testrunner"
)

func init() {
	testrunner.RegisterReporterFormat(ReportFormatJSON, reportJSONFormat)
}

const (
	// ReportFormatJSON reports test results in a JSON format
	ReportFormatJSON testrunner.TestReportFormat = "json"
)

type jsonResult struct {
	Package        string `json:"package"`
	DataStream     string `json:"data_stream,omitempty"`
	TestType       string `json:"test_type"`
	Name           string `json:"name"`
	Result         string `json:"result"`
	TimeElapsed    string `json:"time_elapsed"`
	FailureDetails string `json:"failure_details,omitempty"`
}

func reportJSONFormat(results []testrunner.TestResult) (string, error) {
	if len(results) == 0 {
		return "No test results", nil
	}

	jsonReport := make([]jsonResult, 0, len(results))
	for _, r := range results {
		jsonResult := jsonResult{
			Package:     r.Package,
			DataStream:  r.DataStream,
			TestType:    string(r.TestType),
			Name:        r.Name,
			TimeElapsed: r.TimeElapsed.String(),
		}

		if r.FailureMsg != "" {
			jsonResult.FailureDetails = fmt.Sprintf("%s/%s %s:\n%s\n", r.Package, r.DataStream, r.Name, r.FailureDetails)
		}

		var result string
		if r.ErrorMsg != "" {
			result = fmt.Sprintf("ERROR: %s", r.ErrorMsg)
		} else if r.FailureMsg != "" {
			result = fmt.Sprintf("FAIL: %s", r.FailureMsg)
		} else if r.Skipped != nil {
			result = fmt.Sprintf("SKIPPED: %s", r.Skipped.String())
		} else {
			result = "PASS"
		}
		jsonResult.Result = result

		jsonReport = append(jsonReport, jsonResult)
	}

	b, err := json.Marshal(jsonReport)
	if err != nil {
		return "", fmt.Errorf("marshaling test results to JSON: %w", err)
	}
	return string(b), nil
}
