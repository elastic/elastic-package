// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package export

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/elastic/elastic-package/internal/elasticsearch"
)

const IndexTemplatesExportDir = "index_templates"

type IndexTemplateResponse struct {
	IndexTemplates []json.RawMessage `json:"index_templates"`
}

type IndexTemplate struct {
	Name          string
	IndexTemplate struct {
		ComposedOf []string `json:"composed_of"`
		Template   struct {
			Settings struct {
				Index struct {
					DefaultPipeline string `json:"default_pipeline"`
				} `json:"index"`
			} `json:"settings"`
		} `json:"template"`
	} `json:"index_template"`
}

func IndexTemplates(ctx context.Context, api *elasticsearch.API, output string, templateNames ...string) ([]IndexTemplate, error) {
	if len(templateNames) == 0 {
		return nil, nil
	}

	templatesDir := filepath.Join(output, IndexTemplatesExportDir)
	err := os.MkdirAll(templatesDir, 0755)
	if err != nil {
		return nil, fmt.Errorf("failed to create index templates directory: %w", err)
	}

	var templates []IndexTemplate
	for _, templateName := range templateNames {
		template, err := exportIndexTemplate(ctx, api, templatesDir, templateName)
		if err != nil {
			return nil, err
		}
		templates = append(templates, template)
	}
	return templates, nil
}

func exportIndexTemplate(ctx context.Context, api *elasticsearch.API, output string, template string) (IndexTemplate, error) {
	resp, err := api.Indices.GetIndexTemplate(
		api.Indices.GetIndexTemplate.WithContext(ctx),
		api.Indices.GetIndexTemplate.WithName(template),
		api.Indices.GetIndexTemplate.WithPretty(),
	)
	if err != nil {
		return IndexTemplate{}, fmt.Errorf("failed to get index template %s: %w", template, err)
	}
	defer resp.Body.Close()

	d, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return IndexTemplate{}, fmt.Errorf("failed to read response body: %w", err)
	}

	var templateResponse IndexTemplateResponse
	err = json.Unmarshal(d, &templateResponse)
	if err != nil {
		return IndexTemplate{}, fmt.Errorf("failed to decode response: %w", err)
	}
	if n := len(templateResponse.IndexTemplates); n != 1 {
		return IndexTemplate{}, fmt.Errorf("%d templates received, only one expected for name %s", n, template)
	}

	path := filepath.Join(output, template+".json")
	err = ioutil.WriteFile(path, templateResponse.IndexTemplates[0], 0644)
	if err != nil {
		return IndexTemplate{}, fmt.Errorf("failed to export to file: %w", err)
	}

	var indexTemplate IndexTemplate
	err = json.Unmarshal(templateResponse.IndexTemplates[0], &indexTemplate)
	if err != nil {
		return IndexTemplate{}, fmt.Errorf("failed to parse index template: %w", err)
	}

	return indexTemplate, nil
}
