// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package system

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
	tick    *time.Ticker

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
	c.tick = time.NewTicker(c.interval)
	c.createMetricsIndex()
	var once sync.Once

	c.wg.Add(1)
	go func() {
		defer c.tick.Stop()
		defer c.wg.Done()
		for {
			select {
			case <-c.stopC:
				// last collect before stopping
				c.collectMetricsPreviousToStop(ctx)
				c.publish(c.createEventsFromMetrics(c.endMetrics))
				return
			case <-c.tick.C:
				once.Do(func() {
					c.waitUntilReady()
					c.startIngestMetrics = c.collectIngestMetrics()
					c.startTotalHits = c.collectTotalHits(ctx)
					c.startMetrics = c.collect()
					c.publish(c.createEventsFromMetrics(c.startMetrics))
				})
				m := c.collect()
				c.publish(c.createEventsFromMetrics(m))
			}
		}
	}()
}

func (c *collector) stop() {
	if !c.stopped.CompareAndSwap(false, true) {
		return
	}
	close(c.stopC)
	c.wg.Wait()
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
	for _, e := range events {
		reqBody := bytes.NewReader(e)
		resp, err := c.metricsAPI.Index(c.indexName(), reqBody)
		if err != nil {
			c.logger.Debug("error indexing event", slog.Any("error", err))
			continue
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			c.logger.Error("failed to read index response body", slog.Any("error", err))
		}
		resp.Body.Close()

		if resp.StatusCode != 201 {
			c.logger.Error("error indexing event",
				slog.Int("status.code", resp.StatusCode),
				slog.String("status", resp.Status()),
				slog.Any("error", elasticsearch.NewError(body)),
			)
		}
	}
}

//go:embed metrics_index.json
var metricsIndexBytes []byte

func (c *collector) createMetricsIndex() {
	if c.metricsAPI == nil {
		return
	}

	reader := bytes.NewReader(metricsIndexBytes)

	c.logger.Debug("creating %s index in metricstore...", slog.String("index", c.indexName()))

	createRes, err := c.metricsAPI.Indices.Create(
		c.indexName(),
		c.metricsAPI.Indices.Create.WithBody(reader),
	)
	if err != nil {
		c.logger.Debug("could not create index", slog.Any("error", err))
		return
	}
	createRes.Body.Close()

	if createRes.IsError() {
		c.logger.Debug("got a response error while creating index", slog.Any("response", createRes))
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

	sum.ClusterName = c.startMetrics.nMetrics.ClusterName
	sum.CollectionStartTs = c.startMetrics.ts
	sum.CollectionEndTs = c.endMetrics.ts
	sum.DataStreamStats = c.endMetrics.dsMetrics
	sum.Nodes = len(c.endMetrics.nMetrics.Nodes)

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

func (c *collector) waitUntilReady() {
	c.logger.Debug("waiting for datastream to be created...")

	waitTick := time.NewTicker(time.Second)
	defer waitTick.Stop()

readyLoop:
	for {
		select {
		case <-c.stopC:
			return
		case <-waitTick.C:
		}
		dsstats, err := ingest.GetDataStreamStats(c.esAPI, c.datastream)
		if err != nil {
			c.logger.Debug(err.Error())
		}
		if dsstats != nil {
			break readyLoop
		}
	}

	if c.scenario.WarmupTimePeriod > 0 {
		c.logger.Debug("waiting for warmup period", slog.Duration("warmup.period", c.scenario.WarmupTimePeriod))
		select {
		case <-c.stopC:
			return
		case <-time.After(c.scenario.WarmupTimePeriod):
		}
	}
	c.logger.Debug("metric collection starting...")
}

func (c *collector) collectIngestMetrics() map[string]ingest.PipelineStatsMap {
	ipMetrics, err := ingest.GetPipelineStatsByPrefix(c.esAPI, c.pipelinePrefix)
	if err != nil {
		c.logger.Debug("could not get ingest pipeline metrics", slog.Any("error", err))
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

func (c *collector) collectMetricsPreviousToStop(ctx context.Context) {
	c.endIngestMetrics = c.collectIngestMetrics()
	c.diskUsage = c.collectDiskUsage()
	c.endTotalHits = c.collectTotalHits(ctx)
	c.endMetrics = c.collect()
}

func (c *collector) collectTotalHits(ctx context.Context) int {
	totalHits, err := common.CountDocsInDataStream(ctx, c.esAPI, c.datastream)
	if err != nil {
		c.logger.Debug("could not total hits", slog.Any("error", err))
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

	for node, stats := range m.nMetrics.Nodes {
		nEvents = append(nEvents, nEvent{
			Ts:          m.ts * 1000, // ms to s
			ClusterName: m.nMetrics.ClusterName,
			NodeName:    node,
			NodeStats:   &stats,
			Meta:        c.metadata,
		})
	}

	var events [][]byte
	for _, e := range append(nEvents, dsEvent) {
		b, err := json.Marshal(e)
		if err != nil {
			c.logger.Debug("error marhsaling metrics event", slog.Any("error", err))
			continue
		}
		events = append(events, b)
	}
	return events
}
