// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package export

import (
	"context"
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
		pipeline, err := getIngestPipelineByID(ctx, api, id)
		if err != nil {
			return nil, err
		}
		pipelines = append(pipelines, pipeline)
	}
	return pipelines, nil
}

func getIngestPipelineByID(ctx context.Context, api *elasticsearch.API, id string) (IngestPipeline, error) {
	resp, err := api.Ingest.GetPipeline(
		api.Ingest.GetPipeline.WithContext(ctx),
		api.Ingest.GetPipeline.WithPipelineID(id),
		api.Ingest.GetPipeline.WithPretty(),
	)
	if err != nil {
		return IngestPipeline{}, fmt.Errorf("failed to get ingest pipeline %s: %w", id, err)
	}
	defer resp.Body.Close()

	// TODO: Handle the case of a response with multiple pipelines (no pipeline, or with wildcard).
	// TODO: Get the actual pipeline from the object in the response body.
	d, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return IngestPipeline{}, fmt.Errorf("failed to read response body: %w", err)
	}

	return IngestPipeline{id: id, raw: d}, nil
}
