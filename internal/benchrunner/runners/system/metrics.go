// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package system

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/elastic/elastic-package/internal/elasticsearch"
	"github.com/elastic/elastic-package/internal/elasticsearch/ingest"
	"github.com/elastic/elastic-package/internal/environment"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/servicedeployer"
	"github.com/elastic/elastic-package/internal/signal"
)

var (
	ESMetricstoreHostEnv          = environment.WithElasticPackagePrefix("ESMETRICSTORE_HOST")
	ESMetricstoreUsernameEnv      = environment.WithElasticPackagePrefix("ESMETRICSTORE_USERNAME")
	ESMetricstorePasswordEnv      = environment.WithElasticPackagePrefix("ESMETRICSTORE_PASSWORD")
	ESMetricstoreCACertificateEnv = environment.WithElasticPackagePrefix("ESMETRICSTORE_CA_CERT")
)

type collector struct {
	ctxt     servicedeployer.ServiceContext
	metadata benchMeta
	scenario scenario

	interval       time.Duration
	esapi          *elasticsearch.API
	msapi          *elasticsearch.API
	datastream     string
	pipelinePrefix string

	stopC chan struct{}
	tick  *time.Ticker

	startIngestMetrics map[string]ingest.PipelineStatsMap
	endIngestMetrics   map[string]ingest.PipelineStatsMap
	startMetrics       metrics
	endMetrics         metrics
	diskUsage          map[string]ingest.DiskUsage
	startTotalHits     int
	endTotalHits       int
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
	ctxt servicedeployer.ServiceContext,
	benchName string,
	scenario scenario,
	esapi, msapi *elasticsearch.API,
	interval time.Duration,
	datastream, pipelinePrefix string,
) *collector {
	meta := benchMeta{Parameters: scenario}
	meta.Info.Benchmark = benchName
	meta.Info.RunID = ctxt.Test.RunID
	return &collector{
		ctxt:           ctxt,
		interval:       interval,
		scenario:       scenario,
		metadata:       meta,
		esapi:          esapi,
		msapi:          msapi,
		datastream:     datastream,
		pipelinePrefix: pipelinePrefix,
		stopC:          make(chan struct{}),
	}
}

func (c *collector) start() {
	c.tick = time.NewTicker(c.interval)
	c.createMetricsIndex()
	var once sync.Once

	go func() {
		once.Do(c.waitUntilReady)

		defer c.tick.Stop()

		c.startIngestMetrics = c.collectIngestMetrics()
		c.startTotalHits = c.collectTotalHits()
		c.startMetrics = c.collect()
		c.publish(c.createEventsFromMetrics(c.startMetrics))

		for {
			if signal.SIGINT() {
				logger.Debug("SIGINT: cancel metrics collection")
				c.collectMetricsPreviousToStop()
				c.publish(c.createEventsFromMetrics(c.endMetrics))
				return
			}

			select {
			case <-c.stopC:
				// last collect before stopping
				c.collectMetricsPreviousToStop()
				c.publish(c.createEventsFromMetrics(c.endMetrics))
				c.stopC <- struct{}{}
				return
			case <-c.tick.C:
				m := c.collect()
				c.publish(c.createEventsFromMetrics(m))
			}
		}
	}()
}

func (c *collector) stop() {
	c.stopC <- struct{}{}
	<-c.stopC
	close(c.stopC)
}

func (c *collector) collect() metrics {
	m := metrics{
		ts: time.Now().Unix(),
	}

	nstats, err := ingest.GetNodesStats(c.esapi)
	if err != nil {
		logger.Debug(err)
	} else {
		m.nMetrics = nstats
	}

	dsstats, err := ingest.GetDataStreamStats(c.esapi, c.datastream)
	if err != nil {
		logger.Debug(err)
	} else {
		m.dsMetrics = dsstats
	}

	return m
}

func (c *collector) publish(events [][]byte) {
	if c.msapi == nil {
		return
	}
	for _, e := range events {
		reqBody := bytes.NewReader(e)
		resp, err := c.msapi.Index(c.indexName(), reqBody)
		if err != nil {
			logger.Debugf("error indexing metrics: %e", err)
			continue
		}
		var sr map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
			logger.Debugf("error decoding search response: %e", err)
		}

		resErr, found := sr["error"]
		if found {
			errStr := resErr.(map[string]interface{})["reason"].(string)
			logger.Debugf("error searching for documents: %s", errStr)
		}
	}
}

func (c *collector) createMetricsIndex() {
	if c.msapi == nil {
		return
	}
	reader := bytes.NewReader(
		[]byte(`{
			"settings": {
				"number_of_replicas": 0
			},
			"mappings": {
				"dynamic_templates": [
					{
						"strings_as_keyword": {
							"match_mapping_type": "string",
							"mapping": {
								"ignore_above": 1024,
								"type": "keyword"
							}
						}
					}
				],
				"date_detection": false,
				"properties": {
					"@timestamp": {
						"type": "date"
					}
				}
			}
		}`),
	)

	logger.Debugf("creating %s index in metricstore...", c.indexName())

	createRes, err := c.msapi.Indices.Create(
		c.indexName(),
		c.msapi.Indices.Create.WithBody(reader),
	)
	if err != nil {
		logger.Debugf("could not create index: %v", err)
		return
	}
	createRes.Body.Close()

	if createRes.IsError() {
		logger.Debug("got a response error while creating index")
	}
}

func (c *collector) indexName() string {
	return fmt.Sprintf("bench-metrics-%s-%s", c.datastream, c.ctxt.Test.RunID)
}

func (c *collector) summarize() (*metricsSummary, error) {
	sum := metricsSummary{
		RunID:               c.ctxt.Test.RunID,
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
			logger.Debugf("node %s not found in initial metrics", node)
			continue
		}
		sumStats := make(ingest.PipelineStatsMap)
		for pname, endStats := range endPStats {
			startStats, found := startPStats[pname]
			if !found {
				logger.Debugf("pipeline %s not found in node %s initial metrics", pname, node)
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
	logger.Debug("waiting for datastream to be created...")

	waitTick := time.NewTicker(time.Second)
	defer waitTick.Stop()

readyLoop:
	for {
		if signal.SIGINT() {
			logger.Debug("SIGINT: cancel metrics collection")
			return
		}

		<-waitTick.C
		dsstats, err := ingest.GetDataStreamStats(c.esapi, c.datastream)
		if err != nil {
			logger.Debug(err)
		}
		if dsstats != nil {
			break readyLoop
		}
	}

	if c.scenario.WarmupTimePeriod > 0 {
		logger.Debugf("waiting %s for warmup period", c.scenario.WarmupTimePeriod)
		<-time.After(c.scenario.WarmupTimePeriod)
	}
	logger.Debug("metric collection starting...")
}

func (c *collector) collectIngestMetrics() map[string]ingest.PipelineStatsMap {
	ipMetrics, err := ingest.GetPipelineStatsByPrefix(c.esapi, c.pipelinePrefix)
	if err != nil {
		logger.Debugf("could not get ingest pipeline metrics: %w", err)
		return nil
	}
	return ipMetrics
}

func (c *collector) collectDiskUsage() map[string]ingest.DiskUsage {
	du, err := ingest.GetDiskUsage(c.esapi, c.datastream)
	if err != nil {
		logger.Debugf("could not get disk usage metrics: %w", err)
		return nil
	}
	return du
}

func (c *collector) collectMetricsPreviousToStop() {
	c.endIngestMetrics = c.collectIngestMetrics()
	c.diskUsage = c.collectDiskUsage()
	c.endTotalHits = c.collectTotalHits()
	c.endMetrics = c.collect()
}

func (c *collector) collectTotalHits() int {
	totalHits, err := getTotalHits(c.esapi, c.datastream)
	if err != nil {
		logger.Debugf("could not total hits: %w", err)
	}
	return totalHits
}

func (c *collector) createEventsFromMetrics(m metrics) [][]byte {
	dsEvent := struct {
		Ts int64 `json:"@timestamp"`
		*ingest.DataStreamStats
		Meta benchMeta `json:"benchmark_metadata"`
	}{
		Ts:              m.ts * 1000, // ms to s
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
			logger.Debugf("error marhsaling metrics event: %w", err)
			continue
		}
		events = append(events, b)
	}
	return events
}
