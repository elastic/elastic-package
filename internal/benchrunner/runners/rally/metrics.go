// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package rally

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/elastic/elastic-package/internal/benchrunner/runners/common"
	"github.com/elastic/elastic-package/internal/elasticsearch"
	"github.com/elastic/elastic-package/internal/elasticsearch/ingest"
	"github.com/elastic/elastic-package/internal/servicedeployer"
)

type collector struct {
	svcInfo  servicedeployer.ServiceInfo
	metadata benchMeta
	scenario scenario

	interval       time.Duration
	esAPI          *elasticsearch.API
	metricsAPI     *elasticsearch.API
	datastream     string
	pipelinePrefix string

	wg      sync.WaitGroup
	stopped atomic.Bool
	stopC   chan struct{}

	startIngestMetrics map[string]ingest.PipelineStatsMap
	endIngestMetrics   map[string]ingest.PipelineStatsMap
	startMetrics       metrics
	endMetrics         metrics
	diskUsage          map[string]ingest.DiskUsage
	startTotalHits     int
	endTotalHits       int

	logger *slog.Logger
}

type metrics struct {
	ts        int64
	dsMetrics *ingest.DataStreamStats
	nMetrics  *ingest.NodesStats
}

type metricsSummary struct {
	ClusterName         string
	Nodes               int
	RunID               string
	CollectionStartTs   int64
	CollectionEndTs     int64
	DataStreamStats     *ingest.DataStreamStats
	IngestPipelineStats map[string]ingest.PipelineStatsMap
	DiskUsage           map[string]ingest.DiskUsage
	TotalHits           int
	NodesStats          map[string]ingest.NodeStats
}

func newCollector(
	svcInfo servicedeployer.ServiceInfo,
	benchName string,
	scenario scenario,
	esAPI, metricsAPI *elasticsearch.API,
	interval time.Duration,
	datastream, pipelinePrefix string,
	logger *slog.Logger,
) *collector {
	meta := benchMeta{Parameters: scenario}
	meta.Info.Benchmark = benchName
	meta.Info.RunID = svcInfo.Test.RunID
	return &collector{
		svcInfo:        svcInfo,
		interval:       interval,
		scenario:       scenario,
		metadata:       meta,
		esAPI:          esAPI,
		metricsAPI:     metricsAPI,
		datastream:     datastream,
		pipelinePrefix: pipelinePrefix,
		stopC:          make(chan struct{}),
		logger:         logger,
	}
}

func (c *collector) start(ctx context.Context) {
	c.createMetricsIndex()

	c.collectMetricsBeforeRallyRun(ctx)

	c.wg.Add(1)
	go func() {
		defer c.wg.Done()

		<-c.stopC
		// last collect before stopping
		c.collectMetricsAfterRallyRun(ctx)
		c.publish(c.createEventsFromMetrics(c.endMetrics))
	}()
}

func (c *collector) stop() {
	if !c.stopped.CompareAndSwap(false, true) {
		return
	}
	close(c.stopC)
	c.wg.Wait()
}

func (c *collector) collectMetricsBeforeRallyRun(ctx context.Context) {
	resp, err := c.esAPI.Indices.Refresh(c.esAPI.Indices.Refresh.WithIndex(c.datastream))
	if err != nil {
		c.logger.Error("unable to refresh data stream at the beginning of rally run", slog.Any("error", err))
		return
	}
	defer resp.Body.Close()
	if resp.IsError() {
		c.logger.Error("unable to refresh data stream at the beginning of rally run", slog.String("error", resp.String()))
		return
	}

	c.startTotalHits = c.collectTotalHits(ctx)
	c.startMetrics = c.collect()
	c.startIngestMetrics = c.collectIngestMetrics()
	c.publish(c.createEventsFromMetrics(c.startMetrics))
}

func (c *collector) collect() metrics {
	m := metrics{
		ts: time.Now().Unix(),
	}

	nstats, err := ingest.GetNodesStats(c.esAPI)
	if err != nil {
		c.logger.Debug(err.Error())
	} else {
		m.nMetrics = nstats
	}

	dsstats, err := ingest.GetDataStreamStats(c.esAPI, c.datastream)
	if err != nil {
		c.logger.Debug(err.Error())
	} else {
		m.dsMetrics = dsstats
	}

	return m
}

func (c *collector) publish(events [][]byte) {
	if c.metricsAPI == nil {
		return
	}
	eventsForBulk := bytes.Join(events, []byte("\n"))
	reqBody := bytes.NewReader(eventsForBulk)
	resp, err := c.metricsAPI.Bulk(reqBody, c.metricsAPI.Bulk.WithIndex(c.indexName()))
	if err != nil {
		c.logger.Error("error indexing event in metricstore", slog.Any("error", err), slog.String("index", c.indexName()))
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.logger.Error("failed to read index response body from metricstore", slog.Any("error", err), slog.String("index", c.indexName()))
	}

	if resp.StatusCode != 201 {
		c.logger.Error("error indexing event in metricstore",
			slog.String("index", c.indexName()),
			slog.Int("status.code", resp.StatusCode),
			slog.String("status", resp.Status()),
			slog.Any("error", elasticsearch.NewError(body)),
		)
	}
}

//go:embed metrics_index.json
var metricsIndexBytes []byte

func (c *collector) createMetricsIndex() {
	if c.metricsAPI == nil {
		return
	}

	reader := bytes.NewReader(metricsIndexBytes)

	c.logger.Debug("creating index in metricstore...", slog.String("index", c.indexName()))

	resp, err := c.metricsAPI.Indices.Create(
		c.indexName(),
		c.metricsAPI.Indices.Create.WithBody(reader),
	)
	if err != nil {
		c.logger.Error("could not create index", slog.Any("error", err))
		return
	}
	defer resp.Body.Close()

	if resp.IsError() {
		c.logger.Error("got a response error while creating index", slog.String("error", resp.String()))
	}
}

func (c *collector) indexName() string {
	return fmt.Sprintf("bench-metrics-%s-%s", c.datastream, c.svcInfo.Test.RunID)
}

func (c *collector) summarize() (*metricsSummary, error) {
	sum := metricsSummary{
		RunID:               c.svcInfo.Test.RunID,
		IngestPipelineStats: make(map[string]ingest.PipelineStatsMap),
		NodesStats:          make(map[string]ingest.NodeStats),
		DiskUsage:           c.diskUsage,
		TotalHits:           c.endTotalHits - c.startTotalHits,
	}

	if c.startMetrics.nMetrics != nil {
		sum.ClusterName = c.startMetrics.nMetrics.ClusterName
	}
	sum.CollectionStartTs = c.startMetrics.ts
	sum.CollectionEndTs = c.endMetrics.ts
	if c.endMetrics.dsMetrics != nil {
		sum.DataStreamStats = c.endMetrics.dsMetrics
	}
	if c.endMetrics.nMetrics != nil {
		sum.Nodes = len(c.endMetrics.nMetrics.Nodes)
	}

	for node, endPStats := range c.endIngestMetrics {
		startPStats, found := c.startIngestMetrics[node]
		if !found {
			c.logger.Debug("node not found in initial metrics", slog.String("node", node))
			continue
		}
		sumStats := make(ingest.PipelineStatsMap)
		for pname, endStats := range endPStats {
			startStats, found := startPStats[pname]
			if !found {
				c.logger.Debug("pipeline not found in node initial metrics", slog.String("pipeline", pname), slog.String("node", node))
				continue
			}
			sumStats[pname] = ingest.PipelineStats{
				StatsRecord: ingest.StatsRecord{
					Count:        endStats.Count - startStats.Count,
					Failed:       endStats.Failed - startStats.Failed,
					TimeInMillis: endStats.TimeInMillis - startStats.TimeInMillis,
				},
				Processors: make([]ingest.ProcessorStats, len(endStats.Processors)),
			}
			for i, endPr := range endStats.Processors {
				startPr := startStats.Processors[i]
				sumStats[pname].Processors[i] = ingest.ProcessorStats{
					Type:        endPr.Type,
					Extra:       endPr.Extra,
					Conditional: endPr.Conditional,
					Stats: ingest.StatsRecord{
						Count:        endPr.Stats.Count - startPr.Stats.Count,
						Failed:       endPr.Stats.Failed - startPr.Stats.Failed,
						TimeInMillis: endPr.Stats.TimeInMillis - startPr.Stats.TimeInMillis,
					},
				}
			}
		}
		sum.IngestPipelineStats[node] = sumStats
	}

	return &sum, nil
}

func (c *collector) collectIngestMetrics() map[string]ingest.PipelineStatsMap {
	ipMetrics, err := ingest.GetPipelineStatsByPrefix(c.esAPI, c.pipelinePrefix)
	if err != nil {
		c.logger.Debug("could not get ingest pipeline metricsv", slog.Any("error", err))
		return nil
	}
	return ipMetrics
}

func (c *collector) collectDiskUsage() map[string]ingest.DiskUsage {
	du, err := ingest.GetDiskUsage(c.esAPI, c.datastream)
	if err != nil {
		c.logger.Debug("could not get disk usage metrics", slog.Any("error", err))
		return nil
	}
	return du
}

func (c *collector) collectMetricsAfterRallyRun(ctx context.Context) {
	resp, err := c.esAPI.Indices.Refresh(c.esAPI.Indices.Refresh.WithIndex(c.datastream))
	if err != nil {
		c.logger.Error("unable to refresh data stream at the end of rally run", slog.Any("error", err))
		return
	}
	defer resp.Body.Close()
	if resp.IsError() {
		c.logger.Error("unable to refresh data stream at the end of rally runs", slog.String("error", resp.String()))
		return
	}

	c.diskUsage = c.collectDiskUsage()
	c.endMetrics = c.collect()
	c.endIngestMetrics = c.collectIngestMetrics()
	c.endTotalHits = c.collectTotalHits(ctx)

	c.publish(c.createEventsFromMetrics(c.endMetrics))
}

func (c *collector) collectTotalHits(ctx context.Context) int {
	totalHits, err := common.CountDocsInDataStream(ctx, c.esAPI, c.datastream, c.logger)
	if err != nil {
		c.logger.Debug("could not get total hits", slog.Any("error", err))
	}
	return totalHits
}

func (c *collector) createEventsFromMetrics(m metrics) [][]byte {
	dsEvent := struct {
		Timestamp int64 `json:"@timestamp"`
		*ingest.DataStreamStats
		Meta benchMeta `json:"benchmark_metadata"`
	}{
		Timestamp:       m.ts * 1000, // ms to s
		DataStreamStats: m.dsMetrics,
		Meta:            c.metadata,
	}

	type nEvent struct {
		Ts          int64  `json:"@timestamp"`
		ClusterName string `json:"cluster_name"`
		NodeName    string `json:"node_name"`
		*ingest.NodeStats
		Meta benchMeta `json:"benchmark_metadata"`
	}

	var nEvents []interface{}

	if c.startMetrics.nMetrics != nil {
		for node, stats := range m.nMetrics.Nodes {
			nEvents = append(nEvents, nEvent{
				Ts:          m.ts * 1000, // ms to s
				ClusterName: m.nMetrics.ClusterName,
				NodeName:    node,
				NodeStats:   &stats,
				Meta:        c.metadata,
			})
		}
	}

	var events [][]byte
	for _, e := range append(nEvents, dsEvent) {
		b, err := json.Marshal(e)
		if err != nil {
			c.logger.Debug("error marshalling metrics event", slog.Any("error", err))
			continue
		}
		events = append(events, b)
	}
	return events
}
