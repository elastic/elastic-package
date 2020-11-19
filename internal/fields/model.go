// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package fields

// FieldDefinition describes a single field with its properties.
type FieldDefinition struct {
	Name        string            `yaml:"name"`
	Description string            `yaml:"description"`
	Type        string            `yaml:"type"`
	Pattern     string            `yaml:"pattern"`
	Fields      []FieldDefinition `yaml:"fields"`
}
