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

type GetComponentTemplateResponse struct {
	ComponentTemplates []json.RawMessage `json:"component_templates"`
}

type ComponentTemplate struct {
	Name              string `json:"name"`
	ComponentTemplate struct {
		Template struct {
			Settings struct {
				Index struct {
					Lifecycle struct {
						Name string `json:"name"`
					} `json:"lifecycle"`
				} `json:"index"`
			} `json:"settings"`
		} `json:"template"`
	} `json:"component_template"`
}

const ComponentTemplatesExportDir = "component_templates"

func ComponentTemplates(ctx context.Context, api *elasticsearch.API, output string, names ...string) ([]ComponentTemplate, error) {
	if len(names) == 0 {
		return nil, nil
	}

	templatesDir := filepath.Join(output, ComponentTemplatesExportDir)
	err := os.MkdirAll(templatesDir, 0755)
	if err != nil {
		return nil, fmt.Errorf("failed to create policies directory: %w", err)
	}

	var templates []ComponentTemplate
	for _, name := range names {
		componentTemplates, err := exportComponentTemplates(ctx, api, templatesDir, name)
		if err != nil {
			return nil, err
		}
		templates = append(templates, componentTemplates...)
	}
	return templates, nil
}

func exportComponentTemplates(ctx context.Context, api *elasticsearch.API, output string, name string) ([]ComponentTemplate, error) {
	resp, err := api.Cluster.GetComponentTemplate(
		api.Cluster.GetComponentTemplate.WithContext(ctx),
		api.Cluster.GetComponentTemplate.WithName(name),
		api.Cluster.GetComponentTemplate.WithPretty(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get component template %s: %w", name, err)
	}
	defer resp.Body.Close()

	d, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var templateResponse GetComponentTemplateResponse
	err = json.Unmarshal(d, &templateResponse)
	if err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	var componentTemplates []ComponentTemplate
	for _, componentTemplateRaw := range templateResponse.ComponentTemplates {
		var componentTemplate ComponentTemplate
		err = json.Unmarshal(componentTemplateRaw, &componentTemplate)
		if err != nil {
			return nil, fmt.Errorf("failed to parse component template: %w", err)
		}
		componentTemplates = append(componentTemplates, componentTemplate)

		path := filepath.Join(output, componentTemplate.Name+".json")
		err = ioutil.WriteFile(path, templateResponse.ComponentTemplates[0], 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to export to file: %w", err)
		}
	}

	return componentTemplates, nil
}
