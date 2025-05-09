// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package dump

import (
	"context"

	"github.com/elastic/elastic-package/internal/elasticsearch"
	"github.com/elastic/elastic-package/internal/elasticsearch/ingest"
)

func getIngestPipelines(ctx context.Context, api *elasticsearch.API, ids ...string) ([]RemoteIngestPipeline, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	return ingest.GetRemotePipelinesWithNested(ctx, api, ids...)
}
