// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package pipeline

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"time"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/elasticsearch/ingest"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/testrunner"
)

const (
	// How many attempts to make while approximating
	// benchmark duration by adjusting document count.
	durationAdjustMaxTries = 3

	// How close to the target duration for a benchmark
	// to be is accepted.
	durationToleranceSeconds = 0.5

	// Same, but as a percentage of the target duration.
	durationTolerancePercent = 0.9

	// Minimum acceptable length for a benchmark result.
	minDurationSeconds = 0.001 // 1ms

	// How many top processors to return.
	numTopProcs = 10
)

func BenchmarkPipeline(options testrunner.TestOptions) (*testrunner.BenchmarkResult, error) {
	// Load all test documents
	docs, err := loadAllTestDocs(options.TestFolder.Path)
	if err != nil {
		return nil, errors.Wrap(err, "failed loading test documents")
	}

	// Run benchmark
	bench, err := benchmarkIngest(options, docs)
	if err != nil {
		return nil, errors.Wrap(err, "failed running benchmark")
	}

	// Extract performance measurements
	processorKey := func(pipeline ingest.Pipeline, processor ingest.Processor) string {
		// Don't want to use pipeline processors time in benchmark, as they
		// aggregate the time of all the processors in their pipeline.
		if processor.Type == "pipeline" {
			return ""
		}
		return fmt.Sprintf("%s @ %s:%d", processor.Type, pipeline.Filename(), processor.FirstLine)
	}
	byAbsoluteTime := func(record ingest.StatsRecord) int64 {
		return record.TimeInMillis * int64(time.Millisecond)
	}
	byRelativeTime := func(record ingest.StatsRecord) int64 {
		if record.Count == 0 {
			return 0
		}
		return record.TimeInMillis * int64(time.Millisecond) / record.Count
	}
	asPercentageOfTotalTime := func(perf processorPerformance) testrunner.BenchmarkValue {
		return testrunner.BenchmarkValue{
			Name:        perf.key,
			Description: perf.key,
			Unit:        "%",
			Value:       time.Duration(perf.value).Seconds() * 100 / bench.elapsed.Seconds(),
		}
	}
	asDuration := func(perf processorPerformance) testrunner.BenchmarkValue {
		return testrunner.BenchmarkValue{
			Name:        perf.key,
			Description: perf.key,
			Value:       time.Duration(perf.value),
		}
	}
	nonZero := func(p processorPerformance) bool {
		// This removes pipeline processors (marked with key="") and zero values.
		return p.key != "" && p.value != 0
	}

	topAbsProc, err := bench.
		aggregate(processorKey, byAbsoluteTime).
		filter(nonZero).
		sort(descending).
		top(numTopProcs).
		collect(asPercentageOfTotalTime)
	if err != nil {
		return nil, err
	}

	topRelProcs, err := bench.
		aggregate(processorKey, byRelativeTime).
		filter(nonZero).
		sort(descending).
		top(numTopProcs).
		collect(asDuration)
	if err != nil {
		return nil, err
	}

	// Build result
	result := &testrunner.BenchmarkResult{
		Name: fmt.Sprintf("pipeline benchmark for %s/%s", options.TestFolder.Package, options.TestFolder.DataStream),
		Parameters: []testrunner.BenchmarkValue{
			{
				Name:  "package",
				Value: options.TestFolder.Package,
			},
			{
				Name:  "data_stream",
				Value: options.TestFolder.DataStream,
			},
			{
				Name:  "source doc count",
				Value: len(docs),
			},
			{
				Name:  "doc count",
				Value: bench.numDocs,
			},
		},
		Tests: []testrunner.BenchmarkTest{
			{
				Name: "ingest performance",
				Results: []testrunner.BenchmarkValue{
					{
						Name:        "ingest time",
						Description: "time elapsed in ingest processors",
						Value:       bench.elapsed.Seconds(),
						Unit:        "s",
					},
					{
						Name:        "eps",
						Description: "ingested events per second",
						Value:       float64(bench.numDocs) / bench.elapsed.Seconds(),
					},
				},
			},
			{
				Name:        "processors by total time",
				Detailed:    true,
				Description: fmt.Sprintf("top %d processors by time spent", numTopProcs),
				Results:     topAbsProc,
			},
			{
				Name:        "processors by average time per doc",
				Detailed:    true,
				Description: fmt.Sprintf("top %d processors by average time per document", numTopProcs),
				Results:     topRelProcs,
			},
		},
	}

	return result, nil
}

type ingestResult struct {
	pipelines []ingest.Pipeline
	stats     ingest.PipelineStatsMap
	elapsed   time.Duration
	numDocs   int
}

func benchmarkIngest(options testrunner.TestOptions, baseDocs []json.RawMessage) (ingestResult, error) {
	if options.Benchmark.Duration == time.Duration(0) {
		// Run with a fixed doc count
		return runSingleBenchmark(options, resizeDocs(baseDocs, options.Benchmark.NumDocs))
	}

	// Approximate doc count to target duration
	step, err := runSingleBenchmark(options, baseDocs)
	if err != nil {
		return step, err
	}

	for i, n := 0, len(baseDocs); i < durationAdjustMaxTries && compareFuzzy(step.elapsed, options.Benchmark.Duration) == -1; i++ {
		n = int(seconds(options.Benchmark.Duration) * float64(n) / seconds(step.elapsed))
		baseDocs = resizeDocs(baseDocs, n)
		if step, err = runSingleBenchmark(options, baseDocs); err != nil {
			return step, err
		}
	}
	return step, nil
}

type processorPerformance struct {
	key   string
	value int64
}

type aggregation struct {
	result []processorPerformance
	err    error
}

type (
	keyFn     func(ingest.Pipeline, ingest.Processor) string
	valueFn   func(record ingest.StatsRecord) int64
	mapFn     func(processorPerformance) testrunner.BenchmarkValue
	compareFn func(a, b processorPerformance) bool
	filterFn  func(processorPerformance) bool
)

func (ir ingestResult) aggregate(key keyFn, value valueFn) (agg aggregation) {
	pipelines := make(map[string]ingest.Pipeline, len(ir.pipelines))
	for _, p := range ir.pipelines {
		pipelines[p.Name] = p
	}

	for pipelineName, pipelineStats := range ir.stats {
		pipeline, ok := pipelines[pipelineName]
		if !ok {
			return aggregation{err: fmt.Errorf("unexpected pipeline '%s'", pipelineName)}
		}
		processors, err := pipeline.Processors()
		if err != nil {
			return aggregation{err: err}
		}
		if nSrc, nStats := len(processors), len(pipelineStats.Processors); nSrc != nStats {
			return aggregation{err: fmt.Errorf("pipeline '%s' processor count mismatch. source=%d stats=%d", pipelineName, nSrc, nStats)}
		}
		for procId, procStats := range pipelineStats.Processors {
			agg.result = append(agg.result, processorPerformance{
				key:   key(pipeline, processors[procId]),
				value: value(procStats.Stats),
			})
		}
	}
	return agg
}

func (agg aggregation) sort(compare compareFn) aggregation {
	if agg.err != nil {
		return agg
	}
	sort.Slice(agg.result, func(i, j int) bool {
		return compare(agg.result[i], agg.result[j])
	})
	return agg
}

func ascending(a, b processorPerformance) bool {
	return a.value < b.value
}

func descending(a, b processorPerformance) bool {
	return !ascending(a, b)
}

func (agg aggregation) top(n int) aggregation {
	if n < len(agg.result) {
		agg.result = agg.result[:n]
	}
	return agg
}

func (agg aggregation) filter(keep filterFn) aggregation {
	if agg.err != nil {
		return agg
	}
	o := 0
	for _, entry := range agg.result {
		if keep(entry) {
			agg.result[o] = entry
			o++
		}
	}
	agg.result = agg.result[:o]
	return agg
}

func (agg aggregation) collect(fn mapFn) ([]testrunner.BenchmarkValue, error) {
	if agg.err != nil {
		return nil, agg.err
	}
	r := make([]testrunner.BenchmarkValue, len(agg.result))
	for idx := range r {
		r[idx] = fn(agg.result[idx])
	}
	return r, nil
}

func runSingleBenchmark(options testrunner.TestOptions, docs []json.RawMessage) (ingestResult, error) {
	if len(docs) == 0 {
		return ingestResult{}, errors.New("no docs supplied for benchmark")
	}
	dataStreamPath, found, err := packages.FindDataStreamRootForPath(options.TestFolder.Path)
	if err != nil {
		return ingestResult{}, errors.Wrap(err, "locating data_stream root failed")
	}
	if !found {
		return ingestResult{}, errors.New("data stream root not found")
	}

	testCase := testCase{
		events: docs,
	}
	entryPipeline, pipelines, err := installIngestPipelines(options.API, dataStreamPath)
	if err != nil {
		return ingestResult{}, errors.Wrap(err, "installing ingest pipelines failed")
	}
	defer uninstallIngestPipelines(options.API, pipelines)

	if _, err = simulatePipelineProcessing(options.API, entryPipeline, &testCase); err != nil {
		return ingestResult{}, errors.Wrap(err, "simulate failed")
	}

	stats, err := ingest.GetPipelineStats(options.API, pipelines)
	if err != nil {
		return ingestResult{}, errors.Wrap(err, "error fetching pipeline stats")
	}
	var took time.Duration
	for _, pSt := range stats {
		took += time.Millisecond * time.Duration(pSt.TimeInMillis)
	}
	return ingestResult{
		pipelines: pipelines,
		stats:     stats,
		elapsed:   took,
		numDocs:   len(docs),
	}, nil
}

func resizeDocs(inputDocs []json.RawMessage, want int) []json.RawMessage {
	n := len(inputDocs)
	if n == 0 {
		return nil
	}
	if want == 0 {
		want = 1
	}
	result := make([]json.RawMessage, want)
	for i := 0; i < want; i++ {
		result[i] = inputDocs[i%n]
	}
	return result
}

func seconds(d time.Duration) float64 {
	s := d.Seconds()
	// Don't return durations less than the safe value.
	if s < minDurationSeconds {
		return minDurationSeconds
	}
	return s
}

func compareFuzzy(a, b time.Duration) int {
	sa, sb := seconds(a), seconds(b)
	if sa > sb {
		sa, sb = sb, sa
	}
	if sb-sa <= durationToleranceSeconds || sa/sb >= durationTolerancePercent {
		return 0
	}
	if a < b {
		return -1
	}
	return 1
}

func loadAllTestDocs(testFolderPath string) ([]json.RawMessage, error) {
	testCaseFiles, err := listTestCaseFiles(testFolderPath)
	if err != nil {
		return nil, err
	}

	var docs []json.RawMessage
	for _, file := range testCaseFiles {
		path := filepath.Join(testFolderPath, file)
		tc, err := loadTestCaseFile(path)
		if err != nil {
			return nil, err
		}
		docs = append(docs, tc.events...)
	}
	return docs, err
}
