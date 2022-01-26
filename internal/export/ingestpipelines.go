// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package export

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/elastic/elastic-package/internal/elasticsearch"
)

const IngestPipelinesExportDir = "ingest_pipelines"

func IngestPipelines(ctx context.Context, api *elasticsearch.API, output string, ids ...string) error {
	if len(ids) == 0 {
		return nil
	}

	pipelinesDir := filepath.Join(output, ILMPoliciesExportDir)
	err := os.MkdirAll(pipelinesDir, 0755)
	if err != nil {
		return fmt.Errorf("failed to create ingest pipelines directory: %w", err)
	}

	for _, id := range ids {
		err := exportIngestPipeline(ctx, api, pipelinesDir, id)
		if err != nil {
			return err
		}
	}
	return nil
}

func exportIngestPipeline(ctx context.Context, api *elasticsearch.API, output string, id string) error {
	resp, err := api.Ingest.GetPipeline(
		api.Ingest.GetPipeline.WithContext(ctx),
		api.Ingest.GetPipeline.WithPipelineID(id),
		api.Ingest.GetPipeline.WithPretty(),
	)
	if err != nil {
		return fmt.Errorf("failed to get ingest pipeline %s: %w", id, err)
	}
	defer resp.Body.Close()

	path := filepath.Join(output, id+".json")

	w, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open file (%s) to export ingest pipeline: %w", path, err)
	}
	defer w.Close()

	_, err = io.Copy(w, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to export to file: %w", err)
	}
	return nil
}
