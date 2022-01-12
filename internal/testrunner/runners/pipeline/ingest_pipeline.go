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
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/elasticsearch"
	"github.com/elastic/elastic-package/internal/elasticsearch/ingest"
	"github.com/elastic/elastic-package/internal/packages"
)

var ingestPipelineTag = regexp.MustCompile(`{{\s*IngestPipeline.+}}`)

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

func installIngestPipelines(api *elasticsearch.API, dataStreamPath string) (string, []ingest.Pipeline, error) {
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

	err = installPipelinesInElasticsearch(api, pipelines)

	if err != nil {
		return "", nil, errors.Wrap(err, "installing pipelines failed")
	}
	return mainPipeline, pipelines, nil
}

func loadIngestPipelineFiles(dataStreamPath string, nonce int64) ([]ingest.Pipeline, error) {
	elasticsearchPath := filepath.Join(dataStreamPath, "elasticsearch", "ingest_pipeline")

	var pipelineFiles []string
	for _, pattern := range []string{"*.json", "*.yml"} {
		files, err := filepath.Glob(filepath.Join(elasticsearchPath, pattern))
		if err != nil {
			return nil, errors.Wrapf(err, "listing '%s' in '%s'", pattern, elasticsearchPath)
		}
		pipelineFiles = append(pipelineFiles, files...)
	}

	var pipelines []ingest.Pipeline
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
		pipelines = append(pipelines, ingest.Pipeline{
			Name:    getWithPipelineNameWithNonce(name[:strings.Index(name, ".")], nonce),
			Format:  filepath.Ext(path)[1:],
			Content: c,
		})
	}
	return pipelines, nil
}

func installPipelinesInElasticsearch(api *elasticsearch.API, pipelines []ingest.Pipeline) error {
	for _, p := range pipelines {
		if err := installPipeline(api, p); err != nil {
			return err
		}
	}
	return nil
}

func installPipeline(api *elasticsearch.API, pipeline ingest.Pipeline) error {
	if err := putIngestPipeline(api, pipeline); err != nil {
		return err
	}
	// Just to be sure the pipeline has been uploaded.
	return getIngestPipeline(api, pipeline.Name)
}

func putIngestPipeline(api *elasticsearch.API, pipeline ingest.Pipeline) error {
	source, err := pipeline.MarshalJSON()
	if err != nil {
		return err
	}
	r, err := api.Ingest.PutPipeline(pipeline.Name, bytes.NewReader(source))
	if err != nil {
		return errors.Wrapf(err, "PutPipeline API call failed (pipelineName: %s)", pipeline.Name)
	}
	defer r.Body.Close()

	body, err := io.ReadAll(r.Body)
	if err != nil {
		return errors.Wrapf(err, "failed to read PutPipeline API response body (pipelineName: %s)", pipeline.Name)
	}

	if r.StatusCode != http.StatusOK {

		return errors.Wrapf(elasticsearch.NewError(body), "unexpected response status for PutPipeline (%d): %s (pipelineName: %s)",
			r.StatusCode, r.Status(), pipeline.Name)
	}
	return nil
}

func getIngestPipeline(api *elasticsearch.API, pipelineName string) error {
	r, err := api.Ingest.GetPipeline(func(request *elasticsearch.IngestGetPipelineRequest) {
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

	if r.StatusCode != http.StatusOK {
		return errors.Wrapf(elasticsearch.NewError(body), "unexpected response status for GetPipeline (%d): %s (pipelineName: %s)",
			r.StatusCode, r.Status(), pipelineName)
	}
	return nil
}

func uninstallIngestPipelines(api *elasticsearch.API, pipelines []ingest.Pipeline) error {
	for _, pipeline := range pipelines {
		resp, err := api.Ingest.DeletePipeline(pipeline.Name)
		if err != nil {
			return errors.Wrapf(err, "DeletePipeline API call failed (pipelineName: %s)", pipeline.Name)
		}
		resp.Body.Close()
	}
	return nil
}

func getWithPipelineNameWithNonce(pipelineName string, nonce int64) string {
	return fmt.Sprintf("%s-%d", pipelineName, nonce)
}

func simulatePipelineProcessing(api *elasticsearch.API, pipelineName string, tc *testCase) (*testResult, error) {
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

	var tr testResult
	for _, doc := range response.Docs {
		tr.events = append(tr.events, doc.Doc.Source)
	}
	return &tr, nil
}
