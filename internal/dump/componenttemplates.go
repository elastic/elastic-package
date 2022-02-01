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

// ComponentTemplate contains information related to a component template for exporting purpouses.
// It  contains a partially parsed component template and the original JSON from the response.
type ComponentTemplate struct {
	TemplateName      string `json:"name"`
	ComponentTemplate struct {
		Template struct {
			Settings TemplateSettings `json:"settings"`
		} `json:"template"`
	} `json:"component_template"`

	raw json.RawMessage
}

// Name returns the name of the component template.
func (t ComponentTemplate) Name() string {
	return t.TemplateName
}

// JSON returns the JSON representation of the component template.
func (t ComponentTemplate) JSON() []byte {
	return []byte(t.raw)
}

// TemplateSettings returns the template settings of this template.
func (t ComponentTemplate) TemplateSettings() TemplateSettings {
	return t.ComponentTemplate.Template.Settings
}

type getComponentTemplateResponse struct {
	ComponentTemplates []json.RawMessage `json:"component_templates"`
}

func getComponentTemplates(ctx context.Context, api *elasticsearch.API, names ...string) ([]ComponentTemplate, error) {
	if len(names) == 0 {
		return nil, nil
	}

	var templates []ComponentTemplate
	for _, name := range names {
		componentTemplates, err := getComponentTemplatesByName(ctx, api, name)
		if err != nil {
			return nil, err
		}
		templates = append(templates, componentTemplates...)
	}
	return templates, nil
}

func getComponentTemplatesByName(ctx context.Context, api *elasticsearch.API, name string) ([]ComponentTemplate, error) {
	resp, err := api.Cluster.GetComponentTemplate(
		api.Cluster.GetComponentTemplate.WithContext(ctx),
		api.Cluster.GetComponentTemplate.WithName(name),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get component template %s: %w", name, err)
	}
	defer resp.Body.Close()

	d, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var templateResponse getComponentTemplateResponse
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
		componentTemplate.raw = componentTemplateRaw
		componentTemplates = append(componentTemplates, componentTemplate)
	}

	return componentTemplates, nil
}
