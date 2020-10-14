// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package testrunner

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/elastic/go-elasticsearch/v7"
	"github.com/pkg/errors"
)

// TestType represents the various supported test types
type TestType string

// TestReporter represents the various supported test results reporters
type TestReporter string

// TestOptions contains test runner options.
type TestOptions struct {
	TestFolder         TestFolder
	PackageRootPath    string
	GenerateTestResult bool
	ESClient           *elasticsearch.Client
}

// RunFunc method defines main run function of a test runner.
type RunFunc func(options TestOptions) ([]TestResult, error)

var runners = map[TestType]RunFunc{}

// TestResult contains a single test's results
type TestResult struct {
	Name string

	Package    string
	DataStream string

	TimeElapsed time.Duration

	FailureMsg string
	ErrorMsg   string
}

// ReportFunc defines the reporter function.
type ReportFunc func(results []TestResult) (string, error)

var reporters = map[TestReporter]ReportFunc{}

// TestFolder encapsulates the test folder path and names of the package + data stream
// to which the test folder belongs.
type TestFolder struct {
	Path       string
	Package    string
	DataStream string
}

// FindTestFolders finds test folders for the given package and, optionally, test type and data streams
func FindTestFolders(packageRootPath string, testType TestType, dataStreams []string) ([]TestFolder, error) {
	// Expected folder structure:
	// <packageRootPath>/
	//   data_stream/
	//     <dataStream>/
	//       _dev/
	//         test/
	//           <testType>/

	testTypeGlob := "*"
	if testType != "" {
		testTypeGlob = string(testType)
	}

	var paths []string
	if dataStreams != nil && len(dataStreams) > 0 {
		sort.Strings(dataStreams)
		for _, dataStream := range dataStreams {
			p, err := findTestFolderPaths(packageRootPath, dataStream, testTypeGlob)
			if err != nil {
				return nil, err
			}

			paths = append(paths, p...)
		}
	} else {
		p, err := findTestFolderPaths(packageRootPath, "*", testTypeGlob)
		if err != nil {
			return nil, err
		}

		paths = p
	}

	folders := make([]TestFolder, len(paths))
	_, pkg := filepath.Split(packageRootPath)
	for idx, p := range paths {
		relP := strings.TrimPrefix(p, packageRootPath)
		parts := strings.Split(relP, string(filepath.Separator))
		dataStream := parts[2]

		folder := TestFolder{
			p,
			pkg,
			dataStream,
		}

		folders[idx] = folder
	}

	return folders, nil
}

// RegisterRunner method registers the test runner.
func RegisterRunner(testType TestType, runFunc RunFunc) {
	runners[testType] = runFunc
}

// Run method delegates execution to the registered test runner, based on the test type.
func Run(testType TestType, options TestOptions) ([]TestResult, error) {
	runFunc, defined := runners[testType]
	if !defined {
		return nil, fmt.Errorf("unregistered runner test: %s", testType)
	}
	return runFunc(options)
}

// TestTypes method returns registered test types.
func TestTypes() []TestType {
	var testTypes []TestType
	for t := range runners {
		testTypes = append(testTypes, t)
	}
	return testTypes
}

// RegisterReporter registers a test reporter.
func RegisterReporter(name TestReporter, reportFunc ReportFunc) {
	reporters[name] = reportFunc
}

// Report delegates reporting of test results to the registered test reporter, based on reporter name.
func Report(name TestReporter, results []TestResult) (string, error) {
	reportFunc, defined := reporters[name]
	if !defined {
		return "", fmt.Errorf("unregistered test reporter: %s", name)
	}

	return reportFunc(results)
}

func findTestFolderPaths(packageRootPath, dataStreamGlob, testTypeGlob string) ([]string, error) {
	testFoldersGlob := filepath.Join(packageRootPath, "data_stream", dataStreamGlob, "_dev", "test", testTypeGlob)
	paths, err := filepath.Glob(testFoldersGlob)
	if err != nil {
		return nil, errors.Wrap(err, "error finding test folders")
	}
	return paths, err
}
