package pipeline

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/elastic/go-elasticsearch/v7"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/packages"
)

const defaultPipelineName = "default"

type pipelineResource struct {
	name    string
	format  string
	content []byte
}

func installIngestPipelines(esClient *elasticsearch.Client, datasetPath string) (string, []pipelineResource, error) {
	datasetManifest, err := packages.ReadDatasetManifest(filepath.Join(datasetPath, packages.DatasetManifestFile))
	if err != nil {
		return "", nil, errors.Wrap(err, "reading dataset manifest failed")
	}

	nonce := time.Now().UnixNano()

	mainPipeline := getWithPipelineNameWithNonce(getPipelineNameOrDefault(datasetManifest), nonce)
	pipelines, err := loadIngestPipelineFiles(datasetPath, nonce)
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

func getPipelineNameOrDefault(datasetManifest *packages.DatasetManifest) string {
	if datasetManifest.Elasticsearch != nil && datasetManifest.Elasticsearch.IngestPipelineName != "" {
		return datasetManifest.Elasticsearch.IngestPipelineName
	}
	return defaultPipelineName
}

func loadIngestPipelineFiles(datasetPath string, nonce int64) ([]pipelineResource, error) {
	elasticsearchPath := filepath.Join(datasetPath, "elasticsearch")
	fis, err := ioutil.ReadDir(elasticsearchPath)
	if err != nil {
		return nil, errors.Wrapf(err, "reading ingest pipelines directory failed (path: %s)", elasticsearchPath)
	}

	var paths []string
	for _, fi := range fis {
		path := filepath.Join(datasetPath, fi.Name())
		paths = append(paths, path)
	}

	t, err := template.New("pipeline").
		Funcs(map[string]interface{}{
			"IngestPipeline": func(pipelineName string) string {
				return getWithPipelineNameWithNonce(pipelineName, nonce)
			}},
		).
		ParseFiles(paths...)
	if err != nil {
		return nil, errors.Wrap(err, "parsing ingest pipeline failed")
	}

	var pipelines []pipelineResource
	for _, fi := range fis {
		var buffer bytes.Buffer
		err := t.ExecuteTemplate(&buffer, fi.Name(), nil)
		if err != nil {
			return nil, errors.Wrapf(err, "executing pipeline template failed (filename: %s)", fi.Name())
		}

		pipelines = append(pipelines, pipelineResource{
			name:    getWithPipelineNameWithNonce(fi.Name()[strings.Index(fi.Name(), "."):], nonce),
			format:  filepath.Ext(fi.Name())[1:],
			content: buffer.Bytes(),
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

		c, err := json.Marshal(node)
		if err != nil {
			return nil, errors.Wrapf(err, "marshalling pipeline content failed (pipeline: %s)", pipeline.name)
		}

		jsonPipelines = append(jsonPipelines, pipelineResource{
			name:    pipeline.name,
			format:  pipeline.format,
			content: c,
		})
	}
	return jsonPipelines, nil
}

func installPipelinesInElasticsearch(esClient *elasticsearch.Client, pipelines []pipelineResource) error {
	for _, pipeline := range pipelines {
		_, err := esClient.API.Ingest.PutPipeline(pipeline.name, bytes.NewReader(pipeline.content))
		if err != nil {
			return errors.Wrapf(err, "PutPipeline API call failed (pipelineName: %s)", pipeline.name)
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
