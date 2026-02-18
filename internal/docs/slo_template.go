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

type sloTemplate struct {
	Attributes struct {
		Name        string
		Description string
	}
}

func renderSloTemplates(packageRoot string, linksMap linkMap) (string, error) {
	templatesDir := filepath.Join(packageRoot, "kibana", "slo_template")

	if _, err := os.Stat(templatesDir); os.IsNotExist(err) {
		return "", nil
	}

	var templates []sloTemplate

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
			return fmt.Errorf("failed to read slo template file: %w", err)
		}

		var template sloTemplate
		if err := json.Unmarshal(rawTemplate, &template); err != nil {
			return fmt.Errorf("failed to unmarshal slo template JSON: %w", err)
		}

		templates = append(templates, template)
		return nil
	})

	if err != nil {
		return "", fmt.Errorf("parsing slo templates failed: %w", err)
	}

	var builder strings.Builder

	if len(templates) != 0 {
		docsLink, err := linksMap.RenderLink("slo-templates", linkOptions{})
		if err != nil {
			docsLink = "https://www.elastic.co/docs"
		}

		builder.WriteString(`SLO templates provide pre-defined configurations for creating SLOs in Kibana.

For more information, refer to the [Elastic documentation](` + docsLink + `).

SLO templates require Elastic Stack version 9.4.0 or later.

`)
		builder.WriteString("**The following SLO templates are available:**\n\n")
		renderSloCollapsibleTable(&builder, templates)
		builder.WriteString("\n")
	}

	return builder.String(), nil
}

func renderSloCollapsibleTable(builder *strings.Builder, templates []sloTemplate) {
	builder.WriteString("<details>\n")
	builder.WriteString("<summary>Click to expand SLO templates</summary>\n\n")
	builder.WriteString("| Name | Description |\n")
	builder.WriteString("|---|---|\n")
	for _, t := range templates {
		name := strings.TrimSpace(t.Attributes.Name)
		description := strings.TrimSpace(strings.ReplaceAll(t.Attributes.Description, "\n", " "))
		builder.WriteString(fmt.Sprintf("| %s | %s |\n",
			escaper.Replace(name),
			escaper.Replace(description)))
	}
	builder.WriteString("\n</details>\n")
}
