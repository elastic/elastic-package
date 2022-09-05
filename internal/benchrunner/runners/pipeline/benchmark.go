// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package pipeline

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/benchrunner"
	"github.com/elastic/elastic-package/internal/elasticsearch/ingest"
)

const (
	// How many top processors to return.
	numTopProcs = 10
)

func (r *runner) benchmarkPipeline(b *benchmark, entryPipeline string) (*benchrunner.BenchmarkResult, error) {
	// Run benchmark
	bench, err := r.benchmarkIngest(b, entryPipeline)
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
	asPercentageOfTotalTime := func(perf processorPerformance) benchrunner.BenchmarkValue {
		return benchrunner.BenchmarkValue{
			Name:        perf.key,
			Description: perf.key,
			Unit:        "%",
			Value:       time.Duration(perf.value).Seconds() * 100 / bench.elapsed.Seconds(),
		}
	}
	asDuration := func(perf processorPerformance) benchrunner.BenchmarkValue {
		return benchrunner.BenchmarkValue{
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
	result := &benchrunner.BenchmarkResult{
		Name: fmt.Sprintf("pipeline benchmark for %s/%s", r.options.BenchmarkFolder.Package, r.options.BenchmarkFolder.DataStream),
		Parameters: []benchrunner.BenchmarkValue{
			{
				Name:  "package",
				Value: r.options.BenchmarkFolder.Package,
			},
			{
				Name:  "data_stream",
				Value: r.options.BenchmarkFolder.DataStream,
			},
			{
				Name:  "source doc count",
				Value: len(b.events),
			},
			{
				Name:  "doc count",
				Value: bench.numDocs,
			},
		},
		Tests: []benchrunner.BenchmarkTest{
			{
				Name: "ingest performance",
				Results: []benchrunner.BenchmarkValue{
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

func (r *runner) benchmarkIngest(b *benchmark, entryPipeline string) (ingestResult, error) {
	baseDocs := resizeDocs(b.events, b.config.NumDocs)
	return r.runSingleBenchmark(entryPipeline, baseDocs)
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
	mapFn     func(processorPerformance) benchrunner.BenchmarkValue
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

func (agg aggregation) collect(fn mapFn) ([]benchrunner.BenchmarkValue, error) {
	if agg.err != nil {
		return nil, agg.err
	}
	r := make([]benchrunner.BenchmarkValue, len(agg.result))
	for idx := range r {
		r[idx] = fn(agg.result[idx])
	}
	return r, nil
}

func (r *runner) runSingleBenchmark(entryPipeline string, docs []json.RawMessage) (ingestResult, error) {
	if len(docs) == 0 {
		return ingestResult{}, errors.New("no docs supplied for benchmark")
	}

	if _, err := ingest.SimulatePipeline(r.options.API, entryPipeline, docs); err != nil {
		return ingestResult{}, errors.Wrap(err, "simulate failed")
	}

	stats, err := ingest.GetPipelineStats(r.options.API, r.pipelines)
	if err != nil {
		return ingestResult{}, errors.Wrap(err, "error fetching pipeline stats")
	}
	var took time.Duration
	for _, pSt := range stats {
		took += time.Millisecond * time.Duration(pSt.TimeInMillis)
	}
	return ingestResult{
		pipelines: r.pipelines,
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
