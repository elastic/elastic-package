// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package benchrunner

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/elasticsearch"
)

// BenchType represents the various supported benchmark types
type BenchType string

// BenchOptions contains benchmark runner options.
type BenchOptions struct {
	BenchmarkFolder BenchmarkFolder
	PackageRootPath string
	API             *elasticsearch.API
}

// BenchRunner is the interface all benchmark runners must implement.
type BenchRunner interface {
	// Type returns the benchmark runner's type.
	Type() BenchType

	// String returns the human-friendly name of the benchmark runner.
	String() string

	// Run executes the benchmark runner.
	Run(BenchOptions) (*Result, error)

	// TearDown cleans up any benchmark runner resources. It must be called
	// after the benchmark runner has finished executing.
	TearDown() error
}

var runners = map[BenchType]BenchRunner{}

// Result contains a single benchmark's results
type Result struct {
	// Package to which this benchmark result belongs.
	Package string

	// BenchType indicates the type of benchmark.
	BenchType BenchType

	// Data stream to which this benchmark result belongs.
	DataStream string

	// Time elapsed from running a benchmark case to arriving at its result.
	TimeElapsed time.Duration

	// If there was an error while running the benchmark case, description
	// of the error. An error is when the benchmark cannot complete execution due
	// to an unexpected runtime error in the benchmark execution.
	ErrorMsg string

	// Benchmark results.
	Benchmark *BenchmarkResult
}

// ResultComposer wraps a Result and provides convenience methods for
// manipulating this Result.
type ResultComposer struct {
	Result
	StartTime time.Time
}

// NewResultComposer returns a new ResultComposer with the StartTime
// initialized to now.
func NewResultComposer(tr Result) *ResultComposer {
	return &ResultComposer{
		Result:    tr,
		StartTime: time.Now(),
	}
}

// WithError sets an error on the benchmark result wrapped by ResultComposer.
func (rc *ResultComposer) WithError(err error) ([]Result, error) {
	rc.TimeElapsed = time.Since(rc.StartTime)
	if err == nil {
		return []Result{rc.Result}, nil
	}

	rc.ErrorMsg += err.Error()
	return []Result{rc.Result}, err
}

// WithSuccess marks the benchmark result wrapped by ResultComposer as successful.
func (rc *ResultComposer) WithSuccess() ([]Result, error) {
	return rc.WithError(nil)
}

// BenchmarkFolder encapsulates the benchmark folder path and names of the package + data stream
// to which the benchmark folder belongs.
type BenchmarkFolder struct {
	Path       string
	Package    string
	DataStream string
}

// FindBenchmarkFolders finds benchmark folders for the given package and, optionally, benchmark type and data streams
func FindBenchmarkFolders(packageRootPath string, dataStreams []string, benchType BenchType) ([]BenchmarkFolder, error) {
	// Expected folder structure:
	// <packageRootPath>/
	//   data_stream/
	//     <dataStream>/
	//       _dev/
	//         benchmark/
	//           <benchType>/

	benchTypeGlob := "*"
	if benchType != "" {
		benchTypeGlob = string(benchType)
	}

	var paths []string
	if len(dataStreams) == 0 {
		return nil, errors.New("benchmarks can only be defined at the data_stream level")
	}

	sort.Strings(dataStreams)
	for _, dataStream := range dataStreams {
		p, err := findBenchFolderPaths(packageRootPath, dataStream, benchTypeGlob)
		if err != nil {
			return nil, err
		}

		paths = append(paths, p...)
	}

	folders := make([]BenchmarkFolder, len(paths))
	_, pkg := filepath.Split(packageRootPath)
	for idx, p := range paths {
		relP := strings.TrimPrefix(p, packageRootPath)
		parts := strings.Split(relP, string(filepath.Separator))
		dataStream := parts[2]

		folder := BenchmarkFolder{
			p,
			pkg,
			dataStream,
		}

		folders[idx] = folder
	}

	return folders, nil
}

// RegisterRunner method registers the benchmark runner.
func RegisterRunner(runner BenchRunner) {
	runners[runner.Type()] = runner
}

// Run method delegates execution to the registered benchmark runner, based on the benchmark type.
func Run(benchType BenchType, options BenchOptions) (*Result, error) {
	runner, defined := runners[benchType]
	if !defined {
		return nil, fmt.Errorf("unregistered runner benchmark: %s", benchType)
	}

	result, err := runner.Run(options)
	tdErr := runner.TearDown()
	if err != nil {
		return nil, errors.Wrap(err, "could not complete benchmark run")
	}
	if tdErr != nil {
		return result, errors.Wrap(err, "could not teardown benchmark runner")
	}
	return result, nil
}

// BenchRunners returns registered benchmark runners.
func BenchRunners() map[BenchType]BenchRunner {
	return runners
}

// findBenchFoldersPaths can only be called for benchmark runners that require benchmarks to be defined
// at the data stream level.
func findBenchFolderPaths(packageRootPath, dataStreamGlob, benchTypeGlob string) ([]string, error) {
	benchFoldersGlob := filepath.Join(packageRootPath, "data_stream", dataStreamGlob, "_dev", "benchmark", benchTypeGlob)
	paths, err := filepath.Glob(benchFoldersGlob)
	if err != nil {
		return nil, errors.Wrap(err, "error finding benchmark folders")
	}
	return paths, err
}
