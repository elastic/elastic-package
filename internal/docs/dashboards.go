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

type dashboard struct {
	Attributes struct {
		Title       string
		Description string
	}
}

func renderDashboards(packageRoot string) (string, error) {
	dashboardsDir := filepath.Join(packageRoot, "kibana", "dashboard")

	if _, err := os.Stat(dashboardsDir); os.IsNotExist(err) {
		return "", nil
	}

	var dashboards []dashboard

	err := filepath.WalkDir(dashboardsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if path == dashboardsDir {
			return nil
		}

		if d.IsDir() {
			return filepath.SkipDir
		}

		if filepath.Ext(d.Name()) != ".json" {
			return nil
		}

		rawDashboard, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read dashboard file: %w", err)
		}

		var dash dashboard
		if err := json.Unmarshal(rawDashboard, &dash); err != nil {
			return fmt.Errorf("failed to unmarshal dashboard JSON: %w", err)
		}

		dashboards = append(dashboards, dash)
		return nil
	})

	if err != nil {
		return "", fmt.Errorf("parsing dashboards failed: %w", err)
	}

	var builder strings.Builder

	if len(dashboards) != 0 {
		builder.WriteString("**The following dashboards are available:**\n\n")
		renderDashboardsCollapsibleTable(&builder, dashboards)
		builder.WriteString("\n")
	}

	return builder.String(), nil
}

func renderDashboardsCollapsibleTable(builder *strings.Builder, dashboards []dashboard) {
	builder.WriteString("<details>\n")
	builder.WriteString("<summary>View the dashboards</summary>\n\n")
	builder.WriteString("| Dashboard | Description |\n")
	builder.WriteString("|---|---|\n")
	for _, d := range dashboards {
		title := strings.TrimSpace(d.Attributes.Title)
		description := strings.TrimSpace(strings.ReplaceAll(d.Attributes.Description, "\n", " "))
		fmt.Fprintf(builder, "| **%s** | %s |\n",
			escaper.Replace(title),
			escaper.Replace(description))
	}
	builder.WriteString("\n</details>\n")
}
