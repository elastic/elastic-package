// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package ingest

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/elastic/elastic-package/internal/elasticsearch"
	"github.com/elastic/elastic-package/internal/packages"
)

var ingestPipelineTag = regexp.MustCompile(`{{\s*IngestPipeline.+}}`)

func InstallDataStreamPipelines(api *elasticsearch.API, dataStreamPath string) (string, []Pipeline, error) {
	dataStreamManifest, err := packages.ReadDataStreamManifest(filepath.Join(dataStreamPath, packages.DataStreamManifestFile))
	if err != nil {
		return "", nil, fmt.Errorf("reading data stream manifest failed: %w", err)
	}

	nonce := time.Now().UnixNano()

	mainPipeline := getPipelineNameWithNonce(dataStreamManifest.GetPipelineNameOrDefault(), nonce)
	pipelines, err := loadIngestPipelineFiles(dataStreamPath, nonce)
	if err != nil {
		return "", nil, fmt.Errorf("loading ingest pipeline files failed: %w", err)
	}

	err = installPipelinesInElasticsearch(api, pipelines)
	if err != nil {
		return "", nil, err
	}
	return mainPipeline, pipelines, nil
}

func loadIngestPipelineFiles(dataStreamPath string, nonce int64) ([]Pipeline, error) {
	elasticsearchPath := filepath.Join(dataStreamPath, "elasticsearch", "ingest_pipeline")

	var pipelineFiles []string
	for _, pattern := range []string{"*.json", "*.yml"} {
		files, err := filepath.Glob(filepath.Join(elasticsearchPath, pattern))
		if err != nil {
			return nil, fmt.Errorf("listing '%s' in '%s': %w", pattern, elasticsearchPath, err)
		}
		pipelineFiles = append(pipelineFiles, files...)
	}

	var pipelines []Pipeline
	for _, path := range pipelineFiles {
		c, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("reading ingest pipeline failed (path: %s): %w", path, err)
		}

		c = ingestPipelineTag.ReplaceAllFunc(c, func(found []byte) []byte {
			s := strings.Split(string(found), `"`)
			if len(s) != 3 {
				log.Fatalf("invalid IngestPipeline tag in template (path: %s)", path)
			}
			pipelineTag := s[1]
			return []byte(getPipelineNameWithNonce(pipelineTag, nonce))
		})
		name := filepath.Base(path)
		pipelines = append(pipelines, Pipeline{
			Path:    path,
			Name:    getPipelineNameWithNonce(name[:strings.Index(name, ".")], nonce),
			Format:  filepath.Ext(path)[1:],
			Content: c,
		})
	}
	return pipelines, nil
}

func installPipelinesInElasticsearch(api *elasticsearch.API, pipelines []Pipeline) error {
	for _, p := range pipelines {
		if err := installPipeline(api, p); err != nil {
			return err
		}
	}
	return nil
}

func pipelineError(err error, pipeline Pipeline, format string, args ...interface{}) error {
	context := "pipelineName: " + pipeline.Name
	if pipeline.Path != "" {
		context += ", path: " + pipeline.Path
	}

	errorStr := fmt.Sprintf(format+" ("+context+")", args...)
	return fmt.Errorf("%s: %w", errorStr, err)
}

func installPipeline(api *elasticsearch.API, pipeline Pipeline) error {
	if err := putIngestPipeline(api, pipeline); err != nil {
		return err
	}
	// Just to be sure the pipeline has been uploaded.
	return getIngestPipeline(api, pipeline)
}

func putIngestPipeline(api *elasticsearch.API, pipeline Pipeline) error {
	source, err := pipeline.MarshalJSON()
	if err != nil {
		return err
	}
	r, err := api.Ingest.PutPipeline(pipeline.Name, bytes.NewReader(source))
	if err != nil {
		return pipelineError(err, pipeline, "PutPipeline API call failed")
	}
	defer r.Body.Close()

	body, err := io.ReadAll(r.Body)
	if err != nil {
		return pipelineError(err, pipeline, "failed to read PutPipeline API response body")
	}

	if r.StatusCode != http.StatusOK {
		return pipelineError(elasticsearch.NewError(body), pipeline,
			"unexpected response status for PutPipeline (%d): %s",
			r.StatusCode, r.Status())
	}
	return nil
}

func getIngestPipeline(api *elasticsearch.API, pipeline Pipeline) error {
	r, err := api.Ingest.GetPipeline(func(request *elasticsearch.IngestGetPipelineRequest) {
		request.PipelineID = pipeline.Name
	})
	if err != nil {
		return pipelineError(err, pipeline, "GetPipeline API call failed")
	}
	defer r.Body.Close()

	body, err := io.ReadAll(r.Body)
	if err != nil {
		return pipelineError(err, pipeline, "failed to read GetPipeline API response body")
	}

	if r.StatusCode != http.StatusOK {
		return pipelineError(elasticsearch.NewError(body), pipeline,
			"unexpected response status for GetPipeline (%d): %s",
			r.StatusCode, r.Status())
	}
	return nil
}

func getPipelineNameWithNonce(pipelineName string, nonce int64) string {
	return fmt.Sprintf("%s-%d", pipelineName, nonce)
}
