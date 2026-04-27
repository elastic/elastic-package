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

	"github.com/Masterminds/semver/v3"

	"github.com/elastic/elastic-package/internal/packages"
)

// alertRuleTemplatesCollapsibleTableMinSpecVersion is the minimum package spec
// version from which alert rule templates are rendered as a collapsible table.
// For earlier spec versions, the previous rendering (name and description as a
// list) is preserved to avoid introducing a breaking change.
var alertRuleTemplatesCollapsibleTableMinSpecVersion = semver.MustParse("3.6.0")

type alertRuleTemplate struct {
	Attributes struct {
		Name        string
		Description string
	}
}

func renderAlertRuleTemplates(packageRoot string, linksMap linkMap) (string, error) {
	manifest, err := packages.ReadPackageManifestFromPackageRoot(packageRoot)
	if err != nil {
		return "", fmt.Errorf("failed to read package manifest: %w", err)
	}

	specVersion, err := semver.NewVersion(manifest.SpecVersion)
	if err != nil {
		return "", fmt.Errorf("failed to parse package format version %q: %w", manifest.SpecVersion, err)
	}

	useCollapsibleTable := specVersion.GreaterThanEqual(alertRuleTemplatesCollapsibleTableMinSpecVersion)

	templatesDir := filepath.Join(packageRoot, "kibana", "alerting_rule_template")

	if _, err := os.Stat(templatesDir); os.IsNotExist(err) {
		// no template directory in the package, do nothing
		return "", nil
	}

	var templates []alertRuleTemplate

	err = filepath.WalkDir(templatesDir, func(path string, d fs.DirEntry, err error) error {
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

		templates = append(templates, template)
		return nil
	})

	if err != nil {
		return "", fmt.Errorf("parsing alert rule templates failed: %w", err)
	}

	var builder strings.Builder

	if len(templates) != 0 {
		docsLink, err := linksMap.RenderLink("alert-rule-templates", linkOptions{})
		if err != nil {
			docsLink = "https://www.elastic.co/docs"
		}

		builder.WriteString(`Alert rule templates provide pre-defined configurations for creating alert rules in Kibana.

For more information, refer to the [Elastic documentation](` + docsLink + `).

Alert rule templates require Elastic Stack version 9.2.0 or later.

`)

		if useCollapsibleTable {
			renderAlertRuleCollapsibleTable(&builder, templates)
		} else {
			renderAlertRuleList(&builder, templates)
		}
	}

	return builder.String(), nil
}

func renderAlertRuleList(builder *strings.Builder, templates []alertRuleTemplate) {
	builder.WriteString("The following alert rule templates are available:\n\n")
	for _, template := range templates {
		fmt.Fprintf(builder, "**%s**\n\n", template.Attributes.Name)
		fmt.Fprintf(builder, "%s\n\n", template.Attributes.Description)
	}
}

func renderAlertRuleCollapsibleTable(builder *strings.Builder, templates []alertRuleTemplate) {
	builder.WriteString("**The following alert rule templates are available:**\n\n")
	builder.WriteString("<details>\n")
	builder.WriteString("<summary>View the alert rule templates</summary>\n\n")
	builder.WriteString("| Name | Description |\n")
	builder.WriteString("|---|---|\n")
	for _, t := range templates {
		name := strings.TrimSpace(t.Attributes.Name)
		description := strings.TrimSpace(strings.ReplaceAll(t.Attributes.Description, "\n", " "))
		fmt.Fprintf(builder, "| %s | %s |\n",
			escaper.Replace(name),
			escaper.Replace(description))
	}
	builder.WriteString("\n</details>\n")
	builder.WriteString("\n")
}
