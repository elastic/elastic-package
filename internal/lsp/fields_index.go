// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package lsp

import (
	"os"
	"path/filepath"
	"strings"

	yamlv3 "gopkg.in/yaml.v3"
)

// FieldInfo holds metadata about a single field from fields/*.yml.
type FieldInfo struct {
	Name        string      `yaml:"name"`
	Type        string      `yaml:"type"`
	Description string      `yaml:"description"`
	Unit        string      `yaml:"unit,omitempty"`
	MetricType  string      `yaml:"metric_type,omitempty"`
	External    string      `yaml:"external,omitempty"`
	Fields      []FieldInfo `yaml:"fields,omitempty"`
}

// FieldIndex is a flat map of dotted field names to their info.
type FieldIndex map[string]FieldInfo

// BuildFieldIndex scans all fields/*.yml files under a package root and returns
// a flat index of dotted field names.
func BuildFieldIndex(packageRoot string) FieldIndex {
	idx := make(FieldIndex)

	// Scan package-level fields.
	collectFieldFiles(filepath.Join(packageRoot, "fields"), idx, "")

	// Scan each data stream's fields.
	dsDir := filepath.Join(packageRoot, "data_stream")
	entries, err := os.ReadDir(dsDir)
	if err != nil {
		return idx
	}
	for _, e := range entries {
		if e.IsDir() {
			collectFieldFiles(filepath.Join(dsDir, e.Name(), "fields"), idx, "")
		}
	}

	return idx
}

// BuildFieldIndexForDataStream builds a field index for a specific data stream.
func BuildFieldIndexForDataStream(packageRoot, dataStream string) FieldIndex {
	idx := make(FieldIndex)
	collectFieldFiles(filepath.Join(packageRoot, "fields"), idx, "")
	collectFieldFiles(filepath.Join(packageRoot, "data_stream", dataStream, "fields"), idx, "")
	return idx
}

func collectFieldFiles(dir string, idx FieldIndex, prefix string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() || (!strings.HasSuffix(e.Name(), ".yml") && !strings.HasSuffix(e.Name(), ".yaml")) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var fields []FieldInfo
		if err := yamlv3.Unmarshal(data, &fields); err != nil {
			continue
		}
		flattenFields(fields, prefix, idx)
	}
}

func flattenFields(fields []FieldInfo, prefix string, idx FieldIndex) {
	for _, f := range fields {
		fullName := f.Name
		if prefix != "" {
			fullName = prefix + "." + f.Name
		}
		idx[fullName] = f
		if f.Type == "group" && len(f.Fields) > 0 {
			flattenFields(f.Fields, fullName, idx)
		}
	}
}
