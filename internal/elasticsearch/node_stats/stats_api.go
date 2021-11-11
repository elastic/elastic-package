// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package node_stats

import (
	"encoding/json"
	"io"
	"strings"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/elasticsearch"
	"github.com/elastic/elastic-package/internal/elasticsearch/pipeline"
)

// StatsRecord contains stats for a measurable entity (pipeline, processor, etc.)
type StatsRecord struct {
	Count, Current, Failed int64
	TimeInMillis           int64 `json:"time_in_millis"`
}

// ProcessorStats contains a processor's stats and some metadata.
type ProcessorStats struct {
	Type, Extra string
	Conditional bool
	Stats       StatsRecord
}

// PipelineStats contains stats for a pipeline.
type PipelineStats struct {
	StatsRecord
	Processors []ProcessorStats
}

// PipelineStatsMap holds the stats for a set of pipelines.
type PipelineStatsMap map[string]*PipelineStats

type wrappedProcessor map[string]ProcessorStats

func (p wrappedProcessor) extract() (stats ProcessorStats, err error) {
	if len(p) != 1 {
		return stats, errors.Errorf("can't extract processor stats. Need a single processor, got %d: %+v", len(p), p)
	}
	for k, v := range p {
		stats = v
		if off := strings.Index(k, ":"); off != -1 {
			stats.Extra = k[off+1:]
			k = k[:off]
		}
		switch v.Type {
		case "conditional":
			stats.Conditional = true
		case k:
		default:
			return stats, errors.Errorf("can't understand processor identifier '%s' in %+v", k, p)
		}
		stats.Type = k
	}
	return stats, nil
}

type pipelineStatsRecord struct {
	StatsRecord
	Processors []wrappedProcessor
}

type pipelineStatsRecordMap map[string]pipelineStatsRecord

func (r pipelineStatsRecord) extract() (stats *PipelineStats, err error) {
	stats = &PipelineStats{
		StatsRecord: r.StatsRecord,
		Processors:  make([]ProcessorStats, len(r.Processors)),
	}
	for idx, wrapped := range r.Processors {
		if stats.Processors[idx], err = wrapped.extract(); err != nil {
			return stats, errors.Wrapf(err, "converting processor %d", idx)
		}
	}
	return stats, nil
}

type pipelinesStatsNode struct {
	Ingest struct {
		Pipelines pipelineStatsRecordMap
	}
}

type pipelinesStatsResponse struct {
	Nodes map[string]pipelinesStatsNode
}

func GetPipelineStats(esClient *elasticsearch.API, pipelines []pipeline.Resource) (stats PipelineStatsMap, err error) {
	statsReq := esClient.Nodes.Stats.WithFilterPath("nodes.*.ingest.pipelines")
	resp, err := esClient.Nodes.Stats(statsReq)
	if err != nil {
		return nil, errors.Wrapf(err, "Node Stats API call failed")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read Stats API response body")
	}

	if resp.StatusCode != 200 {
		return nil, errors.Wrapf(elasticsearch.NewError(body), "unexpected response status for Node Stats (%d): %s", resp.StatusCode, resp.Status())
	}

	var statsResponse pipelinesStatsResponse
	if err = json.Unmarshal(body, &statsResponse); err != nil {
		return nil, errors.Wrap(err, "error decoding Node Stats response")
	}

	if nodeCount := len(statsResponse.Nodes); nodeCount != 1 {
		return nil, errors.Errorf("more than 1 ES node in stats response (%d)", nodeCount)
	}
	var nodePipelines map[string]pipelineStatsRecord
	for _, node := range statsResponse.Nodes {
		nodePipelines = node.Ingest.Pipelines
	}
	stats = make(PipelineStatsMap, len(pipelines))
	for _, requested := range pipelines {
		for pName, pStats := range nodePipelines {
			if requested.Name == pName {
				if stats[pName], err = pStats.extract(); err != nil {
					return stats, errors.Wrapf(err, "converting pipeline %s", pName)
				}
			}
		}
	}
	if len(stats) != len(pipelines) {
		var missing []string
		for _, requested := range pipelines {
			if _, found := stats[requested.Name]; !found {
				missing = append(missing, requested.Name)
			}
		}
		return stats, errors.Wrapf(err, "Node Stats response is missing some expected pipelines: %v", missing)
	}

	return stats, nil
}
