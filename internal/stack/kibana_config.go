// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package stack

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	"os"

	"github.com/elastic/go-resource"

	"github.com/elastic/elastic-package/internal/profile"
)

// kibanaConfigWithCustomContent generates kibana.yml with custom config appended
func kibanaConfigWithCustomContent(profile *profile.Profile) func(resource.Context, io.Writer) error {
	return func(ctx resource.Context, w io.Writer) error {
		// First, generate the base kibana.yml from template
		var baseConfig bytes.Buffer
		baseTemplate := staticSource.Template("_static/kibana.yml.tmpl")
		err := baseTemplate(ctx, &baseConfig)
		if err != nil {
			return fmt.Errorf("failed to generate base kibana config: %w", err)
		}

		// Write base config to output
		_, err = w.Write(baseConfig.Bytes())
		if err != nil {
			return fmt.Errorf("failed to write base kibana config: %w", err)
		}

		// Check if custom config file exists
		customConfigPath := profile.Path(KibanaCustomConfigFile)
		customConfigData, err := os.ReadFile(customConfigPath)
		if os.IsNotExist(err) {
			return nil // No custom config file, that's fine
		}
		if err != nil {
			return fmt.Errorf("failed to read custom kibana config: %w", err)
		}

		// Add separator comment
		_, err = w.Write([]byte("\n\n# Custom Kibana Configuration\n"))
		if err != nil {
			return fmt.Errorf("failed to write custom config separator: %w", err)
		}

		// Process custom config as template
		customTemplate, err := template.New("kibana-custom").
			Funcs(templateFuncs).
			Parse(string(customConfigData))
		if err != nil {
			return fmt.Errorf("failed to parse custom kibana config template: %w", err)
		}

		// Create template data from resource context facts
		templateData := createTemplateDataFromContext(ctx)

		err = customTemplate.Execute(w, templateData)
		if err != nil {
			return fmt.Errorf("failed to execute custom kibana config template: %w", err)
		}

		return nil
	}
}

// createTemplateDataFromContext creates template data from resource context
// This function extracts commonly used facts and makes them available to templates
func createTemplateDataFromContext(ctx resource.Context) map[string]interface{} {
	data := make(map[string]interface{})

	// List of facts that should be available in custom templates
	factNames := []string{
		"kibana_version",
		"elasticsearch_version",
		"agent_version",
		"username",
		"password",
		"kibana_host",
		"elasticsearch_host",
		"fleet_url",
		"apm_enabled",
		"logstash_enabled",
		"self_monitor_enabled",
		"kibana_http2_enabled",
		"logsdb_enabled",
		"elastic_subscription",
		"geoip_dir",
		"agent_publish_ports",
		"api_key",
		"enrollment_token",
	}

	// Extract facts from context
	for _, factName := range factNames {
		if value, found := ctx.Fact(factName); found {
			data[factName] = value
		}
	}

	return data
}
