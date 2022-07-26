// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package dump

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"

	"github.com/elastic/elastic-package/internal/common"
	"github.com/elastic/elastic-package/internal/elasticsearch"
)

// IngestPipeline contains the information needed to export an ingest pipeline.
type IngestPipeline struct {
	Processors []struct {
		Pipeline *struct {
			Name string `json:"name"`
		} `json:"pipeline,omitempty"`
	} `json:"processors"`

	id  string
	raw []byte
}

// Name returns the name of the ingest pipeline.
func (p IngestPipeline) Name() string {
	return p.id
}

// JSON returns the JSON representation of the ingest pipeline.
func (p IngestPipeline) JSON() []byte {
	return p.raw
}

func getIngestPipelines(ctx context.Context, api *elasticsearch.API, ids ...string) ([]IngestPipeline, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	var pipelines []IngestPipeline
	var collected []string
	pending := ids
	for len(pending) > 0 {
		for _, id := range pending {
			resultPipelines, err := getIngestPipelineByID(ctx, api, id)
			if err != nil {
				return nil, err
			}
			pipelines = append(pipelines, resultPipelines...)
		}
		collected = append(collected, pending...)
		pending = pendingNestedPipelines(pipelines, collected)
	}

	return pipelines, nil
}

type getIngestPipelineResponse map[string]json.RawMessage

func getIngestPipelineByID(ctx context.Context, api *elasticsearch.API, id string) ([]IngestPipeline, error) {
	resp, err := api.Ingest.GetPipeline(
		api.Ingest.GetPipeline.WithContext(ctx),
		api.Ingest.GetPipeline.WithPipelineID(id),
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
		var pipeline IngestPipeline
		err := json.Unmarshal(raw, &pipeline)
		if err != nil {
			return nil, fmt.Errorf("failed to decode pipeline %s: %w", id, err)
		}
		pipeline.id = id
		pipeline.raw = raw
		pipelines = append(pipelines, pipeline)
	}

	return pipelines, nil
}

func pendingNestedPipelines(pipelines []IngestPipeline, collected []string) []string {
	var names []string
	for _, p := range pipelines {
		for _, processor := range p.Processors {
			if processor.Pipeline == nil {
				continue
			}
			name := processor.Pipeline.Name
			if common.StringSliceContains(collected, name) {
				continue
			}
			if common.StringSliceContains(names, name) {
				continue
			}
			names = append(names, name)
		}
	}
	return names
}
