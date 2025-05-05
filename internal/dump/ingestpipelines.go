// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package dump

import (
	"context"
	"slices"

	"github.com/elastic/elastic-package/internal/elasticsearch"
	"github.com/elastic/elastic-package/internal/elasticsearch/ingest"
)


func getIngestPipelines(ctx context.Context, api *elasticsearch.API, ids ...string) ([]RemoteIngestPipeline, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	var pipelines []ingest.RemotePipeline
	var collected []string
	pending := ids
	for len(pending) > 0 {
		for _, id := range pending {
			resultPipelines, err := ingest.GetRemotePipelines(ctx, api, id)
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

func pendingNestedPipelines(pipelines []RemoteIngestPipeline, collected []string) []string {
	var names []string
	for _, p := range pipelines {
		for _, processor := range p.Processors {
			if processor.Pipeline == nil {
				continue
			}
			name := processor.Pipeline.Name
			if slices.Contains(collected, name) {
				continue
			}
			if slices.Contains(names, name) {
				continue
			}
			names = append(names, name)
		}
	}
	return names
}
