// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package pipeline

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/elastic/elastic-package/internal/benchrunner"
	"github.com/elastic/elastic-package/internal/elasticsearch/ingest"
	"github.com/elastic/elastic-package/internal/packages"
)

const (
	// BenchType defining pipeline benchmarks.
	BenchType benchrunner.BenchType = "pipeline"

	expectedTestResultSuffix = "-expected.json"
	configTestSuffixYAML     = "-config.yml"
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
		return fmt.Errorf("uninstalling ingest pipelines failed: %w", err)
	}
	return nil
}

func (r *runner) run() (*benchrunner.Result, error) {
	dataStreamPath, found, err := packages.FindDataStreamRootForPath(r.options.Folder.Path)
	if err != nil {
		return nil, fmt.Errorf("locating data_stream root failed: %w", err)
	}
	if !found {
		return nil, errors.New("data stream root not found")
	}

	var entryPipeline string
	entryPipeline, r.pipelines, err = ingest.InstallDataStreamPipelines(r.options.API, dataStreamPath)
	if err != nil {
		return nil, fmt.Errorf("installing ingest pipelines failed: %w", err)
	}

	start := time.Now()
	result := &benchrunner.Result{
		BenchType:  BenchType + " benchmark",
		Package:    r.options.Folder.Package,
		DataStream: r.options.Folder.DataStream,
	}

	b, err := r.loadBenchmark()
	if err != nil {
		return nil, fmt.Errorf("loading benchmark failed: %w", err)
	}

	if result.Benchmark, err = r.benchmarkPipeline(b, entryPipeline); err != nil {
		result.ErrorMsg = err.Error()
	}

	result.TimeElapsed = time.Since(start)

	return result, nil
}

func (r *runner) listBenchmarkFiles() ([]string, error) {
	fis, err := os.ReadDir(r.options.Folder.Path)
	if err != nil {
		return nil, fmt.Errorf("reading pipeline benchmarks failed (path: %s): %w", r.options.Folder.Path, err)
	}

	var files []string
	for _, fi := range fis {
		if fi.Name() == configYAML ||
			// since pipeline tests might be included we need to
			// exclude the expected and config files for them
			strings.HasSuffix(fi.Name(), expectedTestResultSuffix) ||
			strings.HasSuffix(fi.Name(), configTestSuffixYAML) {
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
		benchPath := filepath.Join(r.options.Folder.Path, benchFile)
		benchData, err := os.ReadFile(benchPath)
		if err != nil {
			return nil, fmt.Errorf("reading input file failed (benchPath: %s): %w", benchPath, err)
		}

		ext := filepath.Ext(benchFile)
		var entries []json.RawMessage
		switch ext {
		case ".json":
			entries, err = readBenchmarkEntriesForEvents(benchData)
			if err != nil {
				return nil, fmt.Errorf("reading benchmark case entries for events failed (benchmarkPath: %s): %w", benchPath, err)
			}
		case ".log":
			entries, err = readBenchmarkEntriesForRawInput(benchData)
			if err != nil {
				return nil, fmt.Errorf("creating benchmark case entries for raw input failed (benchmarkPath: %s): %w", benchPath, err)
			}
		default:
			return nil, fmt.Errorf("unsupported extension for benchmark case file (ext: %s)", ext)
		}
		allEntries = append(allEntries, entries...)
	}

	config, err := readConfig(r.options.Folder.Path)
	if err != nil {
		return nil, fmt.Errorf("reading config for benchmark failed (benchPath: %s): %w", r.options.Folder.Path, err)
	}

	tc, err := createBenchmark(allEntries, config)
	if err != nil {
		return nil, fmt.Errorf("can't create benchmark case (benchmarkPath: %s): %w", r.options.Folder.Path, err)
	}
	return tc, nil
}

func init() {
	benchrunner.RegisterRunner(&runner{})
}
