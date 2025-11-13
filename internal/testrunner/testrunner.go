// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package testrunner

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/elastic/elastic-package/internal/elasticsearch"
	"github.com/elastic/elastic-package/internal/environment"
	"github.com/elastic/elastic-package/internal/kibana"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/profile"
)

var (
	defaultMaximumRoutines    = runtime.GOMAXPROCS(0) / 2
	maximumNumberParallelTest = environment.WithElasticPackagePrefix("MAXIMUM_NUMBER_PARALLEL_TESTS")
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

// Tester is the interface all test runners must implement.
type Tester interface {
	// Type returns the test runner's type.
	Type() TestType

	// String returns the human-friendly name of the test runner.
	String() string

	// Run executes the test runner.
	Run(context.Context) ([]TestResult, error)

	// TearDown cleans up any test runner resources. It must be called
	// after the test runner has finished executing.
	TearDown(context.Context) error

	// Parallel indicates if this test can be run in parallel or not
	Parallel() bool
}

// TestRunner is the interface test runners that require a global initialization must implement.
type TestRunner interface {
	// Type returns the test runner's type.
	Type() TestType

	// SetupRunner prepares global resources required by the test runner.
	SetupRunner(context.Context) error

	GetTests(context.Context) ([]Tester, error)

	// TearDownRunner cleans up any global test runner resources. It must be called
	// after the test runner has finished executing all its tests.
	TearDownRunner(context.Context) error
}

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

// WithCoverage appends the coverage report to the result composer. Results built with the composer
// will include this coverage report.
func (rc *ResultComposer) WithCoverage(coverage CoverageReport) *ResultComposer {
	rc.TestResult.Coverage = coverage
	return rc
}

// CoveragePackageName returns a package name that can be used in coverage reports, based on information
// in the composer.
func (rc *ResultComposer) CoveragePackageName() string {
	if rc.DataStream != "" {
		return rc.Package + "." + rc.DataStream
	}

	return rc.Package
}

// WithError sets an error on the test result wrapped by ResultComposer.
func (rc *ResultComposer) WithError(err error) ([]TestResult, error) {
	rc.TimeElapsed = time.Since(rc.StartTime)
	if err == nil {
		return []TestResult{rc.TestResult}, nil
	}

	var tcf ErrTestCaseFailed
	if errors.As(err, &tcf) {
		rc.FailureMsg += tcf.Error()
		rc.FailureDetails += tcf.Details
		return []TestResult{rc.TestResult}, nil
	}

	rc.ErrorMsg += err.Error()
	return []TestResult{rc.TestResult}, err
}

// WithErrorf sets an error on the test result wrapped by ResultComposer.
func (rc *ResultComposer) WithErrorf(format string, a ...any) ([]TestResult, error) {
	return rc.WithError(fmt.Errorf(format, a...))
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

func RunSuite(ctx context.Context, runner TestRunner) ([]TestResult, error) {
	testers, err := runner.GetTests(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve tests: %w", err)
	}
	if len(testers) == 0 {
		return nil, nil
	}

	err = runner.SetupRunner(ctx)
	if err != nil {
		cleanupCtx := context.WithoutCancel(ctx)
		tdErr := runner.TearDownRunner(cleanupCtx)
		if tdErr != nil {
			logger.Debugf("failed to tear down %s runner: %s", runner.Type(), tdErr)
		}
		return nil, fmt.Errorf("failed to setup %s runner: %w", runner.Type(), err)
	}

	var parallelTesters, sequentialTesters []Tester
	for _, tester := range testers {
		if tester.Parallel() {
			parallelTesters = append(parallelTesters, tester)
		} else {
			sequentialTesters = append(sequentialTesters, tester)
		}
	}

	var allResults, results []TestResult
	var parallelErr, sequentialErr error

	results, parallelErr = runSuiteParallel(ctx, parallelTesters)
	allResults = append(allResults, results...)

	results, sequentialErr = runSuite(ctx, sequentialTesters)
	allResults = append(allResults, results...)

	// Avoid cancellations during cleanup.
	cleanupCtx := context.WithoutCancel(ctx)
	tdErr := runner.TearDownRunner(cleanupCtx)
	if tdErr != nil {
		return allResults, fmt.Errorf("failed to tear down %s runner: %w", runner.Type(), tdErr)
	}

	if parallelErr != nil {
		return allResults, parallelErr
	}
	if sequentialErr != nil {
		return allResults, sequentialErr
	}

	return allResults, nil
}

func maxNumberRoutines() (int, error) {
	var err error
	maxRoutines := defaultMaximumRoutines
	v, ok := os.LookupEnv(maximumNumberParallelTest)
	if ok {
		maxRoutines, err = strconv.Atoi(v)
		if err != nil {
			return 0, fmt.Errorf("failed to read number of maximum routines from environment variable: %w", err)
		}
	}
	return maxRoutines, nil
}

func runSuite(ctx context.Context, testers []Tester) ([]TestResult, error) {
	if len(testers) == 0 {
		return nil, nil
	}
	logger.Debugf("Running tests sequentially")
	var results []TestResult
	for _, tester := range testers {
		r, err := run(ctx, tester)
		if err != nil {
			return results, fmt.Errorf("error running package %s tests: %w", tester.Type(), err)
		}
		results = append(results, r...)
	}

	return results, nil
}

// runSuiteParallel method delegates execution of tests to the runners generated through the factory function.
func runSuiteParallel(ctx context.Context, testers []Tester) ([]TestResult, error) {
	if len(testers) == 0 {
		return nil, nil
	}
	maxRoutines, err := maxNumberRoutines()
	if err != nil {
		return nil, err
	}

	var wg sync.WaitGroup
	type routineResult struct {
		results []TestResult
		err     error
	}
	chResults := make(chan routineResult, len(testers))

	logger.Debugf("Running tests in parallel. Maximum routines to run in parallel: %d", maxRoutines)
	// Use channel as a semaphore to limit the number of test executions in parallel
	sem := make(chan int, maxRoutines)

	for _, tester := range testers {
		wg.Add(1)
		tester := tester
		sem <- 1
		go func() {
			defer wg.Done()
			defer func() {
				<-sem
			}()
			if err := ctx.Err(); err != nil {
				logger.Errorf("context error: %s", context.Cause(ctx))
				chResults <- routineResult{nil, err}
				return
			}
			r, err := run(ctx, tester)
			chResults <- routineResult{r, err}
		}()
	}

	wg.Wait()
	close(chResults)
	close(sem)

	var results []TestResult
	var multiErr error
	testType := testers[0].Type()
	for testResults := range chResults {
		if testResults.err != nil {
			multiErr = errors.Join(multiErr, testResults.err)
		}

		results = append(results, testResults.results...)
	}

	if multiErr != nil {
		return results, fmt.Errorf("error running package %s tests: %w", testType, multiErr)
	}

	return results, nil
}

// run method delegates execution of tests to the given test runner.
func run(ctx context.Context, tester Tester) ([]TestResult, error) {
	results, err := tester.Run(ctx)
	tdErr := tester.TearDown(ctx)
	if err != nil {
		return nil, fmt.Errorf("could not complete test run: %w", err)
	}
	if tdErr != nil {
		return results, fmt.Errorf("could not teardown test runner: %w", tdErr)
	}
	return results, nil
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

func PackageHasDataStreams(manifest *packages.PackageManifest) (bool, error) {
	switch manifest.Type {
	case "integration":
		return true, nil
	case "input", "content":
		return false, nil
	default:
		return false, fmt.Errorf("unexpected package type %q", manifest.Type)
	}
}

func AnySkipConfig(configs ...*SkipConfig) *SkipConfig {
	for _, config := range configs {
		if config != nil {
			return config
		}
	}
	return nil
}
