// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package testrunner

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/elastic/elastic-package/internal/elasticsearch"
	"github.com/elastic/elastic-package/internal/kibana"
	"github.com/elastic/elastic-package/internal/profile"
)

// TestType represents the various supported test types
type TestType string

// TestOptions contains test runner options.
type TestOptions struct {
	Profile            *profile.Profile
	TestFolder         TestFolder
	PackageRootPath    string
	GenerateTestResult bool
	API                *elasticsearch.API
	KibanaClient       *kibana.Client

	DeferCleanup   time.Duration
	ServiceVariant string
	WithCoverage   bool
	CoverageType   string

	ConfigFilePath string
	RunSetup       bool
	RunTearDown    bool
	RunTestsOnly   bool
}

// TestRunner is the interface all test runners must implement.
type TestRunner interface {
	// Type returns the test runner's type.
	Type() TestType

	// String returns the human-friendly name of the test runner.
	String() string

	// Run executes the test runner.
	Run(TestOptions) ([]TestResult, error)

	// TearDown cleans up any test runner resources. It must be called
	// after the test runner has finished executing.
	TearDown() error

	CanRunPerDataStream() bool

	TestFolderRequired() bool

	CanRunSetupTeardownIndependent() bool
}

var runners = map[TestType]TestRunner{}

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

	// If the test was skipped, the reason it was skipped and a link for more
	// details.
	Skipped *SkipConfig

	// Coverage details in Cobertura format (optional).
	Coverage CoverageReport
}

// ResultComposer wraps a TestResult and provides convenience methods for
// manipulating this TestResult.
type ResultComposer struct {
	TestResult
	StartTime time.Time
}

// NewResultComposer returns a new ResultComposer with the StartTime
// initialized to now.
func NewResultComposer(tr TestResult) *ResultComposer {
	return &ResultComposer{
		TestResult: tr,
		StartTime:  time.Now(),
	}
}

// WithError sets an error on the test result wrapped by ResultComposer.
func (rc *ResultComposer) WithError(err error) ([]TestResult, error) {
	rc.TimeElapsed = time.Since(rc.StartTime)
	if err == nil {
		return []TestResult{rc.TestResult}, nil
	}

	if tcf, ok := err.(ErrTestCaseFailed); ok {
		rc.FailureMsg += tcf.Reason
		rc.FailureDetails += tcf.Details
		return []TestResult{rc.TestResult}, nil
	}

	rc.ErrorMsg += err.Error()
	return []TestResult{rc.TestResult}, err
}

// WithSuccess marks the test result wrapped by ResultComposer as successful.
func (rc *ResultComposer) WithSuccess() ([]TestResult, error) {
	return rc.WithError(nil)
}

// WithSkip marks the test result wrapped by ResultComposer as skipped.
func (rc *ResultComposer) WithSkip(s *SkipConfig) ([]TestResult, error) {
	rc.TestResult.Skipped = s
	return rc.WithError(nil)
}

// TestFolder encapsulates the test folder path and names of the package + data stream
// to which the test folder belongs.
type TestFolder struct {
	Path       string
	Package    string
	DataStream string
}

// AssumeTestFolders assumes potential test folders for the given package, data streams and test types.
func AssumeTestFolders(packageRootPath string, dataStreams []string, testType TestType) ([]TestFolder, error) {
	// Expected folder structure:
	// <packageRootPath>/
	//   data_stream/
	//     <dataStream>/

	dataStreamsPath := filepath.Join(packageRootPath, "data_stream")

	if len(dataStreams) == 0 {
		fileInfos, err := os.ReadDir(dataStreamsPath)
		if errors.Is(err, os.ErrNotExist) {
			return []TestFolder{}, nil // data streams defined
		}
		if err != nil {
			return nil, fmt.Errorf("can't read directory (path: %s): %w", dataStreamsPath, err)
		}

		for _, fi := range fileInfos {
			if !fi.IsDir() {
				continue
			}
			dataStreams = append(dataStreams, fi.Name())
		}
	}

	var folders []TestFolder
	for _, dataStream := range dataStreams {
		folders = append(folders, TestFolder{
			Path:       filepath.Join(dataStreamsPath, dataStream, "_dev", "test", string(testType)),
			Package:    filepath.Base(packageRootPath),
			DataStream: dataStream,
		})
	}
	return folders, nil
}

// FindTestFolders finds test folders for the given package and, optionally, test type and data streams
func FindTestFolders(packageRootPath string, dataStreams []string, testType TestType) ([]TestFolder, error) {
	// Expected folder structure for packages with data streams (integration packages):
	// <packageRootPath>/
	//   data_stream/
	//     <dataStream>/
	//       _dev/
	//         test/
	//           <testType>/
	//
	// Expected folder structure for packages without data streams (input packages):
	// <packageRootPath>/
	//   _dev/
	//     test/
	//       <testType>/

	testTypeGlob := "*"
	if testType != "" {
		testTypeGlob = string(testType)
	}

	var paths []string
	if len(dataStreams) > 0 {
		sort.Strings(dataStreams)
		for _, dataStream := range dataStreams {
			p, err := findDataStreamTestFolderPaths(packageRootPath, dataStream, testTypeGlob)
			if err != nil {
				return nil, err
			}

			paths = append(paths, p...)
		}
	} else {
		// No datastreams specified, try to discover them.
		p, err := findDataStreamTestFolderPaths(packageRootPath, "*", testTypeGlob)
		if err != nil {
			return nil, err
		}

		// Look for tests at the package level, like for input packages.
		if len(p) == 0 {
			p, err = findPackageTestFolderPaths(packageRootPath, testTypeGlob)
			if err != nil {
				return nil, err
			}
		}

		paths = p
	}

	folders := make([]TestFolder, len(paths))
	_, pkg := filepath.Split(packageRootPath)
	for idx, p := range paths {
		dataStream := ExtractDataStreamFromPath(p, packageRootPath)

		folder := TestFolder{
			Path:       p,
			Package:    pkg,
			DataStream: dataStream,
		}

		folders[idx] = folder
	}

	return folders, nil
}

func ExtractDataStreamFromPath(fullPath, packageRootPath string) string {
	relP := strings.TrimPrefix(fullPath, packageRootPath)
	parts := strings.Split(relP, string(filepath.Separator))
	dataStream := ""
	if len(parts) >= 3 && parts[1] == "data_stream" {
		dataStream = parts[2]
	}

	return dataStream
}

// RegisterRunner method registers the test runner.
func RegisterRunner(runner TestRunner) {
	runners[runner.Type()] = runner
}

// Run method delegates execution to the registered test runner, based on the test type.
func Run(testType TestType, options TestOptions) ([]TestResult, error) {
	runner, defined := runners[testType]
	if !defined {
		return nil, fmt.Errorf("unregistered runner test: %s", testType)
	}

	results, err := runner.Run(options)
	tdErr := runner.TearDown()
	if err != nil {
		return nil, fmt.Errorf("could not complete test run: %w", err)
	}
	if tdErr != nil {
		return results, fmt.Errorf("could not teardown test runner: %w", tdErr)
	}
	return results, nil
}

// TestRunners returns registered test runners.
func TestRunners() map[TestType]TestRunner {
	return runners
}

// findDataStreamTestFoldersPaths can only be called for test runners that require tests to be defined
// at the data stream level.
func findDataStreamTestFolderPaths(packageRootPath, dataStreamGlob, testTypeGlob string) ([]string, error) {
	testFoldersGlob := filepath.Join(packageRootPath, "data_stream", dataStreamGlob, "_dev", "test", testTypeGlob)
	paths, err := filepath.Glob(testFoldersGlob)
	if err != nil {
		return nil, fmt.Errorf("error finding test folders: %w", err)
	}
	return paths, err
}

// findPackageTestFolderPaths finds tests at the package level.
func findPackageTestFolderPaths(packageRootPath, testTypeGlob string) ([]string, error) {
	testFoldersGlob := filepath.Join(packageRootPath, "_dev", "test", testTypeGlob)
	paths, err := filepath.Glob(testFoldersGlob)
	if err != nil {
		return nil, fmt.Errorf("error finding test folders: %w", err)
	}
	return paths, err
}
