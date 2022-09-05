// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package pipeline

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/benchrunner"
	"github.com/elastic/elastic-package/internal/elasticsearch/ingest"
	"github.com/elastic/elastic-package/internal/packages"
)

const (
	// BenchType defining pipeline benchmarks.
	BenchType benchrunner.BenchType = "pipeline"
)

type runner struct {
	options   benchrunner.BenchOptions
	pipelines []ingest.Pipeline
}

// Type returns the type of benchmark that can be run by this benchmark runner.
func (r *runner) Type() benchrunner.BenchType {
	return BenchType
}

// String returns the human-friendly name of the benchmark runner.
func (r *runner) String() string {
	return "pipeline"
}

// Run runs the pipeline benchmarks defined under the given folder
func (r *runner) Run(options benchrunner.BenchOptions) (*benchrunner.Result, error) {
	r.options = options
	return r.run()
}

// TearDown shuts down the pipeline benchmark runner.
func (r *runner) TearDown() error {
	if err := ingest.UninstallPipelines(r.options.API, r.pipelines); err != nil {
		return errors.Wrap(err, "uninstalling ingest pipelines failed")
	}
	return nil
}

func (r *runner) run() (*benchrunner.Result, error) {
	dataStreamPath, found, err := packages.FindDataStreamRootForPath(r.options.BenchmarkFolder.Path)
	if err != nil {
		return nil, errors.Wrap(err, "locating data_stream root failed")
	}
	if !found {
		return nil, errors.New("data stream root not found")
	}

	var entryPipeline string
	entryPipeline, r.pipelines, err = ingest.InstallDataStreamPipelines(r.options.API, dataStreamPath)
	if err != nil {
		return nil, errors.Wrap(err, "installing ingest pipelines failed")
	}

	start := time.Now()
	result := &benchrunner.Result{
		BenchType:  BenchType + " benchmark",
		Package:    r.options.BenchmarkFolder.Package,
		DataStream: r.options.BenchmarkFolder.DataStream,
	}

	b, err := r.loadBenchmark()
	if err != nil {
		return nil, errors.Wrap(err, "loading benchmark failed")
	}

	if result.Benchmark, err = r.benchmarkPipeline(b, entryPipeline); err != nil {
		result.ErrorMsg = err.Error()
	}

	result.TimeElapsed = time.Since(start)

	return result, nil
}

func (r *runner) listBenchmarkFiles() ([]string, error) {
	fis, err := os.ReadDir(r.options.BenchmarkFolder.Path)
	if err != nil {
		return nil, errors.Wrapf(err, "reading pipeline benchmarks failed (path: %s)", r.options.BenchmarkFolder.Path)
	}

	var files []string
	for _, fi := range fis {
		if fi.Name() == configYAML {
			continue
		}
		files = append(files, fi.Name())
	}
	return files, nil
}

func (r *runner) loadBenchmark() (*benchmark, error) {
	benchFiles, err := r.listBenchmarkFiles()
	if err != nil {
		return nil, err
	}

	var allEntries []json.RawMessage
	for _, benchFile := range benchFiles {
		benchPath := filepath.Join(r.options.BenchmarkFolder.Path, benchFile)
		benchData, err := os.ReadFile(benchPath)
		if err != nil {
			return nil, errors.Wrapf(err, "reading input file failed (benchPath: %s)", benchPath)
		}

		ext := filepath.Ext(benchFile)
		var entries []json.RawMessage
		switch ext {
		case ".json":
			entries, err = readBenchmarkEntriesForEvents(benchData)
			if err != nil {
				return nil, errors.Wrapf(err, "reading benchmark case entries for events failed (benchmarkPath: %s)", benchPath)
			}
		case ".log":
			entries, err = readBenchmarkEntriesForRawInput(benchData)
			if err != nil {
				return nil, errors.Wrapf(err, "creating benchmark case entries for raw input failed (benchmarkPath: %s)", benchPath)
			}
		default:
			return nil, fmt.Errorf("unsupported extension for benchmark case file (ext: %s)", ext)
		}
		allEntries = append(allEntries, entries...)
	}

	config, err := readConfig(r.options.BenchmarkFolder.Path)
	if err != nil {
		return nil, errors.Wrapf(err, "reading config for benchmark failed (benchPath: %s)", r.options.BenchmarkFolder.Path)
	}

	tc, err := createBenchmark(allEntries, config)
	if err != nil {
		return nil, errors.Wrapf(err, "can't create benchmark case (benchmarkPath: %s)", r.options.BenchmarkFolder.Path)
	}
	return tc, nil
}

func init() {
	benchrunner.RegisterRunner(&runner{})
}
