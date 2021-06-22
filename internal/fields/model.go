// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package fields

// FieldDefinition describes a single field with its properties.
type FieldDefinition struct {
	Name        string            `yaml:"name"`
	Description string            `yaml:"description"`
	Type        string            `yaml:"type"`
	Value       string            `yaml:"value"` // The value to associate with a constant_keyword field.
	Pattern     string            `yaml:"pattern"`
	Unit        string            `yaml:"unit"`
	MetricType  string            `yaml:"metric_type"`
	External    string            `yaml:"external"`
	Fields      []FieldDefinition `yaml:"fields"`
}
