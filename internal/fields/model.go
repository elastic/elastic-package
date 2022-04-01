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
	Index       *bool             `yaml:"index"`
	DocValues   *bool             `yaml:"doc_values"`
	Fields      []FieldDefinition `yaml:"fields,omitempty"`
	MultiFields []FieldDefinition `yaml:"multi_fields,omitempty"`
}

func (orig *FieldDefinition) Update(fd FieldDefinition) {
	if fd.Name != "" {
		orig.Name = fd.Name
	}
	if fd.Description != "" {
		orig.Description = fd.Description
	}
	if fd.Type != "" {
		orig.Type = fd.Type
	}
	if fd.Value != "" {
		orig.Value = fd.Value
	}
	if fd.Pattern != "" {
		orig.Pattern = fd.Pattern
	}
	if fd.Unit != "" {
		orig.Unit = fd.Unit
	}
	if fd.MetricType != "" {
		orig.MetricType = fd.MetricType
	}
	if fd.External != "" {
		orig.External = fd.External
	}
	if fd.Index != nil {
		orig.Index = fd.Index
	}
	if fd.DocValues != nil {
		orig.DocValues = fd.DocValues
	}

	if len(fd.Fields) > 0 {
		orig.Fields = updateFields(orig.Fields, fd.Fields)
	}

	if len(fd.MultiFields) > 0 {
		orig.MultiFields = updateFields(orig.MultiFields, fd.MultiFields)
	}
}

func updateFields(origFields, fields []FieldDefinition) []FieldDefinition {
	// When a subfield the same name exists, update it. When not, append it.
	updatedFields := make([]FieldDefinition, len(origFields))
	copy(updatedFields, origFields)
	for _, newField := range fields {
		found := false
		for i, origField := range origFields {
			if origField.Name != newField.Name {
				continue
			}

			found = true
			updatedFields[i].Update(newField)
			break
		}
		if !found {
			updatedFields = append(updatedFields, newField)
		}
	}
	return updatedFields
}
