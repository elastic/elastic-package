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
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/builder"
	"github.com/elastic/elastic-package/internal/elasticsearch/ingest"
	"github.com/elastic/elastic-package/internal/multierror"
	"github.com/elastic/elastic-package/internal/packages"
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
			if err := tcd.cobertura.Merge(result.Coverage); err != nil {
				tcd.errors = append(tcd.errors, errors.Wrapf(err, "can't merge Cobertura coverage for test `%s`", result.Name))
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
	Hits       int64            `xml:"hits,attr"`
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

// Merge merges two coverage reports for a given class.
func (c *CoberturaClass) Merge(b *CoberturaClass) error {
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
		return errors.Errorf("merging incompatible classes: %+v != %+v", *c, *b)
	}
	// Update methods
	for idx := range b.Methods {
		c.Methods[idx].Hits += b.Methods[idx].Hits
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

// Merge merges two coverage reports for a given package.
func (p *CoberturaPackage) Merge(b *CoberturaPackage) error {
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
			if err := target.Merge(class); err != nil {
				return err
			}
		} else {
			p.Classes = append(p.Classes, class)
		}
	}
	return nil
}

// Merge merges two coverage reports.
func (c *CoberturaCoverage) Merge(b *CoberturaCoverage) error {
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
			if err := target.Merge(pkg); err != nil {
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
		return errors.Wrap(err, "can't collect test coverage details")
	}

	// Use provided cobertura report, or generate a custom report if not available.
	report := details.cobertura
	if report == nil {
		report = transformToCoberturaReport(details)
	}

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

func transformToCoberturaReport(details *testCoverageDetails) *CoberturaCoverage {
	var classes []*CoberturaClass
	for dataStream, testCases := range details.dataStreams {
		if dataStream == "" {
			continue // ignore tests running in the package context (not data stream), mostly referring to installed assets
		}

		var methods []*CoberturaMethod

		if len(testCases) == 0 {
			methods = append(methods, &CoberturaMethod{
				Name:  "Missing",
				Lines: []*CoberturaLine{{Number: 1, Hits: 0}},
			})
		} else {
			methods = append(methods, &CoberturaMethod{
				Name:  "OK",
				Lines: []*CoberturaLine{{Number: 1, Hits: 1}},
			})
		}

		aClass := &CoberturaClass{
			Name:     string(details.testType),
			Filename: details.packageName + "/" + dataStream,
			Methods:  methods,
		}
		classes = append(classes, aClass)
	}

	return &CoberturaCoverage{
		Timestamp: time.Now().UnixNano(),
		Packages: []*CoberturaPackage{
			{
				Name:    details.packageName,
				Classes: classes,
			},
		},
	}
}

func writeCoverageReportFile(report *CoberturaCoverage, packageName string) error {
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

// GetPipelineCoverage returns a coverage report for the provided set of ingest pipelines.
func GetPipelineCoverage(options TestOptions, pipelines []ingest.Pipeline) (*CoberturaCoverage, error) {
	packagePath, err := packages.MustFindPackageRoot()
	if err != nil {
		return nil, errors.Wrap(err, "error finding package root")
	}

	dataStreamPath, found, err := packages.FindDataStreamRootForPath(options.TestFolder.Path)
	if err != nil {
		return nil, errors.Wrap(err, "locating data_stream root failed")
	}
	if !found {
		return nil, errors.New("data stream root not found")
	}

	// Use the Node Stats API to get stats for all installed pipelines.
	// These stats contain hit counts for all main processors in a pipeline.
	stats, err := ingest.GetPipelineStats(options.API, pipelines)
	if err != nil {
		return nil, errors.Wrap(err, "error fetching pipeline stats for code coverage calculations")
	}

	// Construct the Cobertura report.
	pkg := &CoberturaPackage{
		Name: options.TestFolder.Package + "." + options.TestFolder.DataStream,
	}

	coverage := &CoberturaCoverage{
		Sources: []*CoberturaSource{
			{
				Path: packagePath,
			},
		},
		Packages:  []*CoberturaPackage{pkg},
		Timestamp: time.Now().UnixNano(),
	}

	// Calculate coverage for each pipeline
	for _, pipeline := range pipelines {
		covered, class, err := coverageForSinglePipeline(pipeline, stats, packagePath, dataStreamPath)
		if err != nil {
			return nil, errors.Wrapf(err, "error calculating coverage for pipeline '%s'", pipeline.Filename())
		}
		pkg.Classes = append(pkg.Classes, class)
		coverage.LinesValid += int64(len(class.Methods))
		coverage.LinesCovered += covered
	}
	return coverage, nil
}

func coverageForSinglePipeline(pipeline ingest.Pipeline, stats ingest.PipelineStatsMap, packagePath, dataStreamPath string) (linesCovered int64, class *CoberturaClass, err error) {
	// Load the list of main processors from the pipeline source code, annotated with line numbers.
	src, err := pipeline.Processors()
	if err != nil {
		return 0, nil, err
	}

	pstats, found := stats[pipeline.Name]
	if !found {
		return 0, nil, errors.Errorf("pipeline '%s' not installed in Elasticsearch", pipeline.Name)
	}

	// Ensure there is no inconsistency in the list of processors in stats vs obtained from source.
	if len(src) != len(pstats.Processors) {
		return 0, nil, errors.Errorf("processor count mismatch for %s (src:%d stats:%d)", pipeline.Filename(), len(src), len(pstats.Processors))
	}
	for idx, st := range pstats.Processors {
		// Check that we have the expected type of processor, except for `compound` processors.
		// Elasticsearch will return a `compound` processor in the case of `foreach` and
		// any processor that defines `on_failure` processors.
		if st.Type != "compound" && st.Type != src[idx].Type {
			return 0, nil, errors.Errorf("processor type mismatch for %s processor %d (src:%s stats:%s)", pipeline.Filename(), idx, src[idx].Type, st.Type)
		}
	}

	// Tests install pipelines as `filename-<nonce>` (without original extension).
	// Use the filename part for the report.
	pipelineName := pipeline.Name
	if nameEnd := strings.LastIndexByte(pipelineName, '-'); nameEnd != -1 {
		pipelineName = pipelineName[:nameEnd]
	}

	// File path has to be relative to the packagePath added to the cobertura Sources list
	// so that the source is reachable by the report tool.
	pipelinePath := filepath.Join(dataStreamPath, "elasticsearch", "ingest_pipeline", pipeline.Filename())
	pipelineRelPath, err := filepath.Rel(packagePath, pipelinePath)
	if err != nil {
		return 0, nil, errors.Wrapf(err, "cannot create relative path to pipeline file. Package root: '%s', pipeline path: '%s'", packagePath, pipelinePath)
	}

	// Report every pipeline as a "class".
	class = &CoberturaClass{
		Name:     pipelineName,
		Filename: pipelineRelPath,
	}

	// Calculate covered and total processors (reported as both lines and methods).
	for idx, srcProc := range src {
		if pstats.Processors[idx].Stats.Count > 0 {
			linesCovered++
		}
		method := CoberturaMethod{
			Name: srcProc.Type,
			Hits: pstats.Processors[idx].Stats.Count,
		}
		for num := srcProc.FirstLine; num <= srcProc.LastLine; num++ {
			line := &CoberturaLine{
				Number: num,
				Hits:   pstats.Processors[idx].Stats.Count,
			}
			class.Lines = append(class.Lines, line)
			method.Lines = append(method.Lines, line)
		}
		class.Methods = append(class.Methods, &method)
	}
	return linesCovered, class, nil
}
