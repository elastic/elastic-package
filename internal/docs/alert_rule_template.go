// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package docs

import (
	"bytes"
	"encoding/json"
	"errors"
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
// collapsible table. Earlier specs keep the paragraph layout. A package that
// publishes both engines always uses one collapsible table with a column per engine.
var alertRuleTemplatesCollapsibleTableMinSpecVersion = semver.MustParse("3.6.0")

var alertRuleNameEscaper = strings.NewReplacer(
	"\\", "\\\\", "*", "\\*", "_", "\\_", "`", "\\`", "~", "\\~",
	"[", "\\[", "]", "\\]", "{", "\\{", "}", "\\}", "<", "\\<", ">", "\\>",
	"|", "\\|", "&", "&amp;",
)

var alertRuleLineBreaks = strings.NewReplacer("\r\n", " ", "\r", " ", "\n", " ")

// alertRuleTemplate is the part of an alerting_rule_template asset shown in the README.
type alertRuleTemplate struct {
	name        string
	description string
	v2          bool
}

// parseAlertRuleTemplate reads a template's name and description. Alerting v2
// templates keep them under rule.metadata, classic templates on attributes.
func parseAlertRuleTemplate(raw []byte) (alertRuleTemplate, error) {
	var doc struct {
		Attributes struct {
			Name        string          `json:"name"`
			Description string          `json:"description"`
			Engine      any             `json:"engine"`
			Rule        json.RawMessage `json:"rule"`
		} `json:"attributes"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return alertRuleTemplate{}, err
	}
	// Only the string "v2" selects Alerting v2. A missing, null, or non-string
	// engine is classic, as in the package-spec SVR00014 validator.
	if doc.Attributes.Engine != "v2" {
		return alertRuleTemplate{name: doc.Attributes.Name, description: doc.Attributes.Description}, nil
	}
	var rule struct {
		Metadata struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		} `json:"metadata"`
	}
	if !bytes.HasPrefix(bytes.TrimSpace(doc.Attributes.Rule), []byte("{")) {
		return alertRuleTemplate{}, errors.New("Alerting v2 template attributes.rule must be an object") //nolint:staticcheck // capitalized product name is intentional
	}
	if err := json.Unmarshal(doc.Attributes.Rule, &rule); err != nil {
		return alertRuleTemplate{}, fmt.Errorf("invalid Alerting v2 rule: %w", err)
	}
	return alertRuleTemplate{name: rule.Metadata.Name, description: rule.Metadata.Description, v2: true}, nil
}

// alertRuleTemplateCompatibility states the Stack floor for the engines present.
// Standard (classic) templates require 9.2.0. ES|QL (Alerting v2) requires
// 9.6.0. A package that publishes both says so separately, so the classic floor
// is not raised silently and the v2 floor is not omitted.
func alertRuleTemplateCompatibility(hasClassic, hasV2 bool) string {
	switch {
	case hasClassic && hasV2:
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

		template, err := parseAlertRuleTemplate(rawTemplate)
		if err != nil {
			return fmt.Errorf("failed to parse alert rule template %s: %w", path, err)
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

		hasClassic, hasV2 := false, false
		for _, template := range templates {
			if template.v2 {
				hasV2 = true
			} else {
				hasClassic = true
			}
		}

		builder.WriteString(`Alert rule templates provide pre-defined configurations for creating alert rules in Kibana.

For more information, refer to the [Elastic documentation](` + docsLink + `).

`)
		builder.WriteString(alertRuleTemplateCompatibility(hasClassic, hasV2))
		builder.WriteString("\n")

		switch {
		case hasClassic && hasV2:
			renderAlertRuleEngineTable(&builder, groupAlertRules(templates))
		case useCollapsibleTable:
			renderAlertRuleTable(&builder, templates, hasV2)
		default:
			renderAlertRuleList(&builder, templates, hasV2)
		}
	}

	return builder.String(), nil
}

// renderAlertRuleList renders the paragraph layout of a single-engine package.
// Classic text is written as is, so existing package READMEs do not change.
func renderAlertRuleList(builder *strings.Builder, templates []alertRuleTemplate, v2 bool) {
	name, description := keepAlertRuleText, keepAlertRuleText
	if v2 {
		name, description = escapeAlertRuleName, escapeAlertRuleDescription
	}
	builder.WriteString("The following alert rule templates are available:\n\n")
	for _, template := range templates {
		fmt.Fprintf(builder, "**%s**\n\n", name(template.name))
		fmt.Fprintf(builder, "%s\n\n", description(template.description))
	}
}

// renderAlertRuleTable renders the Name | Description table of a single-engine
// package. Classic cells keep the escaping of earlier releases, so existing
// package READMEs do not change.
func renderAlertRuleTable(builder *strings.Builder, templates []alertRuleTemplate, v2 bool) {
	name, description := classicAlertRuleTableName, classicAlertRuleTableDescription
	if v2 {
		name, description = escapeAlertRuleName, escapeAlertRuleTableCell
	}
	writeAlertRuleTableOpen(builder)
	builder.WriteString("| Name | Description |\n")
	builder.WriteString("|---|---|\n")
	for _, template := range templates {
		fmt.Fprintf(builder, "| %s | %s |\n", name(template.name), description(template.description))
	}
	writeAlertRuleTableClose(builder)
}

// alertRuleGroup is one table row: an alert name published on one or both engines.
type alertRuleGroup struct {
	name        string
	description string
	v2          bool
	classic     bool
}

func groupAlertRules(templates []alertRuleTemplate) []alertRuleGroup {
	order := make([]string, 0)
	byName := make(map[string][]alertRuleTemplate)
	for _, template := range templates {
		// Key on the normalized name so templates that differ only in trailing
		// whitespace or line breaks still pair into one row.
		name := normalizeAlertRuleText(template.name)
		if _, seen := byName[name]; !seen {
			order = append(order, name)
		}
		byName[name] = append(byName[name], template)
	}
	var groups []alertRuleGroup
	for _, name := range order {
		groups = append(groups, groupTemplatesByName(name, byName[name])...)
	}
	return groups
}

// groupTemplatesByName merges templates that share a normalized name into one
// row when they fit: at most one per engine and no conflicting descriptions.
// Otherwise it returns one row per template, ES|QL rows first to match Fleet's
// ordering.
func groupTemplatesByName(name string, templates []alertRuleTemplate) []alertRuleGroup {
	merged := alertRuleGroup{name: name}
	mergeable := true
	for _, template := range templates {
		description := normalizeAlertRuleText(template.description)
		if merged.description != "" && description != "" && merged.description != description {
			mergeable = false
			break
		}
		if description != "" {
			merged.description = description
		}
		if template.v2 {
			if merged.v2 {
				mergeable = false
				break
			}
			merged.v2 = true
			continue
		}
		if merged.classic {
			mergeable = false
			break
		}
		merged.classic = true
	}
	if mergeable {
		return []alertRuleGroup{merged}
	}
	groups := make([]alertRuleGroup, 0, len(templates))
	for _, wantV2 := range []bool{true, false} {
		for _, template := range templates {
			if template.v2 != wantV2 {
				continue
			}
			groups = append(groups, alertRuleGroup{
				name:        name,
				description: normalizeAlertRuleText(template.description),
				v2:          template.v2,
				classic:     !template.v2,
			})
		}
	}
	return groups
}

// renderAlertRuleEngineTable renders a package that publishes both engines as
// one table with the ES|QL and Standard columns Fleet shows on the Alerting tab.
// The Description column is left out when every description is blank.
func renderAlertRuleEngineTable(builder *strings.Builder, groups []alertRuleGroup) {
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
		v2, classic := "", ""
		if group.v2 {
			v2 = "✅"
		}
		if group.classic {
			classic = "✅"
		}
		if showDescription {
			fmt.Fprintf(builder, "| %s | %s | %s | %s |\n", name, escapeAlertRuleTableCell(group.description), v2, classic)
			continue
		}
		fmt.Fprintf(builder, "| %s | %s | %s |\n", name, v2, classic)
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

func keepAlertRuleText(value string) string {
	return value
}

func classicAlertRuleTableName(value string) string {
	return escaper.Replace(strings.TrimSpace(value))
}

func classicAlertRuleTableDescription(value string) string {
	return escaper.Replace(strings.TrimSpace(strings.ReplaceAll(value, "\n", " ")))
}

func escapeAlertRuleTableCell(value string) string {
	value = normalizeAlertRuleText(value)
	// Escape existing backslashes first so a backslash before a pipe cannot
	// cancel the pipe escape and split the table cell.
	value = strings.ReplaceAll(value, "\\", "\\\\")
	return strings.ReplaceAll(escaper.Replace(value), "|", "\\|")
}

func escapeAlertRuleDescription(value string) string {
	return escaper.Replace(normalizeAlertRuleText(value))
}

func escapeAlertRuleName(value string) string {
	// Names are labels, while descriptions may contain inline Markdown links.
	return alertRuleNameEscaper.Replace(normalizeAlertRuleText(value))
}

func normalizeAlertRuleText(value string) string {
	return strings.TrimSpace(alertRuleLineBreaks.Replace(value))
}
