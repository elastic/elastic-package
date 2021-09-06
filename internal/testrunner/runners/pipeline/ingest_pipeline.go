// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package pipeline

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/elastic/go-elasticsearch/v7"
	"github.com/elastic/go-elasticsearch/v7/esapi"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"

	es "github.com/elastic/elastic-package/internal/elasticsearch"
	"github.com/elastic/elastic-package/internal/packages"
)

var ingestPipelineTag = regexp.MustCompile(`{{\s*IngestPipeline.+}}`)

type pipelineResource struct {
	name    string
	format  string
	content []byte
}

type simulatePipelineRequest struct {
	Docs []pipelineDocument `json:"docs"`
}

type pipelineDocument struct {
	Source json.RawMessage `json:"_source"`
}

type simulatePipelineResponse struct {
	Docs []pipelineIngestedDocument `json:"docs"`
}

type pipelineIngestedDocument struct {
	Doc pipelineDocument `json:"doc"`
}

func installIngestPipelines(esClient *elasticsearch.Client, dataStreamPath string) (string, []pipelineResource, error) {
	dataStreamManifest, err := packages.ReadDataStreamManifest(filepath.Join(dataStreamPath, packages.DataStreamManifestFile))
	if err != nil {
		return "", nil, errors.Wrap(err, "reading data stream manifest failed")
	}

	nonce := time.Now().UnixNano()

	mainPipeline := getWithPipelineNameWithNonce(dataStreamManifest.GetPipelineNameOrDefault(), nonce)
	pipelines, err := loadIngestPipelineFiles(dataStreamPath, nonce)
	if err != nil {
		return "", nil, errors.Wrap(err, "loading ingest pipeline files failed")
	}

	jsonPipelines, err := convertPipelineToJSON(pipelines)
	if err != nil {
		return "", nil, errors.Wrap(err, "converting pipelines failed")
	}

	err = installPipelinesInElasticsearch(esClient, jsonPipelines)
	if err != nil {
		return "", nil, errors.Wrap(err, "installing pipelines failed")
	}
	return mainPipeline, jsonPipelines, nil
}

func loadIngestPipelineFiles(dataStreamPath string, nonce int64) ([]pipelineResource, error) {
	elasticsearchPath := filepath.Join(dataStreamPath, "elasticsearch", "ingest_pipeline")

	var pipelineFiles []string
	for _, pattern := range []string{"*.json", "*.yml"} {
		files, err := filepath.Glob(filepath.Join(elasticsearchPath, pattern))
		if err != nil {
			return nil, errors.Wrapf(err, "listing '%s' in '%s'", pattern, elasticsearchPath)
		}
		pipelineFiles = append(pipelineFiles, files...)
	}

	var pipelines []pipelineResource
	for _, path := range pipelineFiles {
		c, err := os.ReadFile(path)
		if err != nil {
			return nil, errors.Wrapf(err, "reading ingest pipeline failed (path: %s)", path)
		}

		c = ingestPipelineTag.ReplaceAllFunc(c, func(found []byte) []byte {
			s := strings.Split(string(found), `"`)
			if len(s) != 3 {
				log.Fatalf("invalid IngestPipeline tag in template (path: %s)", path)
			}
			pipelineTag := s[1]
			return []byte(getWithPipelineNameWithNonce(pipelineTag, nonce))
		})
		name := filepath.Base(path)
		pipelines = append(pipelines, pipelineResource{
			name:    getWithPipelineNameWithNonce(name[:strings.Index(name, ".")], nonce),
			format:  filepath.Ext(path)[1:],
			content: c,
		})
	}
	return pipelines, nil
}

func convertPipelineToJSON(pipelines []pipelineResource) ([]pipelineResource, error) {
	var jsonPipelines []pipelineResource
	for _, pipeline := range pipelines {
		if pipeline.format == "json" {
			jsonPipelines = append(jsonPipelines, pipeline)
			continue
		}

		var node map[string]interface{}
		err := yaml.Unmarshal(pipeline.content, &node)
		if err != nil {
			return nil, errors.Wrapf(err, "unmarshalling pipeline content failed (pipeline: %s)", pipeline.name)
		}

		c, err := json.Marshal(&node)
		if err != nil {
			return nil, errors.Wrapf(err, "marshalling pipeline content failed (pipeline: %s)", pipeline.name)
		}

		jsonPipelines = append(jsonPipelines, pipelineResource{
			name:    pipeline.name,
			format:  "json",
			content: c,
		})
	}
	return jsonPipelines, nil
}

func installPipelinesInElasticsearch(esClient *elasticsearch.Client, pipelines []pipelineResource) error {
	for _, pipeline := range pipelines {
		if err := installPipeline(esClient, pipeline); err != nil {
			return err
		}
	}
	return nil
}

func installPipeline(esClient *elasticsearch.Client, pipeline pipelineResource) error {
	if err := putIngestPipeline(esClient, pipeline); err != nil {
		return err
	}
	// Just to be sure the pipeline has been uploaded.
	return getIngestPipeline(esClient, pipeline.name)
}

func putIngestPipeline(esClient *elasticsearch.Client, pipeline pipelineResource) error {
	r, err := esClient.API.Ingest.PutPipeline(pipeline.name, bytes.NewReader(pipeline.content))
	if err != nil {
		return errors.Wrapf(err, "PutPipeline API call failed (pipelineName: %s)", pipeline.name)
	}
	defer r.Body.Close()

	body, err := io.ReadAll(r.Body)
	if err != nil {
		return errors.Wrapf(err, "failed to read PutPipeline API response body (pipelineName: %s)", pipeline.name)
	}

	if r.StatusCode != 200 {
		return errors.Wrapf(es.NewError(body), "unexpected response status for PutPipeline (%d): %s (pipelineName: %s)",
			r.StatusCode, r.Status(), pipeline.name)
	}
	return nil
}

func getIngestPipeline(esClient *elasticsearch.Client, pipelineName string) error {
	r, err := esClient.API.Ingest.GetPipeline(func(request *esapi.IngestGetPipelineRequest) {
		request.PipelineID = pipelineName
	})
	if err != nil {
		return errors.Wrapf(err, "GetPipeline API call failed (pipelineName: %s)", pipelineName)
	}
	defer r.Body.Close()

	body, err := io.ReadAll(r.Body)
	if err != nil {
		return errors.Wrapf(err, "failed to read GetPipeline API response body (pipelineName: %s)", pipelineName)
	}

	if r.StatusCode != 200 {
		return errors.Wrapf(es.NewError(body), "unexpected response status for GetPipeline (%d): %s (pipelineName: %s)",
			r.StatusCode, r.Status(), pipelineName)
	}
	return nil
}

func uninstallIngestPipelines(esClient *elasticsearch.Client, pipelines []pipelineResource) error {
	for _, pipeline := range pipelines {
		_, err := esClient.API.Ingest.DeletePipeline(pipeline.name)
		if err != nil {
			return errors.Wrapf(err, "DeletePipeline API call failed (pipelineName: %s)", pipeline.name)
		}
	}
	return nil
}

func getWithPipelineNameWithNonce(pipelineName string, nonce int64) string {
	return fmt.Sprintf("%s-%d", pipelineName, nonce)
}

func simulatePipelineProcessing(esClient *elasticsearch.Client, pipelineName string, tc *testCase) (*testResult, error) {
	var request simulatePipelineRequest
	for _, event := range tc.events {
		request.Docs = append(request.Docs, pipelineDocument{
			Source: event,
		})
	}

	requestBody, err := json.Marshal(&request)
	if err != nil {
		return nil, errors.Wrap(err, "marshalling simulate request failed")
	}

	r, err := esClient.API.Ingest.Simulate(bytes.NewReader(requestBody), func(request *esapi.IngestSimulateRequest) {
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

	if r.StatusCode != 200 {
		return nil, errors.Wrapf(es.NewError(body), "unexpected response status for Simulate (%d): %s", r.StatusCode, r.Status())
	}

	var response simulatePipelineResponse
	err = json.Unmarshal(body, &response)
	if err != nil {
		return nil, errors.Wrap(err, "unmarshalling simulate request failed")
	}

	var tr testResult
	for _, doc := range response.Docs {
		tr.events = append(tr.events, doc.Doc.Source)
	}
	return &tr, nil
}
