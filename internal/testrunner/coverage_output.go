// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package testrunner

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/builder"
)

const coverageDtd = `<!DOCTYPE coverage SYSTEM "http://cobertura.sourceforge.net/xml/coverage-04.dtd">`

type testCoverageDetails struct {
	packageName string
	dataStreams map[string][]string // <data_stream> : <test case 1, test case 2, ...>
}

func newTestCoverageDetails(packageName string) *testCoverageDetails {
	return &testCoverageDetails{packageName: packageName, dataStreams: map[string][]string{}}
}

func (tcd *testCoverageDetails) withUncoveredDataStreams(dataStreams []string) *testCoverageDetails {
	for _, wt := range dataStreams {
		tcd.dataStreams[wt] = []string{}
	}
	return tcd
}

func (tcd *testCoverageDetails) withTestResults(results []TestResult) *testCoverageDetails {
	for _, result := range results {
		if _, ok := tcd.dataStreams[result.DataStream]; !ok {
			tcd.dataStreams[result.DataStream] = []string{}
		}
		tcd.dataStreams[result.DataStream] = append(tcd.dataStreams[result.DataStream], result.DataStream)
	}
	return tcd
}

type coberturaReport struct {
}

func (r *coberturaReport) bytes() ([]byte, error) {
	out, err := xml.MarshalIndent(&r, "", "  ")
	if err != nil {
		return nil, errors.Wrap(err, "unable to format test results as xUnit")
	}

	var buffer bytes.Buffer
	buffer.WriteString(xml.Header)
	buffer.WriteString("\n")
	buffer.WriteString(coverageDtd)
	buffer.WriteString("\n")
	buffer.Write(out)
	return buffer.Bytes(), nil
}

// WriteCoverage function calculates test coverage for the given package.
// It requires to execute tests for all data streams (same test type), so the coverage can be calculated properly.
// The function includes following test types in the coverage report - pipeline and system.
func WriteCoverage(packageRootPath, packageName string, testType TestType, results []TestResult) error {
	details, err := collectTestCoverageDetails(packageRootPath, packageName, testType, results)
	if err != nil {
		return errors.Wrap(err, "can't collect test coverage details")
	}

	report := transformToCoberturaReport(details)

	err = writeCoverageReportFile(report, packageRootPath)
	if err != nil {
		return errors.Wrap(err, "can't write test coverage report file")
	}
	return nil
}

func collectTestCoverageDetails(packageRootPath, packageName string, testType TestType, results []TestResult) (*testCoverageDetails, error) {
	withoutTests, err := findDataStreamsWithoutTests(packageRootPath, testType)
	if err != nil {
		return nil, errors.Wrap(err, "can't find data streams without tests")
	}

	details := newTestCoverageDetails(packageName).
		withUncoveredDataStreams(withoutTests).
		withTestResults(results)
	return details, nil
}

func findDataStreamsWithoutTests(packageRootPath string, testType TestType) ([]string, error) {
	dataStreamDir := filepath.Join(packageRootPath, "data_stream")
	dataStreams, err := ioutil.ReadDir(dataStreamDir)
	if err != nil {
		return nil, errors.Wrap(err, "can't list data streams directory")
	}

	var noTests []string
	for _, dataStream := range dataStreams {
		if !dataStream.IsDir() {
			continue
		}

		dataStreamTestPath := filepath.Join(packageRootPath, "data_stream", dataStream.Name(), "_dev", "test", string(testType))
		_, err := os.Stat(dataStreamTestPath)
		if errors.Is(err, os.ErrNotExist) {
			noTests = append(noTests, dataStream.Name())
		}
		if err != nil {
			return nil, errors.Wrapf(err, "can't stat path: %s", dataStreamTestPath)
		}
	}
	return noTests, nil
}

func transformToCoberturaReport(details *testCoverageDetails) *coberturaReport {
	panic("TODO")
}

func writeCoverageReportFile(report *coberturaReport, packageName string) error {
	dest, err := testCoverageReportsDir()
	if err != nil {
		return errors.Wrap(err, "could not determine test coverage reports folder")
	}

	// Create test coverage reports folder if it doesn't exist
	_, err = os.Stat(dest)
	if err != nil && os.IsNotExist(err) {
		if err := os.MkdirAll(dest, 0755); err != nil {
			return errors.Wrap(err, "could not create test coverage reports folder")
		}
	}

	fileName := fmt.Sprintf("coverage-%s-%d-report.xml", packageName, time.Now().UnixNano())
	filePath := filepath.Join(dest, fileName)

	b, err := report.bytes()
	if err != nil {
		return errors.Wrap(err, "can't marshal test coverage report")
	}

	if err := ioutil.WriteFile(filePath, b, 0644); err != nil {
		return errors.Wrap(err, "could not write test coverage report file")
	}
	return nil
}

func testCoverageReportsDir() (string, error) {
	buildDir, _, err := builder.FindBuildDirectory()
	if err != nil {
		return "", errors.Wrap(err, "locating build directory failed")
	}
	return filepath.Join(buildDir, "test-coverage"), nil
}
