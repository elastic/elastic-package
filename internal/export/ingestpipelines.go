// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package export

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"

	"github.com/elastic/elastic-package/internal/elasticsearch"
)

type IngestPipeline struct {
	id  string
	raw []byte
}

func (p IngestPipeline) Name() string {
	return p.id
}

func (p IngestPipeline) JSON() []byte {
	return p.raw
}

func getIngestPipelines(ctx context.Context, api *elasticsearch.API, ids ...string) ([]IngestPipeline, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	var pipelines []IngestPipeline
	for _, id := range ids {
		resultPipelines, err := getIngestPipelineByID(ctx, api, id)
		if err != nil {
			return nil, err
		}
		pipelines = append(pipelines, resultPipelines...)
	}
	return pipelines, nil
}

type getIngestPipelineResponse map[string]json.RawMessage

func getIngestPipelineByID(ctx context.Context, api *elasticsearch.API, id string) ([]IngestPipeline, error) {
	resp, err := api.Ingest.GetPipeline(
		api.Ingest.GetPipeline.WithContext(ctx),
		api.Ingest.GetPipeline.WithPipelineID(id),
		api.Ingest.GetPipeline.WithPretty(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get ingest pipeline %s: %w", id, err)
	}
	defer resp.Body.Close()

	d, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var pipelinesResponse getIngestPipelineResponse
	err = json.Unmarshal(d, &pipelinesResponse)
	if err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	var pipelines []IngestPipeline
	for id, raw := range pipelinesResponse {
		pipelines = append(pipelines, IngestPipeline{id: id, raw: raw})
	}

	return pipelines, nil
}
