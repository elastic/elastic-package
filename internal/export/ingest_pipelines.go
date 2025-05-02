// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package export

import (
	"context"
	"fmt"

	"github.com/elastic/elastic-package/internal/elasticsearch"
)

func IngestPipelines(ctx  context.Context, elasticsearchClient *elasticsearch.Client, ingestPipelineIDs []string) error {
	pipelinesMap, err := elasticsearchClient.IngestPipelines(ctx, ingestPipelineIDs)

	if err != nil {
		return fmt.Errorf("exporting ingest pipelines using Elasticsearch failed: %w", err)
	}

	fmt.Printf("Exported %v ingest pipelines\n", pipelinesMap)

	return nil
}