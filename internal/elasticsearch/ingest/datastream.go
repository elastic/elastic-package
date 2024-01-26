// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package ingest

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/elastic/elastic-package/internal/elasticsearch"
	"github.com/elastic/elastic-package/internal/packages"
)

var (
	ingestPipelineTag   = regexp.MustCompile(`{{\s*IngestPipeline.+}}`)
	defaultPipelineJSON = "default.json"
	defaultPipelineYML  = "default.yml"
)

type Rule struct {
	TargetDataset interface{} `yaml:"target_dataset"`
	If            string      `yaml:"if"`
	Namespace     interface{} `yaml:"namespace"`
}

type RoutingRule struct {
	SourceDataset string `yaml:"source_dataset"`
	Rules         []Rule `yaml:"rules"`
}

type RerouteProcessor struct {
	Tag       string   `yaml:"tag"`
	If        string   `yaml:"if"`
	Dataset   []string `yaml:"dataset"`
	Namespace []string `yaml:"namespace"`
}

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
				err = fmt.Errorf("invalid IngestPipeline tag in template (path: %s)", path)
				return nil
			}
			pipelineTag := s[1]
			return []byte(getPipelineNameWithNonce(pipelineTag, nonce))
		})
		if err != nil {
			return nil, err
		}

		cWithRerouteProcessors, err := addRerouteProcessors(c, dataStreamPath, path)
		if err != nil {
			return nil, err
		}

		name := filepath.Base(path)
		pipelines = append(pipelines, Pipeline{
			Path:            path,
			Name:            getPipelineNameWithNonce(name[:strings.Index(name, ".")], nonce),
			Format:          filepath.Ext(path)[1:],
			Content:         cWithRerouteProcessors,
			ContentOriginal: c,
		})
	}
	return pipelines, nil
}

func addRerouteProcessors(pipeline []byte, dataStreamPath, path string) ([]byte, error) {
	// Only attach routing_rules.yml reroute processors after the default pipeline
	filename := filepath.Base(path)
	if filename != defaultPipelineJSON && filename != defaultPipelineYML {
		return pipeline, nil
	}

	// Read routing_rules.yml and convert it into reroute processors in ingest pipeline
	rerouteProcessors, err := loadRoutingRuleFile(dataStreamPath)
	if err != nil {
		return nil, fmt.Errorf("failed loading routing rules: %v", err)
	}
	if len(rerouteProcessors) == 0 {
		return pipeline, nil
	}

	var yamlPipeline map[string]any
	err = yaml.Unmarshal(pipeline, &yamlPipeline)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal ingest pipeline YAML data (path: %s): %w", path, err)
	}

	var processors []any
	v, found := yamlPipeline["processors"]
	if found {
		list, ok := v.([]any)
		if !ok {
			return nil, fmt.Errorf("unexpected processors type, expected []any, found %T", v)
		}
		processors = list
	}
	for _, p := range rerouteProcessors {
		processors = append(processors, p)
	}
	yamlPipeline["processors"] = processors

	pipeline, err = yaml.Marshal(yamlPipeline)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal modified ingest pipeline YAML data: %v", err)
	}

	return pipeline, nil
}

func loadRoutingRuleFile(dataStreamPath string) ([]map[string]interface{}, error) {
	routingRulePath := filepath.Join(dataStreamPath, "routing_rules.yml")
	c, err := os.ReadFile(routingRulePath)
	if err != nil {
		// routing_rules.yml does not exist
		if os.IsNotExist(err) {
			return nil, nil
		} else {
			return nil, fmt.Errorf("reading routing_rules.yml failed (path: %s): %w", routingRulePath, err)
		}
	}

	// unmarshal yaml into a struct
	var routingRule []RoutingRule
	err = yaml.Unmarshal(c, &routingRule)
	if err != nil {
		return nil, fmt.Errorf("unmarshalling routing_rules.yml content failed: %w", err)
	}

	// Now you can work with the data as Go structs
	var rerouteProcessors []map[string]interface{}
	for _, r := range routingRule {
		for _, rule := range r.Rules {
			td, err := convertValue(rule.TargetDataset, "target_dataset")
			if err != nil {
				return nil, fmt.Errorf("convertValue failed: %w", err)
			}

			ns, err := convertValue(rule.Namespace, "namespace")
			if err != nil {
				return nil, fmt.Errorf("convertValue failed: %w", err)
			}

			processor := make(map[string]interface{})
			processor["reroute"] = RerouteProcessor{
				Tag:       r.SourceDataset,
				If:        rule.If,
				Dataset:   td,
				Namespace: ns,
			}
			rerouteProcessors = append(rerouteProcessors, processor)
		}
	}
	return rerouteProcessors, nil
}

func convertValue(value interface{}, label string) ([]string, error) {
	switch value := value.(type) {
	case string:
		return []string{value}, nil
	case []string:
		return value, nil
	case []interface{}:
		result := make([]string, 0, len(value))
		for _, v := range value {
			if vStr, ok := v.(string); ok {
				result = append(result, vStr)
			} else {
				return nil, fmt.Errorf("%s in routing_rules.yml has to be a string or an array of strings: %v", label, value)
			}
		}
		return result, nil
	case nil:
		// namespace is not required in routing_rules.yml
		if label != "namespace" {
			return nil, fmt.Errorf("%s in routing_rules.yml cannot be empty", label)
		}
		return nil, nil
	default:
		return nil, fmt.Errorf("%s in routing_rules.yml has to be a string or an array of strings: %v", label, value)
	}
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
