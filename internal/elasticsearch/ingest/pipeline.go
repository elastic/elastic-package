// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package ingest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/elastic/elastic-package/internal/elasticsearch"
)

type simulatePipelineRequest struct {
	Docs []pipelineDocument `json:"docs"`
}

type simulatePipelineResponse struct {
	Docs []pipelineIngestedDocument `json:"docs"`
}

type pipelineDocument struct {
	Index  string          `json:"_index"`
	Source json.RawMessage `json:"_source"`
}

type pipelineIngestedDocument struct {
	Doc pipelineDocument `json:"doc"`
}

// Pipeline represents a pipeline resource loaded from a file
type Pipeline struct {
	Path            string // Path of the file with the pipeline definition.
	Name            string // Name of the pipeline.
	Format          string // Format (extension) of the pipeline.
	Content         []byte // Content is the pipeline file contents with reroute processors if any.
	ContentOriginal []byte // Content is the original file contents.
}

// Filename returns the original filename associated with the pipeline.
func (p *Pipeline) Filename() string {
	pos := strings.LastIndexByte(p.Name, '-')
	if pos == -1 {
		pos = len(p.Name)
	}
	return p.Name[:pos] + "." + p.Format
}

// MarshalJSON returns the pipeline contents in JSON format.
func (p *Pipeline) MarshalJSON() (asJSON []byte, err error) {
	switch p.Format {
	case "json":
		asJSON = p.Content
	case "yaml", "yml":
		var node map[string]interface{}
		err = yaml.Unmarshal(p.Content, &node)
		if err != nil {
			return nil, fmt.Errorf("unmarshalling pipeline content failed (pipeline: %s): %w", p.Name, err)
		}
		if asJSON, err = json.Marshal(node); err != nil {
			return nil, fmt.Errorf("marshalling pipeline content failed (pipeline: %s): %w", p.Name, err)
		}
	default:
		return nil, fmt.Errorf("unsupported pipeline format '%s' (pipeline: %s)", p.Format, p.Name)
	}
	return asJSON, nil
}

func SimulatePipeline(ctx context.Context, api *elasticsearch.API, pipelineName string, events []json.RawMessage, simulateDataStream string) ([]json.RawMessage, error) {
	var request simulatePipelineRequest
	for _, event := range events {
		request.Docs = append(request.Docs, pipelineDocument{
			Index:  simulateDataStream,
			Source: event,
		})
	}

	requestBody, err := json.Marshal(&request)
	if err != nil {
		return nil, fmt.Errorf("marshalling simulate request failed: %w", err)
	}

	r, err := api.Ingest.Simulate(bytes.NewReader(requestBody), api.Ingest.Simulate.WithContext(ctx), func(request *elasticsearch.IngestSimulateRequest) {
		request.PipelineID = pipelineName
	})
	if err != nil {
		return nil, fmt.Errorf("simulate API call failed (pipelineName: %s): %w", pipelineName, err)
	}
	defer r.Body.Close()

	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read Simulate API response body: %w", err)
	}

	if r.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected response status for Simulate (%d): %s: %w", r.StatusCode, r.Status(), elasticsearch.NewError(body))
	}

	var response simulatePipelineResponse
	err = json.Unmarshal(body, &response)
	if err != nil {
		return nil, fmt.Errorf("unmarshalling simulate request failed: %w", err)
	}

	processedEvents := make([]json.RawMessage, len(response.Docs))
	for i, doc := range response.Docs {
		processedEvents[i] = doc.Doc.Source
	}
	return processedEvents, nil
}

func UninstallPipelines(ctx context.Context, api *elasticsearch.API, pipelines []Pipeline) error {
	for _, p := range pipelines {
		err := uninstallPipeline(ctx, api, p.Name)
		if err != nil {
			return err
		}
	}
	return nil
}

func uninstallPipeline(ctx context.Context, api *elasticsearch.API, name string) error {
	resp, err := api.Ingest.DeletePipeline(name, api.Ingest.DeletePipeline.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("delete pipeline API call failed (pipelineName: %s): %w", name, err)
	}
	defer resp.Body.Close()

	if resp.IsError() {
		return fmt.Errorf("failed to uninstall pipeline %s: %s", name, resp.String())
	}

	return nil
}
