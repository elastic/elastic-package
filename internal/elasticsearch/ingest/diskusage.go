// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package ingest

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/elastic/elastic-package/internal/elasticsearch"
)

type DiskUsageStat struct {
	Total         string `json:"total"`
	TotalInBytes  int    `json:"total_in_bytes"`
	InvertedIndex struct {
		Total        string `json:"total"`
		TotalInBytes int    `json:"total_in_bytes"`
	} `json:"inverted_index"`
	StoredFields        string `json:"stored_fields"`
	StoredFieldsInBytes int    `json:"stored_fields_in_bytes"`
	DocValues           string `json:"doc_values"`
	DocValuesInBytes    int    `json:"doc_values_in_bytes"`
	Points              string `json:"points"`
	PointsInBytes       int    `json:"points_in_bytes"`
	Norms               string `json:"norms"`
	NormsInBytes        int    `json:"norms_in_bytes"`
	TermVectors         string `json:"term_vectors"`
	TermVectorsInBytes  int    `json:"term_vectors_in_bytes"`
	KnnVectors          string `json:"knn_vectors"`
	KnnVectorsInBytes   int    `json:"knn_vectors_in_bytes"`
}

type DiskUsage struct {
	StoreSize        string                   `json:"store_size"`
	StoreSizeInBytes int                      `json:"store_size_in_bytes"`
	AllFields        DiskUsageStat            `json:"all_fields"`
	Fields           map[string]DiskUsageStat `json:"fields"`
}

func GetDiskUsage(esClient *elasticsearch.API, datastream string) (map[string]DiskUsage, error) {
	resp, err := esClient.Indices.DiskUsage(datastream,
		esClient.Indices.DiskUsage.WithFlush(true),
		esClient.Indices.DiskUsage.WithRunExpensiveTasks(true),
	)
	if err != nil {
		return nil, fmt.Errorf("DiskUsage Stats API call failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read Stats API response body: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("unexpected response status for DiskUsage Stats (%d): %s: %w", resp.StatusCode, resp.Status(), elasticsearch.NewError(body))
	}

	var stats map[string]DiskUsage
	if err = json.Unmarshal(body, &stats); err != nil {
		return nil, fmt.Errorf("error decoding stats response: %w", err)
	}
	delete(stats, "_shards")
	return stats, nil
}
