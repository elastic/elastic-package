// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package fields

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/elastic/elastic-package/internal/common"
)

// FieldDefinition describes a single field with its properties.
type FieldDefinition struct {
	Name           string            `yaml:"name"`
	Description    string            `yaml:"description"`
	Type           string            `yaml:"type"`
	ObjectType     string            `yaml:"object_type"`
	Value          string            `yaml:"value"` // The value to associate with a constant_keyword field.
	AllowedValues  AllowedValues     `yaml:"allowed_values"`
	ExpectedValues []string          `yaml:"expected_values"`
	Pattern        string            `yaml:"pattern"`
	Unit           string            `yaml:"unit"`
	MetricType     string            `yaml:"metric_type"`
	External       string            `yaml:"external"`
	Index          *bool             `yaml:"index"`
	DocValues      *bool             `yaml:"doc_values"`
	Fields         FieldDefinitions  `yaml:"fields,omitempty"`
	MultiFields    []FieldDefinition `yaml:"multi_fields,omitempty"`
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
	if fd.ObjectType != "" {
		orig.ObjectType = fd.ObjectType
	}
	if fd.Value != "" {
		orig.Value = fd.Value
	}
	if len(fd.AllowedValues) > 0 {
		orig.AllowedValues = fd.AllowedValues
	}
	if len(fd.ExpectedValues) > 0 {
		orig.ExpectedValues = fd.ExpectedValues
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

// FieldDefinitions is an array of FieldDefinition, this can be unmarshalled from
// a yaml list or a yaml map.
type FieldDefinitions []FieldDefinition

func (fds *FieldDefinitions) UnmarshalYAML(value *yaml.Node) error {
	nilNode := yaml.Kind(0)
	switch value.Kind {
	case yaml.SequenceNode:
		// Fields are defined as a list, this happens in Beats fields files.
		var fields []FieldDefinition
		err := value.Decode(&fields)
		if err != nil {
			return err
		}
		*fds = fields
		return nil
	case yaml.MappingNode:
		// Fields are defined as a map, this happens in ecs fields files.
		if len(value.Content)%2 != 0 {
			return fmt.Errorf("pairs of key-values expected in map")
		}
		var fields []FieldDefinition
		for i := 0; i+1 < len(value.Content); i += 2 {
			key := value.Content[i]
			value := value.Content[i+1]

			var name string
			err := key.Decode(&name)
			if err != nil {
				return err
			}

			var field FieldDefinition
			err = value.Decode(&field)
			if err != nil {
				return err
			}

			field.Name = name
			baseFields := cleanNested(&field)
			if len(baseFields) > 0 {
				// Some groups are used by convention in ECS to include
				// fields that can appear in the root level of the document.
				// Append their child fields directly instead.
				// Examples of such groups are `base` or `tracing`.
				fields = append(fields, baseFields...)
				if len(field.Fields) == 0 {
					// If it had base fields, and doesn't have any other
					// field, don't add it.
					continue
				}
			}

			fields = append(fields, field)
		}
		*fds = fields
		return nil
	case nilNode:
		*fds = nil
		return nil
	default:
		return fmt.Errorf("expected map or sequence")
	}
}

// cleanNested processes fields nested inside another field, and returns
// defined base fields.
// If a field name is prefixed by the parent field, this part is removed,
// so the full path, taking into account the parent name, matches.
// If a field name is not prefixed by the parent field, this is considered
// a base field, that should appear at the top-level. It is removed from
// the list of nested fields and returned as base field.
func cleanNested(parent *FieldDefinition) (base []FieldDefinition) {
	var nested []FieldDefinition
	for _, field := range parent.Fields {
		// If the field name is prefixed by the name of its parent,
		// this is a normal nested field. If not, it is a base field.
		if strings.HasPrefix(field.Name, parent.Name+".") {
			field.Name = field.Name[len(parent.Name)+1:]
			nested = append(nested, field)
		} else {
			base = append(base, field)
		}
	}

	// At the moment of writing this code, a group field has base fields
	// (`base` and `tracing` groups), or nested fields, but not both.
	// This code handles the case of having groups with both kinds of fields,
	// just in case this happens.
	parent.Fields = nested
	return base
}

// AllowedValues is the list of allowed values for a field.
type AllowedValues []AllowedValue

// Allowed returns true if a given value is allowed.
func (avs AllowedValues) IsAllowed(value string) bool {
	if len(avs) == 0 {
		// No configured allowed values, any value is allowed.
		return true
	}
	return common.StringSliceContains(avs.Values(), value)
}

// Values returns the list of allowed values.
func (avs AllowedValues) Values() []string {
	var values []string
	for _, v := range avs {
		values = append(values, v.Name)
	}
	return values
}

// AllowedValue is one of the allowed values for a field.
type AllowedValue struct {
	Name               string   `yaml:"name"`
	Description        string   `yaml:"description"`
	ExpectedEventTypes []string `yaml:"expected_event_types"`
}
