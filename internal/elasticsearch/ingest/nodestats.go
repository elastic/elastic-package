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
			return stats, fmt.Errorf("extracting processor %d: %w", idx, err)
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
	statsResponse, err := requestPipelineStats(esClient)
	if err != nil {
		return nil, err
	}
	return getPipelineStats(statsResponse, pipelines)
}

func GetPipelineStatsByPrefix(esClient *elasticsearch.API, pipelinePrefix string) (map[string]PipelineStatsMap, error) {
	statsResponse, err := requestPipelineStats(esClient)
	if err != nil {
		return nil, err
	}
	return getPipelineStatsByPrefix(statsResponse, pipelinePrefix)
}

func requestPipelineStats(esClient *elasticsearch.API) ([]byte, error) {
	statsReq := esClient.Nodes.Stats.WithFilterPath("nodes.*.ingest.pipelines")
	resp, err := esClient.Nodes.Stats(statsReq)
	if err != nil {
		return nil, fmt.Errorf("node stats API call failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read Stats API response body: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("unexpected response status for Node Stats (%d): %s: %w", resp.StatusCode, resp.Status(), elasticsearch.NewError(body))
	}

	return body, nil
}

func getPipelineStatsByPrefix(body []byte, pipelinePrefix string) (stats map[string]PipelineStatsMap, err error) {
	var statsResponse pipelinesStatsResponse
	if err = json.Unmarshal(body, &statsResponse); err != nil {
		return nil, fmt.Errorf("error decoding Node Stats response: %w", err)
	}

	stats = make(map[string]PipelineStatsMap, len(statsResponse.Nodes))

	for nid, node := range statsResponse.Nodes {
		nodePStats := make(PipelineStatsMap)
		for name, pStats := range node.Ingest.Pipelines {
			if !strings.HasPrefix(name, pipelinePrefix) {
				continue
			}
			if nodePStats[name], err = pStats.extract(); err != nil {
				return stats, fmt.Errorf("converting pipeline %s: %w", name, err)
			}
		}
		stats[nid] = nodePStats
	}

	return stats, nil
}

func getPipelineStats(body []byte, pipelines []Pipeline) (stats PipelineStatsMap, err error) {
	var statsResponse pipelinesStatsResponse
	if err = json.Unmarshal(body, &statsResponse); err != nil {
		return nil, fmt.Errorf("error decoding Node Stats response: %w", err)
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
				return stats, fmt.Errorf("converting pipeline %s: %w", requested.Name, err)
			}
		} else {
			missing = append(missing, requested.Name)
		}
	}
	if len(missing) != 0 {
		return stats, fmt.Errorf("node stats response is missing expected pipelines: %s", strings.Join(missing, ", "))
	}

	return stats, nil
}

type NodesStats struct {
	ClusterName string               `json:"cluster_name"`
	Nodes       map[string]NodeStats `json:"nodes"`
}

type NodeStats struct {
	Breakers map[string]struct {
		LimitSizeInBytes     int     `json:"limit_size_in_bytes"`
		LimitSize            string  `json:"limit_size"`
		EstimatedSizeInBytes int     `json:"estimated_size_in_bytes"`
		EstimatedSize        string  `json:"estimated_size"`
		Overhead             float64 `json:"overhead"`
		Tripped              int     `json:"tripped"`
	}
	Indices struct {
		Docs struct {
			Count   int `json:"count"`
			Deleted int `json:"deleted"`
		} `json:"docs"`
		ShardStats struct {
			TotalCount int `json:"total_count"`
		} `json:"shard_stats"`
		Store struct {
			SizeInBytes             int `json:"size_in_bytes"`
			TotalDataSetSizeInBytes int `json:"total_data_set_size_in_bytes"`
			ReservedInBytes         int `json:"reserved_in_bytes"`
		} `json:"store"`
		Indexing struct {
			IndexTotal           int     `json:"index_total"`
			IndexTimeInMillis    int     `json:"index_time_in_millis"`
			IndexCurrent         int     `json:"index_current"`
			IndexFailed          int     `json:"index_failed"`
			DeleteTotal          int     `json:"delete_total"`
			DeleteTimeInMillis   int     `json:"delete_time_in_millis"`
			DeleteCurrent        int     `json:"delete_current"`
			NoopUpdateTotal      int     `json:"noop_update_total"`
			IsThrottled          bool    `json:"is_throttled"`
			ThrottleTimeInMillis int     `json:"throttle_time_in_millis"`
			WriteLoad            float64 `json:"write_load"`
		} `json:"indexing"`
		Get struct {
			Total               int `json:"total"`
			TimeInMillis        int `json:"time_in_millis"`
			ExistsTotal         int `json:"exists_total"`
			ExistsTimeInMillis  int `json:"exists_time_in_millis"`
			MissingTotal        int `json:"missing_total"`
			MissingTimeInMillis int `json:"missing_time_in_millis"`
			Current             int `json:"current"`
		} `json:"get"`
		Search struct {
			OpenContexts        int `json:"open_contexts"`
			QueryTotal          int `json:"query_total"`
			QueryTimeInMillis   int `json:"query_time_in_millis"`
			QueryCurrent        int `json:"query_current"`
			FetchTotal          int `json:"fetch_total"`
			FetchTimeInMillis   int `json:"fetch_time_in_millis"`
			FetchCurrent        int `json:"fetch_current"`
			ScrollTotal         int `json:"scroll_total"`
			ScrollTimeInMillis  int `json:"scroll_time_in_millis"`
			ScrollCurrent       int `json:"scroll_current"`
			SuggestTotal        int `json:"suggest_total"`
			SuggestTimeInMillis int `json:"suggest_time_in_millis"`
			SuggestCurrent      int `json:"suggest_current"`
		} `json:"search"`
		Merges struct {
			Current                    int   `json:"current"`
			CurrentDocs                int   `json:"current_docs"`
			CurrentSizeInBytes         int   `json:"current_size_in_bytes"`
			Total                      int   `json:"total"`
			TotalTimeInMillis          int   `json:"total_time_in_millis"`
			TotalDocs                  int   `json:"total_docs"`
			TotalSizeInBytes           int   `json:"total_size_in_bytes"`
			TotalStoppedTimeInMillis   int   `json:"total_stopped_time_in_millis"`
			TotalThrottledTimeInMillis int   `json:"total_throttled_time_in_millis"`
			TotalAutoThrottleInBytes   int64 `json:"total_auto_throttle_in_bytes"`
		} `json:"merges"`
		Refresh struct {
			Total                     int `json:"total"`
			TotalTimeInMillis         int `json:"total_time_in_millis"`
			ExternalTotal             int `json:"external_total"`
			ExternalTotalTimeInMillis int `json:"external_total_time_in_millis"`
			Listeners                 int `json:"listeners"`
		} `json:"refresh"`
		Flush struct {
			Total             int `json:"total"`
			Periodic          int `json:"periodic"`
			TotalTimeInMillis int `json:"total_time_in_millis"`
		} `json:"flush"`
		Warmer struct {
			Current           int `json:"current"`
			Total             int `json:"total"`
			TotalTimeInMillis int `json:"total_time_in_millis"`
		} `json:"warmer"`
		QueryCache struct {
			MemorySizeInBytes int `json:"memory_size_in_bytes"`
			TotalCount        int `json:"total_count"`
			HitCount          int `json:"hit_count"`
			MissCount         int `json:"miss_count"`
			CacheSize         int `json:"cache_size"`
			CacheCount        int `json:"cache_count"`
			Evictions         int `json:"evictions"`
		} `json:"query_cache"`
		Fielddata struct {
			MemorySizeInBytes int `json:"memory_size_in_bytes"`
			Evictions         int `json:"evictions"`
		} `json:"fielddata"`
		Completion struct {
			SizeInBytes int `json:"size_in_bytes"`
		} `json:"completion"`
		Segments struct {
			Count                     int            `json:"count"`
			MemoryInBytes             int            `json:"memory_in_bytes"`
			TermsMemoryInBytes        int            `json:"terms_memory_in_bytes"`
			StoredFieldsMemoryInBytes int            `json:"stored_fields_memory_in_bytes"`
			TermVectorsMemoryInBytes  int            `json:"term_vectors_memory_in_bytes"`
			NormsMemoryInBytes        int            `json:"norms_memory_in_bytes"`
			PointsMemoryInBytes       int            `json:"points_memory_in_bytes"`
			DocValuesMemoryInBytes    int            `json:"doc_values_memory_in_bytes"`
			IndexWriterMemoryInBytes  int            `json:"index_writer_memory_in_bytes"`
			VersionMapMemoryInBytes   int            `json:"version_map_memory_in_bytes"`
			FixedBitSetMemoryInBytes  int            `json:"fixed_bit_set_memory_in_bytes"`
			MaxUnsafeAutoIDTimestamp  int            `json:"max_unsafe_auto_id_timestamp"`
			FileSizes                 map[string]int `json:"file_sizes"`
		} `json:"segments"`
		Translog struct {
			Operations              int `json:"operations"`
			SizeInBytes             int `json:"size_in_bytes"`
			UncommittedOperations   int `json:"uncommitted_operations"`
			UncommittedSizeInBytes  int `json:"uncommitted_size_in_bytes"`
			EarliestLastModifiedAge int `json:"earliest_last_modified_age"`
		} `json:"translog"`
		RequestCache struct {
			MemorySizeInBytes int `json:"memory_size_in_bytes"`
			Evictions         int `json:"evictions"`
			HitCount          int `json:"hit_count"`
			MissCount         int `json:"miss_count"`
		} `json:"request_cache"`
		Recovery struct {
			CurrentAsSource      int `json:"current_as_source"`
			CurrentAsTarget      int `json:"current_as_target"`
			ThrottleTimeInMillis int `json:"throttle_time_in_millis"`
		} `json:"recovery"`
		Bulk struct {
			TotalOperations   int64 `json:"total_operations"`
			TotalTimeInMillis int64 `json:"total_time_in_millis"`
			TotalSizeInBytes  int64 `json:"total_size_in_bytes"`
			AvgTimeInMillis   int64 `json:"avg_time_in_millis"`
			AvgSizeInBytes    int64 `json:"avg_size_in_bytes"`
		} `json:"bulk"`
		Mappings struct {
			TotalCount                    int64 `json:"total_count"`
			TotalEstimatedOverheadInBytes int64 `json:"total_estimated_overhead_in_bytes"`
		} `json:"mappings"`
	} `json:"indices"`
	JVM struct {
		Mem struct {
			HeapUsedInBytes         int `json:"heap_used_in_bytes"`
			HeapUsedPercent         int `json:"heap_used_percent"`
			HeapCommittedInBytes    int `json:"heap_committed_in_bytes"`
			HeapMaxInBytes          int `json:"heap_max_in_bytes"`
			NonHeapUsedInBytes      int `json:"non_heap_used_in_bytes"`
			NonHeapCommittedInBytes int `json:"non_heap_committed_in_bytes"`
			Pools                   map[string]struct {
				UsedInBytes     int `json:"used_in_bytes"`
				MaxInBytes      int `json:"max_in_bytes"`
				PeakUsedInBytes int `json:"peak_used_in_bytes"`
				PeakMaxInBytes  int `json:"peak_max_in_bytes"`
			} `json:"pools"`
		} `json:"mem"`
		Gc struct {
			Collectors map[string]struct {
				CollectionCount        int `json:"collection_count"`
				CollectionTimeInMillis int `json:"collection_time_in_millis"`
			} `json:"collectors"`
		} `json:"gc"`
		BufferPools map[string]struct {
			Count                int `json:"count"`
			UsedInBytes          int `json:"used_in_bytes"`
			TotalCapacityInBytes int `json:"total_capacity_in_bytes"`
		} `json:"buffer_pools"`
	} `json:"jvm"`
	OS struct {
		Mem struct {
			TotalInBytes         int64 `json:"total_in_bytes"`
			AdjustedTotalInBytes int64 `json:"adjusted_total_in_bytes"`
			FreeInBytes          int64 `json:"free_in_bytes"`
			UsedInBytes          int64 `json:"used_in_bytes"`
			FreePercent          int   `json:"free_percent"`
			UsedPercent          int   `json:"used_percent"`
		} `json:"mem"`
	} `json:"os"`
	Process struct {
		CPU struct {
			Percent       int   `json:"percent"`
			TotalInMillis int64 `json:"total_in_millis"`
		} `json:"cpu"`
	} `json:"process"`
	ThreadPool map[string]struct {
		Threads   int `json:"threads"`
		Queue     int `json:"queue"`
		Active    int `json:"active"`
		Rejected  int `json:"rejected"`
		Largest   int `json:"largest"`
		Completed int `json:"completed"`
	} `json:"thread_pool"`
	Transport struct {
		ServerOpen                   int `json:"server_open"`
		TotalOutboundConnections     int `json:"total_outbound_connections"`
		RxCount                      int `json:"rx_count"`
		RxSizeInBytes                int `json:"rx_size_in_bytes"`
		TxCount                      int `json:"tx_count"`
		TxSizeInBytes                int `json:"tx_size_in_bytes"`
		InboundHandlingTimeHistogram []struct {
			GEMillis int `json:"ge_millis"`
			LTMillis int `json:"lt_millis"`
			Count    int `json:"count"`
		} `json:"inbound_handling_time_histogram"`
		OutboundHandlingTimeHistogram []struct {
			GEMillis int `json:"ge_millis"`
			LTMillis int `json:"lt_millis"`
			Count    int `json:"count"`
		} `json:"outbound_handling_time_histogram"`
	} `json:"transport"`
}

func GetNodesStats(esClient *elasticsearch.API) (*NodesStats, error) {
	req := esClient.Nodes.Stats.WithFilterPath("cluster_name," +
		"nodes.*.breakers," +
		"nodes.*.indices," +
		"nodes.*.jvm.mem," +
		"nodes.*.jvm.gc," +
		"nodes.*.jvm.buffer_pools," +
		"nodes.*.os.mem," +
		"nodes.*.process.cpu," +
		"nodes.*.thread_pool," +
		"nodes.*.transport",
	)
	resp, err := esClient.Nodes.Stats(req)
	if err != nil {
		return nil, fmt.Errorf("node stats API call failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read Stats API response body: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("unexpected response status for Node Stats (%d): %s: %w", resp.StatusCode, resp.Status(), elasticsearch.NewError(body))
	}

	var statsResponse NodesStats
	if err = json.Unmarshal(body, &statsResponse); err != nil {
		return nil, fmt.Errorf("error decoding Node Stats response: %w", err)
	}
	return &statsResponse, nil
}
