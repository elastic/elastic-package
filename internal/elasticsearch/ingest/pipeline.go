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
	"slices"
	"sort"
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

// RemotePipeline represents resource retrieved from Elasticsearch
type RemotePipeline struct {
	Processors []struct {
		Pipeline *struct {
			Name string `json:"name"`
		} `json:"pipeline,omitempty"`
	} `json:"processors"`
	id  string
	raw []byte
}

// Name returns the name of the ingest pipeline.
func (p RemotePipeline) Name() string {
	return p.id
}

// JSON returns the JSON representation of the ingest pipeline.
func (p RemotePipeline) JSON() []byte {
	return p.raw
}

func (p RemotePipeline) GetProcessorPipelineNames() []string {
	var names []string
	for _, processor := range p.Processors {
		if processor.Pipeline == nil {
			continue
		}
		name := processor.Pipeline.Name
		if slices.Contains(names, name) {
			continue
		}
		names = append(names, name)
	}
	return names
}

func GetRemotePipelineNames(ctx context.Context, api *elasticsearch.API) ([]string, error) {
	resp, err := api.Ingest.GetPipeline(
		api.Ingest.GetPipeline.WithContext(ctx),
		api.Ingest.GetPipeline.WithSummary(true),
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get ingest pipeline names: %w", err)
	}
	defer resp.Body.Close()

	if resp.IsError() {
		return nil, fmt.Errorf("error getting ingest pipeline names: %s", resp)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading ingest pipeline names body: %w", err)
	}

	pipelineMap := map[string]struct {
		Description string `json:"description"`
	}{}

	if err := json.Unmarshal(body, &pipelineMap); err != nil {
		return nil, fmt.Errorf("error unmarshaling ingest pipeline names: %w", err)
	}

	pipelineNames := []string{}

	for name := range pipelineMap {
		pipelineNames = append(pipelineNames, name)
	}

	sort.Slice(pipelineNames, func(i, j int) bool {
		return sort.StringsAreSorted([]string{strings.ToLower(pipelineNames[i]), strings.ToLower(pipelineNames[j])})
	})

	return pipelineNames, nil
}

func GetRemotePipelines(ctx context.Context, api *elasticsearch.API, ids ...string) ([]RemotePipeline, error) {

	commaSepIDs := strings.Join(ids, ",")

	resp, err := api.Ingest.GetPipeline(
		api.Ingest.GetPipeline.WithContext(ctx),
		api.Ingest.GetPipeline.WithPipelineID(commaSepIDs),
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get ingest pipelines: %w", err)
	}
	defer resp.Body.Close()

	// Ingest templates referenced by other templates may not exist.
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}

	if resp.IsError() {
		return nil, fmt.Errorf("failed to get ingest pipelines %s: %s", ids, resp.String())
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	pipelinesResponse := map[string]json.RawMessage{}
	if err := json.Unmarshal(body, &pipelinesResponse); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	var pipelines []RemotePipeline
	for id, raw := range pipelinesResponse {
		var pipeline RemotePipeline
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

func GetRemotePipelinesWithNested(ctx context.Context, api *elasticsearch.API, ids ...string) ([]RemotePipeline, error) {
	var pipelines []RemotePipeline
	var collected []string
	pending := ids
	for len(pending) > 0 {
		resultPipelines, err := GetRemotePipelines(ctx, api, pending...)
		if err != nil {
			return nil, err
		}
		pipelines = append(pipelines, resultPipelines...)
		collected = append(collected, pending...)
		pending = pendingNestedPipelines(pipelines, collected)
	}

	return pipelines, nil
}

func pendingNestedPipelines(pipelines []RemotePipeline, collected []string) []string {
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

	r, err := api.Ingest.Simulate(bytes.NewReader(requestBody),
		api.Ingest.Simulate.WithContext(ctx),
		api.Ingest.Simulate.WithPipelineID(pipelineName),
	)
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
