// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package docs

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Masterminds/semver/v3"

	"github.com/elastic/elastic-package/internal/fields"
	"github.com/elastic/elastic-package/internal/packages"
)

var semver3_2_3 = semver.MustParse("3.3.1")

type fieldsTableRecord struct {
	name        string
	description string
	aType       string
	unit        string
	metricType  string
	value       string
	example	    string
}

var escaper = strings.NewReplacer("*", "\\*", "{", "\\{", "}", "\\}", "<", "\\<", ">", "\\>")

func renderExportedFields(fieldsParentDir string, pkgManifest *packages.PackageManifest) (string, error) {
	injectOptions := fields.InjectFieldsOptions{
		// Keep External parameter when rendering fields, so we can render
		// documentation for empty groups imported from ECS, for backwards compatibility.
		KeepExternal: true,

		// SkipEmptyFields parameter when rendering fields. In other cases we want to
		// keep them to accept them for validation.
		SkipEmptyFields: true,
	}
	validator, err := fields.CreateValidatorForDirectory(fieldsParentDir, fields.WithInjectFieldsOptions(injectOptions))
	if err != nil {
		return "", fmt.Errorf("can't create fields validator instance (path: %s): %w", fieldsParentDir, err)
	}

	collected := collectFieldsFromDefinitions(validator)

	var builder strings.Builder
	builder.WriteString("**Exported fields**\n\n")

	if len(collected) == 0 {
		builder.WriteString("(no fields available)\n")
		return builder.String(), nil
	}

	sv, err := semver.NewVersion(pkgManifest.SpecVersion)
	if err != nil {
		return "", fmt.Errorf("failed to obtain spec version from package manifest: %w", err)
	}

	renderExamples := true
	renderValues := true
	if sv.LessThan(semver3_2_3) {
		renderExamples = false
		renderValues = false
	}

	renderFieldsTable(&builder, collected, renderValues, renderExamples)

	return builder.String(), nil
}

func renderFieldsTable(builder *strings.Builder, collected []fieldsTableRecord, includeValues bool, includeExamples bool) {
	unitsPresent := areUnitsPresent(collected)
	metricTypesPresent := areMetricTypesPresent(collected)
	valuesPresent := areValuesPresent(collected)
	examplePresent := areExamplesPresent(collected)

	builder.WriteString("| Field | Description | Type |")
	if unitsPresent {
		builder.WriteString(" Unit |")
	}
	if metricTypesPresent {
		builder.WriteString(" Metric Type |")
	}
	if valuesPresent && includeValues {
		builder.WriteString(" Value |")
	}
	if examplePresent && includeExamples {
		builder.WriteString(" Example |")
	}

	builder.WriteString("\n")
	builder.WriteString("|---|---|---|")
	if unitsPresent {
		builder.WriteString("---|")
	}
	if metricTypesPresent {
		builder.WriteString("---|")
	}
	if valuesPresent && includeValues {
		builder.WriteString("---|")
	}
	if examplePresent && includeExamples {
		builder.WriteString("---|")
	}

	builder.WriteString("\n")
	for _, c := range collected {
		description := strings.TrimSpace(strings.ReplaceAll(c.description, "\n", " "))
		builder.WriteString(fmt.Sprintf("| %s | %s | %s |",
			escaper.Replace(c.name),
			escaper.Replace(description),
			c.aType))
		if unitsPresent {
			builder.WriteString(fmt.Sprintf(" %s |", c.unit))
		}
		if metricTypesPresent {
			builder.WriteString(fmt.Sprintf(" %s |", c.metricType))
		}
		if valuesPresent && includeValues {
			builder.WriteString(fmt.Sprintf(" %s |", c.value))
		}
		if examplePresent && includeExamples {
			builder.WriteString(fmt.Sprintf(" %s |", c.example))
		}
		builder.WriteString("\n")
	}
}

func areUnitsPresent(collected []fieldsTableRecord) bool {
	for _, c := range collected {
		if c.unit != "" {
			return true
		}
	}
	return false
}

func areMetricTypesPresent(collected []fieldsTableRecord) bool {
	for _, c := range collected {
		if c.metricType != "" {
			return true
		}
	}
	return false
}

func areValuesPresent(collected []fieldsTableRecord) bool {
	for _, c := range collected {
		if c.value != "" {
			return true
		}
	}
	return false
}

func areExamplesPresent(collected []fieldsTableRecord) bool {
	for _, c := range collected {
		if c.example != "" {
			return true
		}
	}
	return false
}

func collectFieldsFromDefinitions(validator *fields.Validator) []fieldsTableRecord {
	var records []fieldsTableRecord

	root := validator.Schema
	for _, f := range root {
		records = visitFields("", f, records)
	}
	return uniqueTableRecords(records)
}

func visitFields(namePrefix string, f fields.FieldDefinition, records []fieldsTableRecord) []fieldsTableRecord {
	var name = namePrefix
	if namePrefix != "" {
		name += "."
	}
	name += f.Name

	if (len(f.Fields) == 0 && f.Type != "group") || f.External != "" {
		records = append(records, fieldsTableRecord{
			name:        name,
			description: f.Description,
			aType:       f.Type,
			unit:        f.Unit,
			metricType:  f.MetricType,
			value:       f.Value,
			example:     f.Example,
		})

		for _, multiField := range f.MultiFields {
			records = append(records, fieldsTableRecord{
				name:        name + "." + multiField.Name,
				description: fmt.Sprintf("Multi-field of %#q.", name),
				aType:       multiField.Type,
			})
		}

		return records
	}

	for _, fieldEntry := range f.Fields {
		records = visitFields(name, fieldEntry, records)
	}
	return records
}

func uniqueTableRecords(records []fieldsTableRecord) []fieldsTableRecord {
	sort.Slice(records, func(i, j int) bool {
		return sort.StringsAreSorted([]string{records[i].name, records[j].name})
	})

	fieldNames := make(map[string]bool)
	var unique []fieldsTableRecord
	for _, r := range records {
		if _, ok := fieldNames[r.name]; !ok {
			fieldNames[r.name] = true
			unique = append(unique, r)
		}
	}
	return unique
}
