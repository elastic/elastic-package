package pipeline

import (
	"io/ioutil"
	"path/filepath"
	"strings"
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

func installIngestPipelines(datasetPath string) (string, []pipelineResource, error) {
	datasetManifest, err := packages.ReadDatasetManifest(filepath.Join(datasetPath, packages.DatasetManifestFile))
	if err != nil {
		return "", nil, errors.Wrap(err, "reading dataset manifest failed")
	}

	mainPipeline := getPipelineNameOrDefault(datasetManifest)
	pipelines, err := loadIngestPipelineFiles(datasetPath)
	if err != nil {
		return "", nil, errors.Wrap(err, "loading ingest pipeline files failed")
	}

	nonce := time.Now().UnixNano()
	rendered, err := renderPipelineTemplates(pipelines, nonce)
	if err != nil {
		return "", nil, errors.Wrap(err, "rendering pipeline templates failed")
	}

	jsonPipelines, err := convertPipelineToJSON(rendered)
	if err != nil {
		return "", nil, errors.Wrap(err, "converting pipelines failed")
	}

	err = installPipelinesInElasticsearch(jsonPipelines)
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

func loadIngestPipelineFiles(datasetPath string) ([]pipelineResource, error) {
	elasticsearchPath := filepath.Join(datasetPath, "elasticsearch")
	fis, err := ioutil.ReadDir(elasticsearchPath)
	if err != nil {
		return nil, errors.Wrapf(err, "reading ingest pipelines directory failed (path: %s)", elasticsearchPath)
	}

	var pipelines []pipelineResource
	for _, fi := range fis {
		path := filepath.Join(datasetPath, fi.Name())
		c, err := ioutil.ReadFile(path)
		if err != nil {
			return nil, errors.Wrapf(err, "reading pipeline file failed (path: %s)", path)
		}
		pipelines = append(pipelines, pipelineResource{
			name:    fi.Name()[strings.Index(fi.Name(), "."):],
			format:  filepath.Ext(fi.Name())[1:],
			content: c,
		})
	}
	return pipelines, nil
}

func renderPipelineTemplates(pipelines []pipelineResource, nonce int64) ([]pipelineResource, error) {
	return nil, errors.New("not implemented yet") // TODO
}

func convertPipelineToJSON(pipelines []pipelineResource) ([]pipelineResource, error) {
	return nil, errors.New("not implemented yet") // TODO
}

func installPipelinesInElasticsearch(pipelines []pipelineResource) error {
	return errors.New("not implemented yet") // TODO
}

func uninstallIngestPipelines(pipelines []pipelineResource) error {
	return errors.New("not implemented yet") // TODO
}
