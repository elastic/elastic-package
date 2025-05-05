// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package export

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/elastic/elastic-package/internal/elasticsearch"
	"github.com/elastic/elastic-package/internal/elasticsearch/ingest"
)

func IngestPipelines(ctx  context.Context, api *elasticsearch.API, ingestPipelineIDs ...string) error {
	pipelines, err := ingest.GetRemotePipelinesWithNested(ctx, api, ingestPipelineIDs...)

	if err != nil {
		return fmt.Errorf("exporting ingest pipelines using Elasticsearch failed: %w", err)
	}

	pipelineJSON, _ := json.MarshalIndent(pipelines, "", "  ")

	fmt.Printf("Exported ingest pipelines %v\n", string(pipelineJSON))

	return nil
}
