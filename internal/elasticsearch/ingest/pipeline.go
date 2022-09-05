// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package ingest

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/elastic/elastic-package/internal/elasticsearch"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

type simulatePipelineRequest struct {
	Docs []pipelineDocument `json:"docs"`
}

type simulatePipelineResponse struct {
	Docs []pipelineIngestedDocument `json:"docs"`
}

type pipelineDocument struct {
	Source json.RawMessage `json:"_source"`
}

type pipelineIngestedDocument struct {
	Doc pipelineDocument `json:"doc"`
}

// Pipeline represents a pipeline resource loaded from a file
type Pipeline struct {
	Name    string // Name of the pipeline
	Format  string // Format (extension) of the pipeline
	Content []byte // Content is the original file contents.
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
			return nil, errors.Wrapf(err, "unmarshalling pipeline content failed (pipeline: %s)", p.Name)
		}
		if asJSON, err = json.Marshal(node); err != nil {
			return nil, errors.Wrapf(err, "marshalling pipeline content failed (pipeline: %s)", p.Name)
		}
	default:
		return nil, errors.Errorf("unsupported pipeline format '%s' (pipeline: %s)", p.Format, p.Name)
	}
	return asJSON, nil
}

func SimulatePipeline(api *elasticsearch.API, pipelineName string, events []json.RawMessage) ([]json.RawMessage, error) {
	var request simulatePipelineRequest
	for _, event := range events {
		request.Docs = append(request.Docs, pipelineDocument{
			Source: event,
		})
	}

	requestBody, err := json.Marshal(&request)
	if err != nil {
		return nil, errors.Wrap(err, "marshalling simulate request failed")
	}

	r, err := api.Ingest.Simulate(bytes.NewReader(requestBody), func(request *elasticsearch.IngestSimulateRequest) {
		request.PipelineID = pipelineName
	})
	if err != nil {
		return nil, errors.Wrapf(err, "Simulate API call failed (pipelineName: %s)", pipelineName)
	}
	defer r.Body.Close()

	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read Simulate API response body")
	}

	if r.StatusCode != http.StatusOK {
		return nil, errors.Wrapf(elasticsearch.NewError(body), "unexpected response status for Simulate (%d): %s", r.StatusCode, r.Status())
	}

	var response simulatePipelineResponse
	err = json.Unmarshal(body, &response)
	if err != nil {
		return nil, errors.Wrap(err, "unmarshalling simulate request failed")
	}

	processedEvents := make([]json.RawMessage, len(response.Docs))
	for i, doc := range response.Docs {
		processedEvents[i] = doc.Doc.Source
	}
	return processedEvents, nil
}

func UninstallPipelines(api *elasticsearch.API, pipelines []Pipeline) error {
	for _, p := range pipelines {
		resp, err := api.Ingest.DeletePipeline(p.Name)
		if err != nil {
			return errors.Wrapf(err, "DeletePipeline API call failed (pipelineName: %s)", p.Name)
		}
		resp.Body.Close()
	}
	return nil
}
