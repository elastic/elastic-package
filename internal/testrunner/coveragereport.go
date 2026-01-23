// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package testrunner

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/elastic/elastic-package/internal/builder"
	"github.com/elastic/elastic-package/internal/multierror"
)

type CoverageReport interface {
	TimeStamp() int64
	Merge(CoverageReport) error
	Bytes() ([]byte, error)
}

var coverageReportFormatters = []string{}

// registerCoverageReporterFormat registers a test coverage report formatter.
func registerCoverageReporterFormat(name string) {
	coverageReportFormatters = append(coverageReportFormatters, name)
}

func CoverageFormatsList() []string {
	return coverageReportFormatters
}

type testCoverageDetails struct {
	packageName string
	packageType string
	testType    TestType
	dataStreams map[string][]string // <data_stream> : <test case 1, test case 2, ...>
	coverage    CoverageReport      // For tests to provide custom Coverage results.
	errors      multierror.Error
}

func newTestCoverageDetails(packageName, packageType string, testType TestType) *testCoverageDetails {
	return &testCoverageDetails{packageName: packageName, packageType: packageType, testType: testType, dataStreams: map[string][]string{}}
}

func (tcd *testCoverageDetails) withUncoveredDataStreams(dataStreams []string) *testCoverageDetails {
	for _, wt := range dataStreams {
		tcd.dataStreams[wt] = []string{}
	}
	return tcd
}

func (tcd *testCoverageDetails) withCoverage(coverage CoverageReport) *testCoverageDetails {
	tcd.coverage = coverage
	return tcd
}

func (tcd *testCoverageDetails) withTestResults(results []TestResult) *testCoverageDetails {
	for _, result := range results {
		if _, ok := tcd.dataStreams[result.DataStream]; !ok {
			tcd.dataStreams[result.DataStream] = []string{}
		}
		tcd.dataStreams[result.DataStream] = append(tcd.dataStreams[result.DataStream], result.Name)
		if tcd.coverage != nil && result.Coverage != nil {
			if err := tcd.coverage.Merge(result.Coverage); err != nil {
				tcd.errors = append(tcd.errors, fmt.Errorf("can't merge coverage for test `%s`: %w", result.Name, err))
			}
		} else if tcd.coverage == nil {
			tcd.coverage = result.Coverage
		}
	}
	return tcd
}

// WriteCoverage function calculates test coverage for the given package.
// It requires to execute tests for all data streams (same test type), so the coverage can be calculated properly.
func WriteCoverage(workDir, packageRootPath, packageName, packageType string, testType TestType, results []TestResult, format string) error {
	report, err := createCoverageReport(packageRootPath, workDir, packageName, packageType, testType, results, format)
	if err != nil {
		return fmt.Errorf("can't create coverage report: %w", err)
	}
	if report == nil {
		return fmt.Errorf("coverage not found for test type %s", testType)
	}

	err = writeCoverageReportFile(workDir, report, packageName, string(testType))
	if err != nil {
		return fmt.Errorf("can't write test coverage report file: %w", err)
	}
	return nil
}

func createCoverageReport(packageRootPath, workDir, packageName, packageType string, testType TestType, results []TestResult, format string) (CoverageReport, error) {
	details, err := collectTestCoverageDetails(packageRootPath, workDir, packageName, packageType, testType, results, format)
	if err != nil {
		return nil, fmt.Errorf("can't collect test coverage details: %w", err)
	}

	// Use provided coverage report
	return details.coverage, nil
}

func collectTestCoverageDetails(packageRootPath, workDir, packageName, packageType string, testType TestType, results []TestResult, format string) (*testCoverageDetails, error) {
	withoutTests, err := findDataStreamsWithoutTests(packageRootPath, testType)
	if err != nil {
		return nil, fmt.Errorf("can't find data streams without tests: %w", err)
	}

	emptyCoverage, err := GenerateBasePackageCoverageReport(packageName, workDir, packageRootPath, format)
	if err != nil {
		return nil, fmt.Errorf("can't generate initial base coverage report: %w", err)
	}

	details := newTestCoverageDetails(packageName, packageType, testType).
		withUncoveredDataStreams(withoutTests).
		withCoverage(emptyCoverage).
		withTestResults(results)
	if len(details.errors) > 0 {
		return nil, details.errors
	}
	return details, nil
}

func findDataStreamsWithoutTests(packageRootPath string, testType TestType) ([]string, error) {
	var noTests []string

	dataStreamDir := filepath.Join(packageRootPath, "data_stream")
	dataStreams, err := os.ReadDir(dataStreamDir)
	if errors.Is(err, os.ErrNotExist) {
		return noTests, nil // there are packages that don't have any data streams (fleet_server, security_detection_engine)
	} else if err != nil {
		return nil, fmt.Errorf("can't list data streams directory: %w", err)
	}

	for _, dataStream := range dataStreams {
		if !dataStream.IsDir() {
			continue
		}

		expected, err := verifyTestExpected(packageRootPath, dataStream.Name(), testType)
		if err != nil {
			return nil, fmt.Errorf("can't verify if test is expected: %w", err)
		}
		if !expected {
			continue
		}

		dataStreamTestPath := filepath.Join(packageRootPath, "data_stream", dataStream.Name(), "_dev", "test", string(testType))
		_, err = os.Stat(dataStreamTestPath)
		if errors.Is(err, os.ErrNotExist) {
			noTests = append(noTests, dataStream.Name())
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("can't stat path: %s: %w", dataStreamTestPath, err)
		}
	}
	return noTests, nil
}

// verifyTestExpected function checks if tests are actually expected.
// Pipeline tests require an ingest pipeline to be defined in the data stream.
func verifyTestExpected(packageRootPath string, dataStreamName string, testType TestType) (bool, error) {
	if testType != "pipeline" {
		return true, nil
	}

	ingestPipelinePath := filepath.Join(packageRootPath, "data_stream", dataStreamName, "elasticsearch", "ingest_pipeline")
	_, err := os.Stat(ingestPipelinePath)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("can't stat path: %s: %w", ingestPipelinePath, err)
	}
	return true, nil
}

func writeCoverageReportFile(workDir string, report CoverageReport, packageName, testType string) error {
	dest, err := testCoverageReportsDir(workDir)
	if err != nil {
		return fmt.Errorf("could not determine test coverage reports folder: %w", err)
	}

	// Create test coverage reports folder if it doesn't exist
	_, err = os.Stat(dest)
	if err != nil && errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(dest, 0755); err != nil {
			return fmt.Errorf("could not create test coverage reports folder: %w", err)
		}
	}

	fileName := fmt.Sprintf("coverage-%s-%s-%d-report.xml", packageName, testType, report.TimeStamp())
	filePath := filepath.Join(dest, fileName)

	b, err := report.Bytes()
	if err != nil {
		return fmt.Errorf("can't marshal test coverage report: %w", err)
	}

	if err := os.WriteFile(filePath, b, 0644); err != nil {
		return fmt.Errorf("could not write test coverage report file: %w", err)
	}
	return nil
}

func testCoverageReportsDir(workDir string) (string, error) {
	buildDir, err := builder.BuildDirectory(workDir)
	if err != nil {
		return "", fmt.Errorf("locating build directory failed: %w", err)
	}
	return filepath.Join(buildDir, "test-coverage"), nil
}
