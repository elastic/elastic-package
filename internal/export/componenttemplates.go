// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package export

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/elastic/elastic-package/internal/elasticsearch"
)

const ComponentTemplatesExportDir = "component_templates"

func ComponentTemplates(ctx context.Context, api *elasticsearch.API, output string, templates ...string) error {
	if len(templates) == 0 {
		return nil
	}

	templatesDir := filepath.Join(output, ComponentTemplatesExportDir)
	err := os.MkdirAll(templatesDir, 0755)
	if err != nil {
		return fmt.Errorf("failed to create policies directory: %w", err)
	}

	for _, template := range templates {
		err := exportComponentTemplate(ctx, api, templatesDir, template)
		if err != nil {
			return err
		}
	}
	return nil
}

func exportComponentTemplate(ctx context.Context, api *elasticsearch.API, output string, template string) error {
	resp, err := api.Cluster.GetComponentTemplate(
		api.Cluster.GetComponentTemplate.WithContext(ctx),
		api.Cluster.GetComponentTemplate.WithName(template),
		api.Cluster.GetComponentTemplate.WithPretty(),
	)
	if err != nil {
		return fmt.Errorf("failed to get policy %s: %w", template, err)
	}
	defer resp.Body.Close()

	path := filepath.Join(output, template+".json")

	w, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open file (%s) to export component template: %w", path, err)
	}
	defer w.Close()

	_, err = io.Copy(w, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to export to file: %w", err)
	}
	return nil
}
