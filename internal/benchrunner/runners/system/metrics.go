// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package system

import (
	"sync"
	"time"

	"github.com/elastic/elastic-package/internal/benchrunner/runners/system/servicedeployer"
	"github.com/elastic/elastic-package/internal/elasticsearch"
	"github.com/elastic/elastic-package/internal/elasticsearch/ingest"
	"github.com/elastic/elastic-package/internal/environment"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/signal"
)

var (
	ESMetricstoreHostEnv          = environment.WithElasticPackagePrefix("ESMETRICSTORE_HOST")
	ESMetricstoreUsernameEnv      = environment.WithElasticPackagePrefix("ESMETRICSTORE_USERNAME")
	ESMetricstorePasswordEnv      = environment.WithElasticPackagePrefix("ESMETRICSTORE_PASSWORD")
	ESMetricstoreCACertificateEnv = environment.WithElasticPackagePrefix("ESMETRICSTORE_CA_CERT")
)

type collector struct {
	ctxt           servicedeployer.ServiceContext
	warmupD        time.Duration
	interval       time.Duration
	esapi          *elasticsearch.API
	msapi          *elasticsearch.API
	datastream     string
	pipelinePrefix string

	stopC chan struct{}
	tick  *time.Ticker

	startIngestMetrics map[string]ingest.PipelineStatsMap
	endIngestMetrics   map[string]ingest.PipelineStatsMap
	collectedMetrics   []metrics
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
}

func newCollector(
	ctxt servicedeployer.ServiceContext,
	esapi, msapi *elasticsearch.API,
	datastream, pipelinePrefix string,
) *collector {
	return &collector{
		ctxt:           ctxt,
		interval:       interval,
		warmupD:        warmup,
		esapi:          esapi,
		msapi:          msapi,
		datastream:     datastream,
		pipelinePrefix: pipelinePrefix,
		stopC:          make(chan struct{}, 1),
	}
}

func (c *collector) start() {
	c.tick = time.NewTicker(c.interval)

	go func() {
		var once sync.Once
		once.Do(c.waitUntilReady)

		defer c.tick.Stop()

		c.startIngestMetrics = c.collectIngestMetrics()
		c.startTotalHits = c.collectTotalHits()

		for {
			if signal.SIGINT() {
				logger.Debug("SIGINT: cancel metrics collection")
				c.collectMetricsPreviousToStop()
				return
			}

			select {
			case <-c.stopC:
				// last collect before stopping
				c.collectMetricsPreviousToStop()
				c.stopC <- struct{}{}
				return
			case <-c.tick.C:
				c.collect()
			}
		}
	}()
}

func (c *collector) stop() {
	c.stopC <- struct{}{}
	<-c.stopC
	close(c.stopC)
}

func (c *collector) collect() {
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

	c.collectedMetrics = append(c.collectedMetrics, m)
}

func (c *collector) summarize() (*metricsSummary, error) {
	sum := metricsSummary{
		RunID:               c.ctxt.Bench.RunID,
		IngestPipelineStats: make(map[string]ingest.PipelineStatsMap),
		DiskUsage:           c.diskUsage,
		TotalHits:           c.endTotalHits - c.startTotalHits,
	}

	if len(c.collectedMetrics) > 0 {
		sum.CollectionStartTs = c.collectedMetrics[0].ts
		sum.CollectionEndTs = c.collectedMetrics[len(c.collectedMetrics)-1].ts
		sum.DataStreamStats = c.collectedMetrics[len(c.collectedMetrics)-1].dsMetrics
		sum.ClusterName = c.collectedMetrics[0].nMetrics.ClusterName
		sum.Nodes = len(c.collectedMetrics[len(c.collectedMetrics)-1].nMetrics.Nodes)
	}

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
					Count:  endStats.Count - startStats.Count,
					Failed: endStats.TimeInMillis - startStats.TimeInMillis,
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

	if c.warmupD > 0 {
		logger.Debugf("waiting %s for warmup period", c.warmupD)
		<-time.After(c.warmupD)
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
	c.collect()
	c.endIngestMetrics = c.collectIngestMetrics()
	c.diskUsage = c.collectDiskUsage()
	c.endTotalHits = c.collectTotalHits()
}

func (c *collector) collectTotalHits() int {
	totalHits, err := getTotalHits(c.esapi, c.datastream)
	if err != nil {
		logger.Debugf("could not total hits: %w", err)
	}
	return totalHits
}
