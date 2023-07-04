// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package system

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jedib0t/go-pretty/table"
	"github.com/jedib0t/go-pretty/text"

	"github.com/elastic/elastic-package/internal/benchrunner/reporters"
	"github.com/elastic/elastic-package/internal/elasticsearch/ingest"
)

type report struct {
	Info struct {
		Benchmark            string
		Description          string
		RunID                string
		Package              string
		StartTs              int64
		EndTs                int64
		Duration             time.Duration
		GeneratedCorporaFile string
	}
	Parameters struct {
		PackageVersion      string
		Input               string
		Vars                map[string]interface{}
		DataStream          dataStream
		WarmupTimePeriod    time.Duration
		BenchmarkTimePeriod time.Duration
		WaitForDataTimeout  time.Duration
		Corpora             corpora
	}
	ClusterName         string
	Nodes               int
	DataStreamStats     *ingest.DataStreamStats
	IngestPipelineStats map[string]ingest.PipelineStatsMap
	DiskUsage           map[string]ingest.DiskUsage
	TotalHits           int
}

func createReport(benchName, corporaFile string, s *scenario, sum *metricsSummary) (reporters.Reportable, error) {
	r := newReport(benchName, corporaFile, s, sum)
	human := reporters.NewReport(s.Package, reportHumanFormat(r))

	jsonBytes, err := reportJSONFormat(r)
	if err != nil {
		return nil, fmt.Errorf("rendering JSON report: %w", err)
	}

	jsonFile := reporters.NewFileReport(s.Package, fmt.Sprintf("system/%s/report.json", sum.RunID), jsonBytes)

	mr := reporters.NewMultiReport(s.Package, []reporters.Reportable{human, jsonFile})

	return mr, nil
}

func newReport(benchName, corporaFile string, s *scenario, sum *metricsSummary) *report {
	var report report
	report.Info.Benchmark = benchName
	report.Info.Description = s.Description
	report.Info.RunID = sum.RunID
	report.Info.Package = s.Package
	report.Info.StartTs = sum.CollectionStartTs
	report.Info.EndTs = sum.CollectionEndTs
	report.Info.Duration = time.Duration(sum.CollectionEndTs-sum.CollectionStartTs) * time.Second
	report.Info.GeneratedCorporaFile = corporaFile
	report.Parameters.PackageVersion = s.Version
	report.Parameters.Input = s.Input
	report.Parameters.Vars = s.Vars
	report.Parameters.DataStream = s.DataStream
	report.Parameters.WarmupTimePeriod = s.WarmupTimePeriod
	report.Parameters.BenchmarkTimePeriod = s.BenchmarkTimePeriod
	report.Parameters.WaitForDataTimeout = s.WaitForDataTimeout
	report.Parameters.Corpora = s.Corpora
	report.ClusterName = sum.ClusterName
	report.Nodes = sum.Nodes
	report.DataStreamStats = sum.DataStreamStats
	report.IngestPipelineStats = sum.IngestPipelineStats
	report.DiskUsage = sum.DiskUsage
	report.TotalHits = sum.TotalHits
	return &report
}

func reportJSONFormat(r *report) ([]byte, error) {
	b, err := json.MarshalIndent(r, "", "\t")
	if err != nil {
		return nil, err
	}
	return b, nil
}

func reportHumanFormat(r *report) []byte {
	var report strings.Builder
	report.WriteString(renderBenchmarkTable(
		"info",
		"benchmark", r.Info.Benchmark,
		"description", r.Info.Description,
		"run ID", r.Info.RunID,
		"package", r.Info.Package,
		"start ts (s)", r.Info.StartTs,
		"end ts (s)", r.Info.EndTs,
		"duration", r.Info.Duration,
		"generated corpora file", r.Info.GeneratedCorporaFile,
	) + "\n")

	pkvs := []interface{}{
		"package version", r.Parameters.PackageVersion,
		"input", r.Parameters.Input,
	}

	for k, v := range r.Parameters.Vars {
		pkvs = append(pkvs, fmt.Sprintf("vars.%s", k), v)
	}

	pkvs = append(pkvs, "data_stream.name", r.Parameters.DataStream.Name)

	for k, v := range r.Parameters.DataStream.Vars {
		pkvs = append(pkvs, fmt.Sprintf("data_stream.vars.%s", k), v)
	}

	pkvs = append(pkvs,
		"warmup time period", r.Parameters.WarmupTimePeriod,
		"benchmark time period", r.Parameters.BenchmarkTimePeriod,
		"wait for data timeout", r.Parameters.WaitForDataTimeout,
	)

	if r.Parameters.Corpora.Generator != nil {
		pkvs = append(pkvs,
			"corpora.generator.size", r.Parameters.Corpora.Generator.Size,
			"corpora.generator.template.path", r.Parameters.Corpora.Generator.Template.Path,
			"corpora.generator.template.raw", r.Parameters.Corpora.Generator.Template.Raw,
			"corpora.generator.template.type", r.Parameters.Corpora.Generator.Template.Type,
			"corpora.generator.config.path", r.Parameters.Corpora.Generator.Config.Path,
			"corpora.generator.config.raw", r.Parameters.Corpora.Generator.Config.Raw,
			"corpora.generator.fields.path", r.Parameters.Corpora.Generator.Fields.Path,
			"corpora.generator.fields.raw", r.Parameters.Corpora.Generator.Fields.Raw,
		)
	}

	if r.Parameters.Corpora.InputService != nil {
		pkvs = append(pkvs,
			"corpora.input_service.name", r.Parameters.Corpora.InputService.Name,
			"corpora.input_service.signal", r.Parameters.Corpora.InputService.Signal,
		)
	}

	report.WriteString(renderBenchmarkTable("parameters", pkvs...) + "\n")

	report.WriteString(renderBenchmarkTable(
		"cluster info",
		"name", r.ClusterName,
		"nodes", r.Nodes,
	) + "\n")

	report.WriteString(renderBenchmarkTable(
		"data stream stats",
		"data stream", r.DataStreamStats.DataStream,
		"approx total docs ingested", r.TotalHits,
		"backing indices", r.DataStreamStats.BackingIndices,
		"store size bytes", r.DataStreamStats.StoreSizeBytes,
		"maximum ts (ms)", r.DataStreamStats.MaximumTimestamp,
	) + "\n")

	for index, du := range r.DiskUsage {
		adu := du.AllFields
		report.WriteString(renderBenchmarkTable(
			fmt.Sprintf("disk usage for index %s (for all fields)", index),
			"total", adu.Total,
			"inverted_index.total", adu.InvertedIndex.Total,
			"inverted_index.stored_fields", adu.StoredFields,
			"inverted_index.doc_values", adu.DocValues,
			"inverted_index.points", adu.Points,
			"inverted_index.norms", adu.Norms,
			"inverted_index.term_vectors", adu.TermVectors,
			"inverted_index.knn_vectors", adu.KnnVectors,
		) + "\n")
	}

	for node, pStats := range r.IngestPipelineStats {
		for pipeline, stats := range pStats {
			var kvs []interface{}
			for _, procStats := range stats.Processors {
				str := fmt.Sprintf(
					"Count: %d | Failed: %d | Time: %s",
					procStats.Stats.Count,
					procStats.Stats.Failed,
					time.Duration(procStats.Stats.TimeInMillis)*time.Millisecond,
				)
				kvs = append(kvs, fmt.Sprintf("%s (%s)", procStats.Type, procStats.Extra), str)
			}
			report.WriteString(renderBenchmarkTable(
				fmt.Sprintf("pipeline %s stats in node %s", pipeline, node),
				kvs...,
			) + "\n")
		}
	}

	return []byte(report.String())
}

func renderBenchmarkTable(title string, kv ...interface{}) string {
	t := table.NewWriter()
	t.SetStyle(table.StyleRounded)
	t.SetTitle(title)
	t.SetColumnConfigs([]table.ColumnConfig{
		{
			Number: 2,
			Align:  text.AlignRight,
		},
	})
	for i := 0; i < len(kv)-1; i += 2 {
		t.AppendRow(table.Row{kv[i], kv[i+1]})
	}
	return t.Render()
}
