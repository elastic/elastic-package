// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package fields

// FieldDefinition describes a single field with its properties.
type FieldDefinition struct {
	Name        string                 `yaml:"name"`
	Description string                 `yaml:"description"`
	Type        string                 `yaml:"type"`
	Value       string                 `yaml:"value"` // The value to associate with a constant_keyword field.
	Pattern     string                 `yaml:"pattern"`
	Unit        string                 `yaml:"unit"`
	MetricType  string                 `yaml:"metric_type"`
	External    string                 `yaml:"external"`
	Index       *bool                  `yaml:"index"`
	DocValues   *bool                  `yaml:"doc_values"`
	Fields      []FieldDefinition      `yaml:"fields,omitempty"`
	MultiFields []MultiFieldDefinition `yaml:"multi_fields,omitempty"`
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
		orig.updateFields(fd.Fields)
	}

	if len(fd.MultiFields) > 0 {
		orig.updateMultiFields(fd.MultiFields)
	}
}

func (orig *FieldDefinition) updateFields(fields []FieldDefinition) {
	// When a subfield the same name exists, update it. When not, append it.
	updatedFields := make([]FieldDefinition, len(orig.Fields))
	copy(updatedFields, orig.Fields)
	for _, newField := range fields {
		found := false
		for i, origField := range orig.Fields {
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
	orig.Fields = updatedFields
}

func (orig *FieldDefinition) updateMultiFields(fields []MultiFieldDefinition) {
	// When a subfield the same name exists, update it. When not, append it.
	updatedFields := make([]MultiFieldDefinition, len(orig.MultiFields))
	copy(updatedFields, orig.MultiFields)
	for _, newField := range fields {
		found := false
		for i, origField := range orig.MultiFields {
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
	orig.MultiFields = updatedFields
}

// MultiFieldDefinition describes a multi field with its properties.
type MultiFieldDefinition struct {
	Name string `yaml:"name"`
	Type string `yaml:"type"`
}

func (orig *MultiFieldDefinition) Update(fd MultiFieldDefinition) {
	if fd.Name != "" {
		orig.Name = fd.Name
	}
	if fd.Type != "" {
		orig.Type = fd.Type
	}
}
