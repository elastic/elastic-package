// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package docs

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/fields"
)

type fieldsTableRecord struct {
	name        string
	description string
	aType       string
}

func renderExportedFields(packageRoot, dataStreamName string) (string, error) {
	dataStreamPath := filepath.Join(packageRoot, "data_stream", dataStreamName)
	definitions, err := fields.LoadFieldsForDataStream(dataStreamPath)
	if err != nil {
		return "", errors.Wrapf(err, "can't load fields for data stream (path: %s)", dataStreamPath)
	}

	collected, err := collectFieldsFromDefinitions(definitions)
	if err != nil {
		return "", errors.Wrap(err, "collecting fields files failed")
	}

	var builder strings.Builder
	builder.WriteString("**Exported fields**\n\n")

	if len(collected) == 0 {
		builder.WriteString("(no fields available)\n")
		return builder.String(), nil
	}
	builder.WriteString("| Field | Description | Type |\n")
	builder.WriteString("|---|---|---|\n")
	for _, c := range collected {
		description := strings.TrimSpace(strings.ReplaceAll(c.description, "\n", " "))
		builder.WriteString(fmt.Sprintf("| %s | %s | %s |\n", c.name, description, c.aType))
	}
	return builder.String(), nil
}

func collectFieldsFromDefinitions(fieldDefinitions []fields.FieldDefinition) ([]fieldsTableRecord, error) {
	var records []fieldsTableRecord

	root := fieldDefinitions
	var err error
	for _, f := range root {
		records, err = visitFields("", f, records)
		if err != nil {
			return nil, errors.Wrapf(err, "visiting fields failed")
		}
	}
	return uniqueTableRecords(records), nil
}

func visitFields(namePrefix string, f fields.FieldDefinition, records []fieldsTableRecord) ([]fieldsTableRecord, error) {
	var name = namePrefix
	if namePrefix != "" {
		name += "."
	}
	name += f.Name

	if len(f.Fields) == 0 && f.Type != "group" {
		records = append(records, fieldsTableRecord{
			name:        name,
			description: f.Description,
			aType:       f.Type,
		})
		return records, nil
	}

	var err error
	for _, fieldEntry := range f.Fields {
		records, err = visitFields(name, fieldEntry, records)
		if err != nil {
			return nil, errors.Wrapf(err, "recursive visiting fields failed (namePrefix: %s)", namePrefix)
		}
	}
	return records, nil
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
