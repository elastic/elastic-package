// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/elastic/elastic-package/internal/elasticsearch/ingest"
)

type BenchmarkResult struct {
	// XMLName is a zero-length field used as an annotation for XML marshaling.
	XMLName struct{} `xml:"group" json:"-"`
	// Type of benchmark
	Type string `xml:"type" json:"type"`
	// Package of the benchmark
	Package string `xml:"package" json:"package"`
	// DataStream of the benchmark
	DataStream string `xml:"data_stream" json:"data_stream"`
	// Description of the benchmark run.
	Description string `xml:"description,omitempty" json:"description,omitempty"`
	// Parameters used for this benchmark.
	Parameters []BenchmarkValue `xml:"parameters,omitempty" json:"parameters,omitempty"`
	// Tests holds the results for the benchmark.
	Tests []BenchmarkTest `xml:"test" json:"test"`
}

// BenchmarkTest models a particular test performed during a benchmark.
type BenchmarkTest struct {
	// Name of this test.
	Name string `xml:"name" json:"name"`
	// Detailed benchmark tests will be printed to the output but not
	// included in file reports.
	Detailed bool `xml:"-" json:"-"`
	// Description of this test.
	Description string `xml:"description,omitempty" json:"description,omitempty"`
	// Parameters for this test.
	Parameters []BenchmarkValue `xml:"parameters,omitempty" json:"parameters,omitempty"`
	// Results of the test.
	Results []BenchmarkValue `xml:"result" json:"result"`
}

// BenchmarkValue represents a value (result or parameter)
// with an optional associated unit.
type BenchmarkValue struct {
	// Name of the value.
	Name string `xml:"name" json:"name"`
	// Description of the value.
	Description string `xml:"description,omitempty" json:"description,omitempty"`
	// Unit used for this value.
	Unit string `xml:"unit,omitempty" json:"unit,omitempty"`
	// Value is of any type, usually string or numeric.
	Value interface{} `xml:"value,omitempty" json:"value,omitempty"`
}

// String returns a BenchmarkValue's value nicely-formatted.
func (p BenchmarkValue) String() (r string) {
	if str, ok := p.Value.(fmt.Stringer); ok {
		return str.String()
	}
	if float, ok := p.Value.(float64); ok {
		r = fmt.Sprintf("%.02f", float)
	} else {
		r = fmt.Sprintf("%v", p.Value)
	}
	if p.Unit != "" {
		r += p.Unit
	}
	return r
}

func (r *runner) benchmarkPipeline(ctx context.Context, b *benchmark, entryPipeline string) (*BenchmarkResult, error) {
	// Run benchmark
	bench, err := r.benchmarkIngest(ctx, b, entryPipeline)
	if err != nil {
		return nil, fmt.Errorf("failed running benchmark: %w", err)
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
	asPercentageOfTotalTime := func(perf processorPerformance) BenchmarkValue {
		return BenchmarkValue{
			Name:        perf.key,
			Description: perf.key,
			Unit:        "%",
			Value:       time.Duration(perf.value).Seconds() * 100 / bench.elapsed.Seconds(),
		}
	}
	asDuration := func(perf processorPerformance) BenchmarkValue {
		return BenchmarkValue{
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
		top(r.options.NumTopProcs).
		collect(asPercentageOfTotalTime)
	if err != nil {
		return nil, err
	}

	topRelProcs, err := bench.
		aggregate(processorKey, byRelativeTime).
		filter(nonZero).
		sort(descending).
		top(r.options.NumTopProcs).
		collect(asDuration)
	if err != nil {
		return nil, err
	}

	// Build result
	result := &BenchmarkResult{
		Type:        string(BenchType),
		Package:     r.options.Folder.Package,
		DataStream:  r.options.Folder.DataStream,
		Description: fmt.Sprintf("pipeline benchmark for %s/%s", r.options.Folder.Package, r.options.Folder.DataStream),
		Parameters: []BenchmarkValue{
			{
				Name:  "source_doc_count",
				Value: len(b.events),
			},
			{
				Name:  "doc_count",
				Value: bench.numDocs,
			},
		},
		Tests: []BenchmarkTest{
			{
				Name: "pipeline_performance",
				Results: []BenchmarkValue{
					{
						Name:        "processing_time",
						Description: "time elapsed in pipeline processors",
						Value:       bench.elapsed.Seconds(),
						Unit:        "s",
					},
					{
						Name:        "eps",
						Description: "processed events per second",
						Value:       float64(bench.numDocs) / bench.elapsed.Seconds(),
					},
				},
			},
			{
				Name:        "procs_by_total_time",
				Description: fmt.Sprintf("top %d processors by time spent", r.options.NumTopProcs),
				Results:     topAbsProc,
			},
			{
				Name:        "procs_by_avg_time_per_doc",
				Description: fmt.Sprintf("top %d processors by average time per document", r.options.NumTopProcs),
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

func (r *runner) benchmarkIngest(ctx context.Context, b *benchmark, entryPipeline string) (ingestResult, error) {
	baseDocs := resizeDocs(b.events, b.config.NumDocs)
	return r.runSingleBenchmark(ctx, entryPipeline, baseDocs)
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
	mapFn     func(processorPerformance) BenchmarkValue
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

func (agg aggregation) collect(fn mapFn) ([]BenchmarkValue, error) {
	if agg.err != nil {
		return nil, agg.err
	}
	r := make([]BenchmarkValue, len(agg.result))
	for idx := range r {
		r[idx] = fn(agg.result[idx])
	}
	return r, nil
}

func (r *runner) runSingleBenchmark(ctx context.Context, entryPipeline string, docs []json.RawMessage) (ingestResult, error) {
	if len(docs) == 0 {
		return ingestResult{}, errors.New("no docs supplied for benchmark")
	}

	if _, err := ingest.SimulatePipeline(ctx, r.options.API, entryPipeline, docs, "test-generic-default"); err != nil {
		return ingestResult{}, fmt.Errorf("simulate failed: %w", err)
	}

	stats, err := ingest.GetPipelineStats(r.options.API, r.pipelines)
	if err != nil {
		return ingestResult{}, fmt.Errorf("error fetching pipeline stats: %w", err)
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
