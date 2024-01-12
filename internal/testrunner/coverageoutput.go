// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package testrunner

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/elastic/elastic-package/internal/builder"
	"github.com/elastic/elastic-package/internal/files"
	"github.com/elastic/elastic-package/internal/multierror"
)

const coverageDtd = `<!DOCTYPE coverage SYSTEM "http://cobertura.sourceforge.net/xml/coverage-04.dtd">`

type testCoverageDetails struct {
	packageName string
	testType    TestType
	dataStreams map[string][]string // <data_stream> : <test case 1, test case 2, ...>
	cobertura   *CoberturaCoverage  // For tests to provide custom Cobertura results.
	errors      multierror.Error
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
		if tcd.cobertura != nil && result.Coverage != nil {
			if err := tcd.cobertura.merge(result.Coverage); err != nil {
				tcd.errors = append(tcd.errors, fmt.Errorf("can't merge Cobertura coverage for test `%s`: %w", result.Name, err))
			}
		} else if tcd.cobertura == nil {
			tcd.cobertura = result.Coverage
		}
	}
	return tcd
}

// CoberturaCoverage is the root element for a Cobertura XML report.
type CoberturaCoverage struct {
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
	Sources         []*CoberturaSource  `xml:"sources>source"`
	Packages        []*CoberturaPackage `xml:"packages>package"`
}

// CoberturaSource represents a base path to the covered source code.
type CoberturaSource struct {
	Path string `xml:",chardata"`
}

// CoberturaPackage represents a package in a Cobertura XML report.
type CoberturaPackage struct {
	Name       string            `xml:"name,attr"`
	LineRate   float32           `xml:"line-rate,attr"`
	BranchRate float32           `xml:"branch-rate,attr"`
	Complexity float32           `xml:"complexity,attr"`
	Classes    []*CoberturaClass `xml:"classes>class"`
}

// CoberturaClass represents a class in a Cobertura XML report.
type CoberturaClass struct {
	Name       string             `xml:"name,attr"`
	Filename   string             `xml:"filename,attr"`
	LineRate   float32            `xml:"line-rate,attr"`
	BranchRate float32            `xml:"branch-rate,attr"`
	Complexity float32            `xml:"complexity,attr"`
	Methods    []*CoberturaMethod `xml:"methods>method"`
	Lines      []*CoberturaLine   `xml:"lines>line"`
}

// CoberturaMethod represents a method in a Cobertura XML report.
type CoberturaMethod struct {
	Name       string           `xml:"name,attr"`
	Signature  string           `xml:"signature,attr"`
	LineRate   float32          `xml:"line-rate,attr"`
	BranchRate float32          `xml:"branch-rate,attr"`
	Complexity float32          `xml:"complexity,attr"`
	Lines      []*CoberturaLine `xml:"lines>line"`
}

// CoberturaLine represents a source line in a Cobertura XML report.
type CoberturaLine struct {
	Number int   `xml:"number,attr"`
	Hits   int64 `xml:"hits,attr"`
}

func (c *CoberturaCoverage) bytes() ([]byte, error) {
	out, err := xml.MarshalIndent(&c, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("unable to format test results as xUnit: %w", err)
	}

	var buffer bytes.Buffer
	buffer.WriteString(xml.Header)
	buffer.WriteString("\n")
	buffer.WriteString(coverageDtd)
	buffer.WriteString("\n")
	buffer.Write(out)
	return buffer.Bytes(), nil
}

// merge merges two coverage reports for a given class.
func (c *CoberturaClass) merge(b *CoberturaClass) error {
	// Check preconditions: classes should be the same.
	equal := c.Name == b.Name &&
		c.Filename == b.Filename &&
		len(c.Lines) == len(b.Lines) &&
		len(c.Methods) == len(b.Methods)
	for idx := range c.Lines {
		equal = equal && c.Lines[idx].Number == b.Lines[idx].Number
	}
	for idx := range c.Methods {
		equal = equal && c.Methods[idx].Name == b.Methods[idx].Name &&
			len(c.Methods[idx].Lines) == len(b.Methods[idx].Lines)
	}
	if !equal {
		return fmt.Errorf("merging incompatible classes: %+v != %+v", *c, *b)
	}
	// Update methods
	for idx := range b.Methods {
		for l := range b.Methods[idx].Lines {
			c.Methods[idx].Lines[l].Hits += b.Methods[idx].Lines[l].Hits
		}
	}
	// Rebuild lines
	c.Lines = nil
	for _, m := range c.Methods {
		c.Lines = append(c.Lines, m.Lines...)
	}
	return nil
}

// merge merges two coverage reports for a given package.
func (p *CoberturaPackage) merge(b *CoberturaPackage) error {
	// Merge classes
	for _, class := range b.Classes {
		var target *CoberturaClass
		for _, existing := range p.Classes {
			if existing.Name == class.Name {
				target = existing
				break
			}
		}
		if target != nil {
			if err := target.merge(class); err != nil {
				return err
			}
		} else {
			p.Classes = append(p.Classes, class)
		}
	}
	return nil
}

// merge merges two coverage reports.
func (c *CoberturaCoverage) merge(b *CoberturaCoverage) error {
	// Merge source paths
	for _, path := range b.Sources {
		found := false
		for _, existing := range c.Sources {
			if found = existing.Path == path.Path; found {
				break
			}
		}
		if !found {
			c.Sources = append(c.Sources, path)
		}
	}

	// Merge packages
	for _, pkg := range b.Packages {
		var target *CoberturaPackage
		for _, existing := range c.Packages {
			if existing.Name == pkg.Name {
				target = existing
				break
			}
		}
		if target != nil {
			if err := target.merge(pkg); err != nil {
				return err
			}
		} else {
			c.Packages = append(c.Packages, pkg)
		}
	}

	// Recalculate global line coverage count
	c.LinesValid = 0
	c.LinesCovered = 0
	for _, pkg := range c.Packages {
		for _, cls := range pkg.Classes {
			for _, line := range cls.Lines {
				c.LinesValid++
				if line.Hits > 0 {
					c.LinesCovered++
				}
			}
		}
	}
	return nil
}

// WriteCoverage function calculates test coverage for the given package.
// It requires to execute tests for all data streams (same test type), so the coverage can be calculated properly.
func WriteCoverage(packageRootPath, packageName string, testType TestType, results []TestResult) error {
	details, err := collectTestCoverageDetails(packageRootPath, packageName, testType, results)
	if err != nil {
		return fmt.Errorf("can't collect test coverage details: %w", err)
	}

	dir, err := files.FindRepositoryRootDirectory()
	if err != nil {
		return err
	}

	relativePath := strings.TrimPrefix(packageRootPath, dir)
	relativePath = strings.TrimPrefix(relativePath, "/")
	baseFolder := filepath.Dir(relativePath)

	// Use provided cobertura report, or generate a custom report if not available.
	report := details.cobertura
	if report == nil {
		report = transformToCoberturaReport(details, baseFolder)
	}

	err = writeCoverageReportFile(report, packageName)
	if err != nil {
		return fmt.Errorf("can't write test coverage report file: %w", err)
	}
	return nil
}

func collectTestCoverageDetails(packageRootPath, packageName string, testType TestType, results []TestResult) (*testCoverageDetails, error) {
	withoutTests, err := findDataStreamsWithoutTests(packageRootPath, testType)
	if err != nil {
		return nil, fmt.Errorf("can't find data streams without tests: %w", err)
	}

	details := newTestCoverageDetails(packageName, testType).
		withUncoveredDataStreams(withoutTests).
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

func transformToCoberturaReport(details *testCoverageDetails, baseFolder string) *CoberturaCoverage {
	var classes []*CoberturaClass
	var lineNumberPerTestType map[string]int = map[string]int{
		"asset":    1,
		"pipeline": 2,
		"system":   3,
		"static":   4,
	}
	lineNumber, ok := lineNumberPerTestType[string(details.testType)]
	if !ok {
		lineNumber = 5
	}
	for dataStream, testCases := range details.dataStreams {
		if dataStream == "" {
			continue // ignore tests running in the package context (not data stream), mostly referring to installed assets
		}

		var methods []*CoberturaMethod
		var lines []*CoberturaLine

		if len(testCases) == 0 {
			methods = append(methods, &CoberturaMethod{
				Name:  "Missing",
				Lines: []*CoberturaLine{{Number: lineNumber, Hits: 0}},
			})
			lines = append(lines, []*CoberturaLine{{Number: lineNumber, Hits: 0}}...)
		} else {
			methods = append(methods, &CoberturaMethod{
				Name:  "OK",
				Lines: []*CoberturaLine{{Number: lineNumber, Hits: 1}},
			})
			lines = append(lines, []*CoberturaLine{{Number: lineNumber, Hits: 1}}...)
		}

		aClass := &CoberturaClass{
			Name:     string(details.testType),
			Filename: path.Join(baseFolder, details.packageName, "data_stream", dataStream, "manifest.yml"),
			Methods:  methods,
			Lines:    lines,
		}
		classes = append(classes, aClass)
	}

	return &CoberturaCoverage{
		Timestamp: time.Now().UnixNano(),
		Packages: []*CoberturaPackage{
			{
				Name:    strings.Replace(strings.TrimSuffix(baseFolder, "/"), "/", ".", -1) + "." + details.packageName,
				Classes: classes,
			},
		},
	}
}

func writeCoverageReportFile(report *CoberturaCoverage, packageName string) error {
	dest, err := testCoverageReportsDir()
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

	fileName := fmt.Sprintf("coverage-%s-%d-report.xml", packageName, report.Timestamp)
	filePath := filepath.Join(dest, fileName)

	b, err := report.bytes()
	if err != nil {
		return fmt.Errorf("can't marshal test coverage report: %w", err)
	}

	if err := os.WriteFile(filePath, b, 0644); err != nil {
		return fmt.Errorf("could not write test coverage report file: %w", err)
	}
	return nil
}

func testCoverageReportsDir() (string, error) {
	buildDir, err := builder.BuildDirectory()
	if err != nil {
		return "", fmt.Errorf("locating build directory failed: %w", err)
	}
	return filepath.Join(buildDir, "test-coverage"), nil
}
