// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package ingest

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/elastic/elastic-package/internal/elasticsearch"
)

type DataStreamsStats struct {
	DataStreams []DataStreamStats `json:"data_streams"`
}

type DataStreamStats struct {
	DataStream       string `json:"data_stream"`
	BackingIndices   int    `json:"backing_indices"`
	StoreSizeBytes   int    `json:"store_size_bytes"`
	MaximumTimestamp int    `json:"maximum_timestamp"`
}

func GetDataStreamStats(esClient *elasticsearch.API, datastream string) (*DataStreamStats, error) {
	req := esClient.Indices.DataStreamsStats.WithName(datastream)
	resp, err := esClient.Indices.DataStreamsStats(req)
	if err != nil {
		return nil, fmt.Errorf("failed call to DataStream Stats API: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read Stats API response body: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("unexpected response status for DataStream Stats (%d): %s: %w", resp.StatusCode, resp.Status(), elasticsearch.NewError(body))
	}

	var statsResponse DataStreamsStats
	if err = json.Unmarshal(body, &statsResponse); err != nil {
		return nil, fmt.Errorf("error decoding DataStream Stats response: %w", err)
	}
	if len(statsResponse.DataStreams) > 0 {
		return &statsResponse.DataStreams[0], nil
	}

	return nil, errors.New("couldn't get DataStream stats")
}
