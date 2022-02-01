// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package dump

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"

	"github.com/elastic/elastic-package/internal/elasticsearch"
)

// IndexTemplate contains information related to an index template for exporting purpouses.
// It  contains a partially parsed index template and the original JSON from the response.
type IndexTemplate struct {
	TemplateName  string `json:"name"`
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
			Settings TemplateSettings `json:"settings"`
		} `json:"template"`
	} `json:"index_template"`

	raw json.RawMessage
}

// TemplateSettings are common settings to all kinds of templates.
type TemplateSettings struct {
	Index struct {
		DefaultPipeline string `json:"default_pipeline"`
		FinalPipeline   string `json:"final_pipeline"`
		Lifecycle       struct {
			Name string `json:"name"`
		} `json:"lifecycle"`
	} `json:"index"`
}

// Name returns the name of the index template.
func (t IndexTemplate) Name() string {
	return t.TemplateName
}

// JSON returns the JSON representation of the index template.
func (t IndexTemplate) JSON() []byte {
	return []byte(t.raw)
}

// TemplateSettings returns the template settings of this template.
func (t IndexTemplate) TemplateSettings() TemplateSettings {
	return t.IndexTemplate.Template.Settings
}

type getIndexTemplateResponse struct {
	IndexTemplates []json.RawMessage `json:"index_templates"`
}

func getIndexTemplatesForPackage(ctx context.Context, api *elasticsearch.API, packageName string) ([]IndexTemplate, error) {
	resp, err := api.Indices.GetIndexTemplate(
		api.Indices.GetIndexTemplate.WithContext(ctx),

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

	var templateResponse getIndexTemplateResponse
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
		indexTemplate.raw = indexTemplateRaw

		meta := indexTemplate.IndexTemplate.Meta
		if meta.Package.Name != packageName || meta.ManagedBy != "ingest-manager" {
			// This is not the droid you are looking for.
			continue
		}

		indexTemplates = append(indexTemplates, indexTemplate)
	}

	return indexTemplates, nil
}
