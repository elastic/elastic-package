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
// version from which a single-engine package renders alert rule templates as a
// collapsible table. Earlier specs keep the paragraph layout. A package that publishes both
// engines uses one collapsible table with a column per engine.
var alertRuleTemplatesCollapsibleTableMinSpecVersion = semver.MustParse("3.6.0")

var alertRuleNameEscaper = strings.NewReplacer(
	"\\", "\\\\", "*", "\\*", "_", "\\_", "`", "\\`", "~", "\\~",
	"[", "\\[", "]", "\\]", "{", "\\{", "}", "\\}", "<", "\\<", ">", "\\>",
	"|", "\\|", "&", "&amp;",
)

var alertRuleLineBreaks = strings.NewReplacer("\r\n", " ", "\r", " ", "\n", " ")

type alertRuleTemplate struct {
	Attributes struct {
		// Classic templates carry name and description on attributes.
		Name        string `json:"name"`
		Description string `json:"description"`
		// Alerting v2 templates use { engine: "v2", rule: { metadata: {...} } }.
		Engine string `json:"engine"`
		Rule   struct {
			Metadata struct {
				Name        string `json:"name"`
				Description string `json:"description"`
			} `json:"metadata"`
		} `json:"rule"`
	} `json:"attributes"`
}

func (t alertRuleTemplate) isV2() bool {
	return t.Attributes.Engine == "v2"
}

func (t alertRuleTemplate) displayName() string {
	if t.isV2() {
		return t.Attributes.Rule.Metadata.Name
	}
	return t.Attributes.Name
}

func (t alertRuleTemplate) displayDescription() string {
	if t.isV2() {
		return t.Attributes.Rule.Metadata.Description
	}
	return t.Attributes.Description
}

// alertRuleTemplateCompatibility states the Stack floor for the engines present.
// Standard (classic) templates require 9.2.0. ES|QL (Alerting v2) requires
// 9.6.0. A package that publishes both says so separately, so the classic floor
// is not raised silently and the v2 floor is not omitted.
func alertRuleTemplateCompatibility(hasV1, hasV2 bool) string {
	switch {
	case hasV1 && hasV2:
		return "Standard alert rule templates require Elastic Stack version 9.2.0 or later. ES|QL alert rule templates require Elastic Stack version 9.6.0 or later.\n"
	case hasV2:
		return "Alert rule templates require Elastic Stack version 9.6.0 or later.\n"
	default:
		return "Alert rule templates require Elastic Stack version 9.2.0 or later.\n"
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

		hasV1, hasV2 := false, false
		for _, template := range templates {
			if template.isV2() {
				hasV2 = true
			} else {
				hasV1 = true
			}
		}

		builder.WriteString(`Alert rule templates provide pre-defined configurations for creating alert rules in Kibana.

For more information, refer to the [Elastic documentation](` + docsLink + `).

`)
		builder.WriteString(alertRuleTemplateCompatibility(hasV1, hasV2))
		builder.WriteString("\n")

		// Both engines are listed under the same headings Fleet uses on the Alerting
		// tab, ES|QL then Standard, so each template name appears once. A single
		// engine keeps the collapsible table from spec 3.6.0 and paragraphs before that.
		if hasV1 && hasV2 {
			renderAlertRuleEngineSections(&builder, templates)
		} else if useCollapsibleTable {
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
		fmt.Fprintf(builder, "**%s**\n\n", template.displayName())
		fmt.Fprintf(builder, "%s\n\n", template.displayDescription())
	}
}

// alertRuleGroup is one alert name published on one or both engines.
type alertRuleGroup struct {
	name        string
	description string
	v2          bool
	v1          bool
}

// renderAlertRuleEngineSections renders a dual-publish package as one table.
// Each alert is a single row, with a centered mark in the ES|QL and Standard
// columns, so the name is not repeated. When two templates share a name but
// their descriptions differ, or one engine has two templates with that name,
// the table falls back to one row per template.
func renderAlertRuleEngineSections(builder *strings.Builder, templates []alertRuleTemplate) {
	if groups, ok := groupAlertRules(templates); ok {
		renderGroupedEngineTable(builder, groups)
		return
	}
	renderPerTemplateEngineTable(builder, templates)
}

func groupAlertRules(templates []alertRuleTemplate) ([]alertRuleGroup, bool) {
	order := make([]string, 0)
	byName := make(map[string]*alertRuleGroup)
	for _, template := range templates {
		name := template.displayName()
		group, seen := byName[name]
		if !seen {
			group = &alertRuleGroup{name: name}
			byName[name] = group
			order = append(order, name)
		}
		description := normalizeAlertRuleText(template.displayDescription())
		if group.description != "" && description != "" && group.description != description {
			return nil, false
		}
		if group.description == "" {
			group.description = description
		}
		if template.isV2() {
			if group.v2 {
				return nil, false
			}
			group.v2 = true
			continue
		}
		if group.v1 {
			return nil, false
		}
		group.v1 = true
	}
	groups := make([]alertRuleGroup, 0, len(order))
	for _, name := range order {
		groups = append(groups, *byName[name])
	}
	return groups, true
}

func renderGroupedEngineTable(builder *strings.Builder, groups []alertRuleGroup) {
	showDescription := false
	for _, group := range groups {
		if group.description != "" {
			showDescription = true
			break
		}
	}
	writeAlertRuleTableOpen(builder)
	if showDescription {
		builder.WriteString("| Name | Description | ES\\|QL | Standard |\n")
		builder.WriteString("|---|---|:---:|:---:|\n")
	} else {
		builder.WriteString("| Name | ES\\|QL | Standard |\n")
		builder.WriteString("|---|:---:|:---:|\n")
	}
	for _, group := range groups {
		name := escapeAlertRuleName(group.name)
		v2, v1 := "", ""
		if group.v2 {
			v2 = "✓"
		}
		if group.v1 {
			v1 = "✓"
		}
		if showDescription {
			fmt.Fprintf(builder, "| %s | %s | %s | %s |\n", name, escapeAlertRuleTableCell(group.description), v2, v1)
			continue
		}
		fmt.Fprintf(builder, "| %s | %s | %s |\n", name, v2, v1)
	}
	writeAlertRuleTableClose(builder)
}

func renderPerTemplateEngineTable(builder *strings.Builder, templates []alertRuleTemplate) {
	showDescription := hasAlertRuleDescriptions(templates)
	writeAlertRuleTableOpen(builder)
	if showDescription {
		builder.WriteString("| Name | Description | Engine |\n")
		builder.WriteString("|---|---|---|\n")
	} else {
		builder.WriteString("| Name | Engine |\n")
		builder.WriteString("|---|---|\n")
	}
	for _, wantV2 := range []bool{true, false} {
		for _, template := range templates {
			if template.isV2() != wantV2 {
				continue
			}
			name := escapeAlertRuleName(template.displayName())
			engine := escapeAlertRuleTableCell(engineName(template))
			if showDescription {
				fmt.Fprintf(builder, "| %s | %s | %s |\n", name, escapeAlertRuleTableCell(template.displayDescription()), engine)
				continue
			}
			fmt.Fprintf(builder, "| %s | %s |\n", name, engine)
		}
	}
	writeAlertRuleTableClose(builder)
}

func writeAlertRuleTableOpen(builder *strings.Builder) {
	builder.WriteString("**The following alert rule templates are available:**\n\n")
	builder.WriteString("<details>\n")
	builder.WriteString("<summary>View the alert rule templates</summary>\n\n")
}

func writeAlertRuleTableClose(builder *strings.Builder) {
	builder.WriteString("\n</details>\n\n")
}

func engineName(template alertRuleTemplate) string {
	if template.isV2() {
		return "ES|QL"
	}
	return "Standard"
}

func escapeAlertRuleTableCell(value string) string {
	value = normalizeAlertRuleText(value)
	// Escape existing backslashes first so a backslash before a pipe cannot
	// cancel the pipe escape and split the table cell.
	value = strings.ReplaceAll(value, "\\", "\\\\")
	return strings.ReplaceAll(escaper.Replace(value), "|", "\\|")
}

func escapeAlertRuleName(value string) string {
	// Names are labels, while descriptions may contain inline Markdown links.
	return alertRuleNameEscaper.Replace(normalizeAlertRuleText(value))
}

func normalizeAlertRuleText(value string) string {
	return strings.TrimSpace(alertRuleLineBreaks.Replace(value))
}

func renderAlertRuleCollapsibleTable(builder *strings.Builder, templates []alertRuleTemplate) {
	writeAlertRuleTableOpen(builder)
	builder.WriteString("| Name | Description |\n")
	builder.WriteString("|---|---|\n")
	for _, t := range templates {
		// Single-engine tables keep the historic escaping so existing package
		// READMEs stay byte-identical: escape only the Markdown emphasis and
		// angle-bracket characters, and leave labels like "[AWS EC2 OTel] ..."
		// and literal pipes untouched. The dual-engine tables below opt into the
		// stricter cell escaping, which no published README depends on yet.
		fmt.Fprintf(builder, "| %s | %s |\n",
			escaper.Replace(normalizeAlertRuleText(t.displayName())),
			escaper.Replace(normalizeAlertRuleText(t.displayDescription())))
	}
	writeAlertRuleTableClose(builder)
}

func hasAlertRuleDescriptions(templates []alertRuleTemplate) bool {
	for _, template := range templates {
		if normalizeAlertRuleText(template.displayDescription()) != "" {
			return true
		}
	}
	return false
}
