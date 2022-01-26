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

type GetIndexTemplateResponse struct {
	IndexTemplates []json.RawMessage `json:"index_templates"`
}

type IndexTemplate struct {
	Name          string
	IndexTemplate struct {
		Meta struct {
			ManagedBy string `json:"managed_by"`
			Managed   bool   `json:"managed"`
			Package   struct {
				Name string `json:"name"`
			} `json:"package"`
		} `json:"_meta"`
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

func IndexTemplatesForPackage(ctx context.Context, api *elasticsearch.API, output string, packageName string) ([]IndexTemplate, error) {
	templatesDir := filepath.Join(output, IndexTemplatesExportDir)
	err := os.MkdirAll(templatesDir, 0755)
	if err != nil {
		return nil, fmt.Errorf("failed to create index templates directory: %w", err)
	}

	resp, err := api.Indices.GetIndexTemplate(
		api.Indices.GetIndexTemplate.WithContext(ctx),
		api.Indices.GetIndexTemplate.WithPretty(),

		// Wildcard may be too wide, we will double check below if it is a managed template.
		api.Indices.GetIndexTemplate.WithName(fmt.Sprintf("*-%s.*", packageName)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get index templates: %w", err)
	}
	defer resp.Body.Close()

	d, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var templateResponse GetIndexTemplateResponse
	err = json.Unmarshal(d, &templateResponse)
	if err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	var indexTemplates []IndexTemplate
	for _, indexTemplateRaw := range templateResponse.IndexTemplates {
		var indexTemplate IndexTemplate
		err = json.Unmarshal(indexTemplateRaw, &indexTemplate)
		if err != nil {
			return nil, fmt.Errorf("failed to parse index template: %w", err)
		}

		meta := indexTemplate.IndexTemplate.Meta
		if meta.Package.Name != packageName || meta.ManagedBy != "ingest-manager" {
			// This is not the droid you are looking for.
			continue
		}

		indexTemplates = append(indexTemplates, indexTemplate)

		path := filepath.Join(templatesDir, indexTemplate.Name+".json")
		err = ioutil.WriteFile(path, templateResponse.IndexTemplates[0], 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to export to file: %w", err)
		}
	}

	return indexTemplates, nil
}
