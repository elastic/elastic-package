// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package ingest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/elastic/elastic-package/internal/elasticsearch"
)

func EnableFailureStore(ctx context.Context, api *elasticsearch.API, indexTemplateName string, enabled bool) error {
	resp, err := api.Indices.GetIndexTemplate(
		api.Indices.GetIndexTemplate.WithContext(ctx),
		api.Indices.GetIndexTemplate.WithName(indexTemplateName),
	)
	if err != nil {
		return fmt.Errorf("failed to get index template %s: %w", indexTemplateName, err)
	}
	defer resp.Body.Close()
	if resp.IsError() {
		return fmt.Errorf("failed to get index template %s: %s", indexTemplateName, resp.String())
	}

	var templateResponse struct {
		IndexTemplates []struct {
			IndexTemplate map[string]any `json:"index_template"`
		} `json:"index_templates"`
	}
	err = json.NewDecoder(resp.Body).Decode(&templateResponse)
	if err != nil {
		return fmt.Errorf("failed to decode response while getting index template %s: %w", indexTemplateName, err)
	}
	if n := len(templateResponse.IndexTemplates); n != 1 {
		return fmt.Errorf("unexpected number of index templates obtained while getting %s, expected 1, found %d", indexTemplateName, err)
	}

	template := templateResponse.IndexTemplates[0].IndexTemplate
	dsSettings, found := template["data_stream"]
	if found {
		dsMap, ok := dsSettings.(map[string]any)
		if !ok {
			return fmt.Errorf("unexpected type for data stream settings in index template %s, expected map, found %T", indexTemplateName, dsMap)
		}
		dsMap["failure_store"] = enabled
		template["data_stream"] = dsMap
	} else {
		template["data_stream"] = map[string]any{
			"failure_store": enabled,
		}
	}

	d, err := json.Marshal(template)
	if err != nil {
		return fmt.Errorf("failed to marshal template after updating it: %w", err)
	}

	updateResp, err := api.Indices.PutIndexTemplate(indexTemplateName, bytes.NewReader(d),
		api.Indices.PutIndexTemplate.WithContext(ctx),
		api.Indices.PutIndexTemplate.WithCreate(false),
	)
	if err != nil {
		return fmt.Errorf("failed to update index template %s: %w", indexTemplateName, err)
	}
	defer updateResp.Body.Close()
	if updateResp.IsError() {
		return fmt.Errorf("failed to update index template %s: %s", indexTemplateName, resp.String())
	}

	return nil
}
