// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package export

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/elastic/elastic-package/internal/elasticsearch"
	"github.com/elastic/elastic-package/internal/elasticsearch/ingest"
)

type PipelineWriteLocationType string
const (
	PipelineWriteLocationTypeDataStream PipelineWriteLocationType = "data_stream"
	PipelineWriteLocationTypeRoot	   PipelineWriteLocationType = "root"
)

// Represents a target write location for exporting an ingest pipeline
type PipelineWriteLocation struct {
	Type PipelineWriteLocationType 
	Name string
	ParentPath string
}

func (p PipelineWriteLocation) WritePath() string {
	return filepath.Join(p.ParentPath, "elasticsearch", "ingest-pipeline")
}

type PipelineWriteAssignments map[string]PipelineWriteLocation

func IngestPipelines(ctx  context.Context, api *elasticsearch.API, writeAssignments PipelineWriteAssignments) error {
	var pipelineIDs []string

	for pipelineID, _ := range writeAssignments {
		pipelineIDs = append(pipelineIDs, pipelineID)
	}
	
	pipelines, err := ingest.GetRemotePipelinesWithNested(ctx, api, pipelineIDs...)

	if err != nil {
		return fmt.Errorf("exporting ingest pipelines using Elasticsearch failed: %w", err)
	}

	pipelineJSON, _ := json.MarshalIndent(pipelines, "", "  ")

	fmt.Printf("Exported ingest pipelines %v\n", string(pipelineJSON))

	return nil
}
