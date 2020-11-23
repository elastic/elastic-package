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

// TestOptions contains test runner options.
type TestOptions struct {
	TestFolder         TestFolder
	PackageRootPath    string
	GenerateTestResult bool
	ESClient           *elasticsearch.Client

	DeferCleanup time.Duration
}

// RunFunc method defines main run function of a test runner.
type RunFunc func(options TestOptions) ([]TestResult, error)

var runners = map[TestType]RunFunc{}

// TestResult contains a single test's results
type TestResult struct {
	// Name of test result. Optional.
	Name string

	// Package to which this test result belongs.
	Package string

	// TestType indicates the type of test.
	TestType TestType

	// Data stream to which this test result belongs.
	DataStream string

	// Time elapsed from running a test case to arriving at its result.
	TimeElapsed time.Duration

	// If test case failed, short description of the failure. A failure is
	// when the test completes execution but the actual results of the test
	// don't match the expected results.
	FailureMsg string

	// If test case failed, longer description of the failure.
	FailureDetails string

	// If there was an error while running the test case, description
	// of the error. An error is when the test cannot complete execution due
	// to an unexpected runtime error in the test execution.
	ErrorMsg string
}

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

func findTestFolderPaths(packageRootPath, dataStreamGlob, testTypeGlob string) ([]string, error) {
	testFoldersGlob := filepath.Join(packageRootPath, "data_stream", dataStreamGlob, "_dev", "test", testTypeGlob)
	paths, err := filepath.Glob(testFoldersGlob)
	if err != nil {
		return nil, errors.Wrap(err, "error finding test folders")
	}
	return paths, err
}
