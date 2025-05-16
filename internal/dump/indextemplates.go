// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package dump

import (
	"context"

	"github.com/elastic/elastic-package/internal/elasticsearch"
	"github.com/elastic/elastic-package/internal/elasticsearch/ingest"
)

type IndexTemplate = ingest.IndexTemplate
type TemplateSettings = ingest.TemplateSettings
type RemoteIngestPipeline = ingest.RemotePipeline

func getIndexTemplatesForPackage(ctx context.Context, api *elasticsearch.API, packageName string) ([]ingest.IndexTemplate, error) {
	return ingest.GetIndexTemplatesForPackage(ctx, api, packageName)
}
