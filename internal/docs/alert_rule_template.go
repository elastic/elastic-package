// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package docs

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

type alertRuleTemplate struct {
	Attributes struct {
		Name        string
		Description string
	}
}

func renderAlertRuleTemplates(packageRoot string) (string, error) {
	templatesDir := filepath.Join(packageRoot, "kibana", "alerting_rule_template")

	if _, err := os.Stat(templatesDir); os.IsNotExist(err) {
		// no template directory in the package, do nothing
		return "", nil
	}

	var builder strings.Builder
	builder.WriteString(`Alert rule templates provide pre-defined configurations for creating alert rules in Kibana.

For more information, refer to the [Elastic documentation](https://www.elastic.co/docs/reference/fleet/alert-templates#alert-templates).

Alert rule templates require Elastic Stack version 9.2.0 or later.

`)

	builder.WriteString("The following alert rule templates are available:\n\n")

	err := filepath.WalkDir(templatesDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if path == templatesDir {
			return nil
		}

		if d.IsDir() {
			return filepath.SkipDir
		}

		if filepath.Ext(d.Name()) != ".json" {
			return nil
		}

		rawTemplate, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read alert rule template file: %w", err)
		}

		var template alertRuleTemplate
		if err := json.Unmarshal(rawTemplate, &template); err != nil {
			return fmt.Errorf("failed to unmarshal alert rule template JSON: %w", err)
		}

		builder.WriteString(fmt.Sprintf("**%s**\n\n", template.Attributes.Name))
		builder.WriteString(fmt.Sprintf("%s\n\n", template.Attributes.Description))
		return nil
	})

	if err != nil {
		return "", fmt.Errorf("processing alert rule templates failed: %w", err)
	}

	return builder.String(), nil
}
