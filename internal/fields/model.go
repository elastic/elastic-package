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
	Name          string            `yaml:"name"`
	Description   string            `yaml:"description"`
	Type          string            `yaml:"type"`
	Value         string            `yaml:"value"` // The value to associate with a constant_keyword field.
	AllowedValues AllowedValues     `yaml:"allowed_values"`
	Pattern       string            `yaml:"pattern"`
	Unit          string            `yaml:"unit"`
	MetricType    string            `yaml:"metric_type"`
	External      string            `yaml:"external"`
	Index         *bool             `yaml:"index"`
	DocValues     *bool             `yaml:"doc_values"`
	Fields        FieldDefinitions  `yaml:"fields,omitempty"`
	MultiFields   []FieldDefinition `yaml:"multi_fields,omitempty"`
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
	if len(fd.AllowedValues) > 0 {
		orig.AllowedValues = fd.AllowedValues
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

			// "base" group is used by convention in ECS to include
			// fields that can appear in the root level of the document.
			// Append its child fields directly instead.
			if name == "base" {
				fields = append(fields, field.Fields...)
			} else {
				field.Name = name
				cleanNestedNames(field.Name, field.Fields)
				fields = append(fields, field)
			}
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

func cleanNestedNames(parent string, fields []FieldDefinition) {
	for i := range fields {
		if strings.HasPrefix(fields[i].Name, parent+".") {
			fields[i].Name = fields[i].Name[len(parent)+1:]
		}
	}
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
