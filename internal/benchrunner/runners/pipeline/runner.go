// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/elastic/elastic-package/internal/benchrunner"
	"github.com/elastic/elastic-package/internal/benchrunner/reporters"
	"github.com/elastic/elastic-package/internal/elasticsearch/ingest"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/testrunner"
)

const (
	// BenchType defining pipeline benchmarks.
	BenchType benchrunner.Type = "pipeline"

	expectedTestResultSuffix = "-expected.json"
	configTestSuffixYAML     = "-config.yml"
)

type runner struct {
	options       Options
	entryPipeline string
	pipelines     []ingest.Pipeline
}

func NewPipelineBenchmark(opts Options) benchrunner.Runner {
	return &runner{options: opts}
}

func (r *runner) SetUp(ctx context.Context) error {
	dataStreamRoot, found, err := packages.FindDataStreamRootForPath(r.options.Folder.Path)
	if err != nil {
		return fmt.Errorf("locating data_stream root failed: %w", err)
	}
	if !found {
		return errors.New("data stream root not found")
	}

	r.entryPipeline, r.pipelines, err = ingest.InstallDataStreamPipelines(ctx, r.options.API, dataStreamRoot, r.options.RepositoryRoot)
	if err != nil {
		return fmt.Errorf("installing ingest pipelines failed: %w", err)
	}

	return nil
}

// TearDown shuts down the pipeline benchmark runner.
func (r *runner) TearDown(ctx context.Context) error {
	if err := ingest.UninstallPipelines(ctx, r.options.API, r.pipelines); err != nil {
		return fmt.Errorf("uninstalling ingest pipelines failed: %w", err)
	}
	return nil
}

// Run runs the pipeline benchmarks defined under the given folder
func (r *runner) Run(ctx context.Context) (reporters.Reportable, error) {
	return r.run(ctx)
}

func (r *runner) run(ctx context.Context) (reporters.Reportable, error) {
	b, err := r.loadBenchmark()
	if err != nil {
		return nil, fmt.Errorf("loading benchmark failed: %w", err)
	}

	benchmark, err := r.benchmarkPipeline(ctx, b, r.entryPipeline)
	if err != nil {
		return nil, err
	}

	formattedReport, err := formatResult(r.options.Format, benchmark)
	if err != nil {
		return nil, err
	}

	switch r.options.Format {
	case ReportFormatHuman:
		return reporters.NewReport(r.options.Folder.Package, formattedReport), nil
	}

	return reporters.NewFileReport(
		r.options.BenchName,
		filenameByFormat(r.options.BenchName, r.options.Format),
		formattedReport,
	), nil
}

// FindBenchmarkFolders finds benchmark folders for the given package and, optionally, benchmark type and data streams
func FindBenchmarkFolders(packageRoot string, dataStreams []string) ([]testrunner.TestFolder, error) {
	// Expected folder structure:
	// <packageRoot>/
	//   data_stream/
	//     <dataStream>/
	//       _dev/
	//         benchmark/
	//           <benchType>/

	var paths []string
	if len(dataStreams) > 0 {
		sort.Strings(dataStreams)
		for _, dataStream := range dataStreams {
			p, err := findBenchFolderPaths(packageRoot, dataStream)
			if err != nil {
				return nil, err
			}

			paths = append(paths, p...)
		}
	} else {
		p, err := findBenchFolderPaths(packageRoot, "*")
		if err != nil {
			return nil, err
		}

		paths = p
	}

	sort.Strings(dataStreams)
	for _, dataStream := range dataStreams {
		p, err := findBenchFolderPaths(packageRoot, dataStream)
		if err != nil {
			return nil, err
		}

		paths = append(paths, p...)
	}

	folders := make([]testrunner.TestFolder, len(paths))
	_, pkg := filepath.Split(packageRoot)
	for idx, p := range paths {
		relP := strings.TrimPrefix(p, packageRoot)
		parts := strings.Split(relP, string(filepath.Separator))
		dataStream := parts[2]

		folder := testrunner.TestFolder{
			Path:       p,
			Package:    pkg,
			DataStream: dataStream,
		}

		folders[idx] = folder
	}

	return folders, nil
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

// findBenchFoldersPaths can only be called for benchmark runners that require benchmarks to be defined
// at the data stream level.
func findBenchFolderPaths(packageRoot, dataStreamGlob string) ([]string, error) {
	benchFoldersGlob := filepath.Join(packageRoot, "data_stream", dataStreamGlob, "_dev", "benchmark", "pipeline")
	paths, err := filepath.Glob(benchFoldersGlob)
	if err != nil {
		return nil, fmt.Errorf("error finding benchmark folders: %w", err)
	}
	return paths, err
}
