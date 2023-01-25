// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package ingest

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/elastic/elastic-package/internal/elasticsearch"
)

// StatsRecord contains stats for a measurable entity (pipeline, processor, etc.)
type StatsRecord struct {
	Count, Current, Failed int64
	TimeInMillis           int64 `json:"time_in_millis"`
}

// ProcessorStats contains a processor's stats and some metadata.
type ProcessorStats struct {
	Type        string
	Extra       string
	Conditional bool
	Stats       StatsRecord
}

// PipelineStats contains stats for a pipeline.
type PipelineStats struct {
	StatsRecord
	Processors []ProcessorStats
}

// PipelineStatsMap holds the stats for a set of pipelines.
type PipelineStatsMap map[string]PipelineStats

type wrappedProcessor map[string]ProcessorStats

// Extract ProcessorStats from an object in the form:
// `{ "processor_type": { ...ProcessorStats...} }`
func (p wrappedProcessor) extract() (stats ProcessorStats, err error) {
	if len(p) != 1 {
		keys := make([]string, 0, len(p))
		for k := range p {
			keys = append(keys, k)
		}
		return stats, fmt.Errorf("can't extract processor stats. Need a single key in the processor identifier, got %d: %v", len(p), keys)
	}

	// Read single entry in map.
	var processorType string
	for processorType, stats = range p {
	}

	// Handle compound processors in the form compound:[...extra...]
	if off := strings.Index(processorType, ":"); off != -1 {
		stats.Extra = processorType[off+1:]
		processorType = processorType[:off]
	}
	switch stats.Type {
	case processorType:
	case "conditional":
		stats.Conditional = true
	default:
		return stats, fmt.Errorf("can't understand processor identifier '%s' in %+v", processorType, p)
	}
	stats.Type = processorType

	return stats, nil
}

type pipelineStatsRecord struct {
	StatsRecord
	Processors []wrappedProcessor
}

func (r pipelineStatsRecord) extract() (stats PipelineStats, err error) {
	stats = PipelineStats{
		StatsRecord: r.StatsRecord,
		Processors:  make([]ProcessorStats, len(r.Processors)),
	}
	for idx, wrapped := range r.Processors {
		if stats.Processors[idx], err = wrapped.extract(); err != nil {
			return stats, fmt.Errorf("extracting processor %d: %s", idx, err)
		}
	}
	return stats, nil
}

type pipelineStatsRecordMap map[string]pipelineStatsRecord

type pipelinesStatsNode struct {
	Ingest struct {
		Pipelines pipelineStatsRecordMap
	}
}

type pipelinesStatsResponse struct {
	Nodes map[string]pipelinesStatsNode
}

func GetPipelineStats(esClient *elasticsearch.API, pipelines []Pipeline) (stats PipelineStatsMap, err error) {
	statsReq := esClient.Nodes.Stats.WithFilterPath("nodes.*.ingest.pipelines")
	resp, err := esClient.Nodes.Stats(statsReq)
	if err != nil {
		return nil, fmt.Errorf("node Stats API call failed: %s", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read Stats API response body: %s", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("unexpected response status for Node Stats (%d): %s: %s", resp.StatusCode, resp.Status(), elasticsearch.NewError(body))
	}
	return getPipelineStats(body, pipelines)
}

func getPipelineStats(body []byte, pipelines []Pipeline) (stats PipelineStatsMap, err error) {
	var statsResponse pipelinesStatsResponse
	if err = json.Unmarshal(body, &statsResponse); err != nil {
		return nil, fmt.Errorf("error decoding Node Stats response: %s", err)
	}

	if nodeCount := len(statsResponse.Nodes); nodeCount != 1 {
		return nil, fmt.Errorf("need exactly one ES node in stats response (got %d)", nodeCount)
	}
	var nodePipelines map[string]pipelineStatsRecord
	for _, node := range statsResponse.Nodes {
		nodePipelines = node.Ingest.Pipelines
	}
	stats = make(PipelineStatsMap, len(pipelines))
	var missing []string
	for _, requested := range pipelines {
		if pStats, found := nodePipelines[requested.Name]; found {
			if stats[requested.Name], err = pStats.extract(); err != nil {
				return stats, fmt.Errorf("converting pipeline %s: %s", requested.Name, err)
			}
		} else {
			missing = append(missing, requested.Name)
		}
	}
	if len(missing) != 0 {
		return stats, fmt.Errorf("node Stats response is missing expected pipelines: %s", strings.Join(missing, ", "))
	}

	return stats, nil
}
