// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package testrunner

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/builder"
)

const coverageDtd = `<!DOCTYPE coverage SYSTEM "http://cobertura.sourceforge.net/xml/coverage-04.dtd">`

type testCoverageDetails struct {
	packageName string
	testType    TestType
	dataStreams map[string][]string // <data_stream> : <test case 1, test case 2, ...>
}

func newTestCoverageDetails(packageName string, testType TestType) *testCoverageDetails {
	return &testCoverageDetails{packageName: packageName, testType: testType, dataStreams: map[string][]string{}}
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
		tcd.dataStreams[result.DataStream] = append(tcd.dataStreams[result.DataStream], result.Name)
	}
	return tcd
}

type coberturaCoverage struct {
	XMLName         xml.Name            `xml:"coverage"`
	LineRate        float32             `xml:"line-rate,attr"`
	BranchRate      float32             `xml:"branch-rate,attr"`
	Version         string              `xml:"version,attr"`
	Timestamp       int64               `xml:"timestamp,attr"`
	LinesCovered    int64               `xml:"lines-covered,attr"`
	LinesValid      int64               `xml:"lines-valid,attr"`
	BranchesCovered int64               `xml:"branches-covered,attr"`
	BranchesValid   int64               `xml:"branches-valid,attr"`
	Complexity      float32             `xml:"complexity,attr"`
	Sources         []*coberturaSource  `xml:"sources>source"`
	Packages        []*coberturaPackage `xml:"packages>package"`
}

type coberturaSource struct {
	Path string `xml:",chardata"`
}

type coberturaPackage struct {
	Name       string            `xml:"name,attr"`
	LineRate   float32           `xml:"line-rate,attr"`
	BranchRate float32           `xml:"branch-rate,attr"`
	Complexity float32           `xml:"complexity,attr"`
	Classes    []*coberturaClass `xml:"classes>class"`
}

type coberturaClass struct {
	Name       string             `xml:"name,attr"`
	Filename   string             `xml:"filename,attr"`
	LineRate   float32            `xml:"line-rate,attr"`
	BranchRate float32            `xml:"branch-rate,attr"`
	Complexity float32            `xml:"complexity,attr"`
	Methods    []*coberturaMethod `xml:"methods>method"`
}

type coberturaMethod struct {
	Name       string         `xml:"name,attr"`
	Signature  string         `xml:"signature,attr"`
	LineRate   float32        `xml:"line-rate,attr"`
	BranchRate float32        `xml:"branch-rate,attr"`
	Complexity float32        `xml:"complexity,attr"`
	Lines      coberturaLines `xml:"lines>line"`
}

type coberturaLine struct {
	Number int   `xml:"number,attr"`
	Hits   int64 `xml:"hits,attr"`
}

type coberturaLines []*coberturaLine

func (c *coberturaCoverage) bytes() ([]byte, error) {
	out, err := xml.MarshalIndent(&c, "", "  ")
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
func WriteCoverage(packageRootPath, packageName string, testType TestType, results []TestResult) error {
	details, err := collectTestCoverageDetails(packageRootPath, packageName, testType, results)
	if err != nil {
		return errors.Wrap(err, "can't collect test coverage details")
	}

	report := transformToCoberturaReport(details)

	err = writeCoverageReportFile(report, packageName)
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

	details := newTestCoverageDetails(packageName, testType).
		withUncoveredDataStreams(withoutTests).
		withTestResults(results)
	return details, nil
}

func findDataStreamsWithoutTests(packageRootPath string, testType TestType) ([]string, error) {
	var noTests []string

	dataStreamDir := filepath.Join(packageRootPath, "data_stream")
	dataStreams, err := os.ReadDir(dataStreamDir)
	if errors.Is(err, os.ErrNotExist) {
		return noTests, nil // there are packages that don't have any data streams (fleet_server, security_detection_engine)
	} else if err != nil {
		return nil, errors.Wrap(err, "can't list data streams directory")
	}

	for _, dataStream := range dataStreams {
		if !dataStream.IsDir() {
			continue
		}

		expected, err := verifyTestExpected(packageRootPath, dataStream.Name(), testType)
		if err != nil {
			return nil, errors.Wrap(err, "can't verify if test is expected")
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
			return nil, errors.Wrapf(err, "can't stat path: %s", dataStreamTestPath)
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
		return false, errors.Wrapf(err, "can't stat path: %s", ingestPipelinePath)
	}
	return true, nil
}

func transformToCoberturaReport(details *testCoverageDetails) *coberturaCoverage {
	var classes []*coberturaClass
	for dataStream, testCases := range details.dataStreams {
		if dataStream == "" {
			continue // ignore tests running in the package context (not data stream), mostly referring to installed assets
		}

		var methods []*coberturaMethod

		if len(testCases) == 0 {
			methods = append(methods, &coberturaMethod{
				Name:  "Missing",
				Lines: []*coberturaLine{{Number: 1, Hits: 0}},
			})
		} else {
			methods = append(methods, &coberturaMethod{
				Name:  "OK",
				Lines: []*coberturaLine{{Number: 1, Hits: 1}},
			})
		}

		aClass := &coberturaClass{
			Name:     string(details.testType),
			Filename: details.packageName + "/" + dataStream,
			Methods:  methods,
		}
		classes = append(classes, aClass)
	}

	return &coberturaCoverage{
		Timestamp: time.Now().UnixNano(),
		Packages: []*coberturaPackage{
			{
				Name:    details.packageName,
				Classes: classes,
			},
		},
	}
}

func writeCoverageReportFile(report *coberturaCoverage, packageName string) error {
	dest, err := testCoverageReportsDir()
	if err != nil {
		return errors.Wrap(err, "could not determine test coverage reports folder")
	}

	// Create test coverage reports folder if it doesn't exist
	_, err = os.Stat(dest)
	if err != nil && errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(dest, 0755); err != nil {
			return errors.Wrap(err, "could not create test coverage reports folder")
		}
	}

	fileName := fmt.Sprintf("coverage-%s-%d-report.xml", packageName, report.Timestamp)
	filePath := filepath.Join(dest, fileName)

	b, err := report.bytes()
	if err != nil {
		return errors.Wrap(err, "can't marshal test coverage report")
	}

	if err := os.WriteFile(filePath, b, 0644); err != nil {
		return errors.Wrap(err, "could not write test coverage report file")
	}
	return nil
}

func testCoverageReportsDir() (string, error) {
	buildDir, err := builder.BuildDirectory()
	if err != nil {
		return "", errors.Wrap(err, "locating build directory failed")
	}
	return filepath.Join(buildDir, "test-coverage"), nil
}
