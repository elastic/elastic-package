// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package export

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/elastic/elastic-package/internal/elasticsearch"
	"github.com/elastic/elastic-package/internal/elasticsearch/ingest"
)

type PipelineWriteLocationType string

const (
	PipelineWriteLocationTypeDataStream PipelineWriteLocationType = "data_stream"
	PipelineWriteLocationTypeRoot       PipelineWriteLocationType = "root"
)

// Represents a target write location for exporting an ingest pipeline
type PipelineWriteLocation struct {
	Type       PipelineWriteLocationType
	Name       string
	ParentPath string
}

func (p PipelineWriteLocation) WritePath() string {
	return filepath.Join(p.ParentPath, "elasticsearch", "ingest_pipeline")
}

type PipelineWriteAssignments map[string]PipelineWriteLocation

func IngestPipelines(ctx context.Context, api *elasticsearch.API, writeAssignments PipelineWriteAssignments) error {
	var pipelineIDs []string

	for pipelineID := range writeAssignments {
		pipelineIDs = append(pipelineIDs, pipelineID)
	}

	pipelines, err := ingest.GetRemotePipelinesWithNested(ctx, api, pipelineIDs...)

	if err != nil {
		return fmt.Errorf("exporting ingest pipelines using Elasticsearch failed: %w", err)
	}

	pipelineLookup := make(map[string]ingest.RemotePipeline)
	for _, pipeline := range pipelines {
		pipelineLookup[pipeline.Name()] = pipeline
	}

	err = writePipelinesToFiles(writeAssignments, pipelineLookup)

	if err != nil {
		return err
	}

	return nil
}

func writePipelinesToFiles(writeAssignments PipelineWriteAssignments, pipelineLookup map[string]ingest.RemotePipeline) error {
	if len(writeAssignments) == 0 {
		return nil
	}

	for name, writeLocation := range writeAssignments {
		pipeline, ok := pipelineLookup[name]
		if !ok {
			continue
		}
		err := writePipelineToFile(pipeline, writeLocation)
		if err != nil {
			return err
		}

		depPipelineWriteAssignments := createWriteAssignments(writeLocation, pipeline.GetProcessorPipelineNames())
		err = writePipelinesToFiles(depPipelineWriteAssignments, pipelineLookup)
		if err != nil {
			return err
		}
	}

	return nil
}

func writePipelineToFile(pipeline ingest.RemotePipeline, writeLocation PipelineWriteLocation) error {
	var jsonPipeline map[string]any
	err := json.Unmarshal(pipeline.JSON(), &jsonPipeline)
	if err != nil {
		return fmt.Errorf("unmarshalling ingest pipeline failed (ID: %s): %w", pipeline.Name(), err)
	}

	delete(jsonPipeline, "_meta")
	delete(jsonPipeline, "version")

	var documentBytes bytes.Buffer
	// requirement: https://github.com/elastic/package-spec/pull/54
	documentBytes.WriteString("---\n")
	yamlEncoder := yaml.NewEncoder(&documentBytes)
	yamlEncoder.SetIndent(2)
	err = yamlEncoder.Encode(jsonPipeline)
	if err != nil {
		return fmt.Errorf("marshalling ingest pipeline json to yaml failed (ID: %s): %w", pipeline.Name(), err)
	}

	err = os.MkdirAll(writeLocation.WritePath(), 0755)
	if err != nil {
		return fmt.Errorf("creating target directory failed (path: %s): %w", writeLocation.WritePath(), err)
	}

	pipelineFilePath := filepath.Join(writeLocation.WritePath(), pipeline.Name()+".yml")

	err = os.WriteFile(pipelineFilePath, documentBytes.Bytes(), 0644)

	if err != nil {
		return fmt.Errorf("writing to file '%s' failed: %w", pipelineFilePath, err)
	}

	return nil
}

func createWriteAssignments(writeLocation PipelineWriteLocation, pipelineNames []string) PipelineWriteAssignments {
	writeAssignments := make(PipelineWriteAssignments)
	for _, pipelineName := range pipelineNames {
		writeAssignments[pipelineName] = writeLocation
	}

	return writeAssignments
}
