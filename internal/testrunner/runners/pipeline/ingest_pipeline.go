// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package pipeline

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/elastic/go-elasticsearch/v7"
	"github.com/elastic/go-elasticsearch/v7/esapi"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"

	"github.com/elastic/elastic-package/internal/packages"
)

const defaultPipelineName = "default"

var ingestPipelineTag = regexp.MustCompile("{{\\s*IngestPipeline.+}}")

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

	mainPipeline := getWithPipelineNameWithNonce(getPipelineNameOrDefault(dataStreamManifest), nonce)
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

func getPipelineNameOrDefault(dsm *packages.DataStreamManifest) string {
	if dsm.Elasticsearch != nil && dsm.Elasticsearch.IngestPipeline != nil && dsm.Elasticsearch.IngestPipeline.Name != "" {
		return dsm.Elasticsearch.IngestPipeline.Name
	}
	return defaultPipelineName
}

func loadIngestPipelineFiles(dataStreamPath string, nonce int64) ([]pipelineResource, error) {
	elasticsearchPath := filepath.Join(dataStreamPath, "elasticsearch", "ingest_pipeline")
	fis, err := ioutil.ReadDir(elasticsearchPath)
	if err != nil {
		return nil, errors.Wrapf(err, "reading ingest pipelines directory failed (path: %s)", elasticsearchPath)
	}

	var pipelines []pipelineResource
	for _, fi := range fis {
		path := filepath.Join(elasticsearchPath, fi.Name())
		c, err := ioutil.ReadFile(path)
		if err != nil {
			return nil, errors.Wrap(err, "reading ingest pipeline failed")
		}

		c = ingestPipelineTag.ReplaceAllFunc(c, func(found []byte) []byte {
			s := strings.Split(string(found), `"`)
			if len(s) != 3 {
				log.Fatalf("invalid IngestPipeline tag in template (path: %s)", path)
			}
			pipelineTag := s[1]
			return []byte(getWithPipelineNameWithNonce(pipelineTag, nonce))
		})
		pipelines = append(pipelines, pipelineResource{
			name:    getWithPipelineNameWithNonce(fi.Name()[:strings.Index(fi.Name(), ".")], nonce),
			format:  filepath.Ext(fi.Name())[1:],
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
		r, err := esClient.API.Ingest.PutPipeline(pipeline.name, bytes.NewReader(pipeline.content))
		if err != nil {
			return errors.Wrapf(err, "PutPipeline API call failed (pipelineName: %s)", pipeline.name)
		}

		if r.StatusCode != 200 {
			return fmt.Errorf("unexpected response status for PutPipeline (%d): %s", r.StatusCode, r.Status())
		}

		// Just to be sure the pipeline has been uploaded
		r, err = esClient.API.Ingest.GetPipeline(func(request *esapi.IngestGetPipelineRequest) {
			request.PipelineID = pipeline.name
		})
		if err != nil {
			return errors.Wrapf(err, "GetPipeline API call failed (pipelineName: %s)", pipeline.name)
		}

		if r.StatusCode != 200 {
			return fmt.Errorf("unexpected response status for GetPipeline (%d): %s", r.StatusCode, r.Status())
		}
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

	if r.StatusCode != 200 {
		return nil, fmt.Errorf("unexpected response status for Simulate (%d): %s", r.StatusCode, r.Status())
	}

	body, err := ioutil.ReadAll(r.Body)
	defer r.Body.Close()

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
