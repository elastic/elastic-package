// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package fields

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFieldDefinitionUpdate(t *testing.T) {
	cases := []struct {
		title    string
		original FieldDefinition
		updated  FieldDefinition
		result   FieldDefinition
	}{
		{
			"empty", FieldDefinition{}, FieldDefinition{}, FieldDefinition{},
		},
		{
			"external field update",
			FieldDefinition{
				Name:     "container.id",
				External: "ecs",
			},
			FieldDefinition{
				Name:        "container.id",
				Description: "A container id.",
				Type:        "keyword",
			},
			FieldDefinition{
				Name:        "container.id",
				Description: "A container id.",
				External:    "ecs",
				Type:        "keyword",
			},
		},
		{
			"field with subfields",
			FieldDefinition{
				Name: "data_stream",
				Fields: []FieldDefinition{
					{
						Name:  "type",
						Value: "logs",
					},
					{
						Name:  "dataset",
						Value: "nginx.access",
					},
				},
			},
			FieldDefinition{
				Name: "data_stream",
				Fields: []FieldDefinition{
					{
						Name: "dataset",
						Type: "constant_keyword",
					},
					{
						Name: "type",
						Type: "constant_keyword",
					},
				},
			},
			FieldDefinition{
				Name: "data_stream",
				Fields: []FieldDefinition{
					{
						Name:  "type",
						Value: "logs",
						Type:  "constant_keyword",
					},
					{
						Name:  "dataset",
						Value: "nginx.access",
						Type:  "constant_keyword",
					},
				},
			},
		},
		{
			"field with more subfields",
			FieldDefinition{
				Name: "data_stream",
				Fields: []FieldDefinition{
					{
						Name:  "type",
						Value: "logs",
					},
					{
						Name:  "dataset",
						Value: "nginx.access",
					},
				},
			},
			FieldDefinition{
				Name: "data_stream",
				Type: "group",
				Fields: []FieldDefinition{
					{
						Name: "dataset",
						Type: "constant_keyword",
					},
				},
			},
			FieldDefinition{
				Name: "data_stream",
				Type: "group",
				Fields: []FieldDefinition{
					{
						Name:  "type",
						Value: "logs",
					},
					{
						Name:  "dataset",
						Value: "nginx.access",
						Type:  "constant_keyword",
					},
				},
			},
		},
		{
			"field with less subfields",
			FieldDefinition{
				Name: "data_stream",
				Fields: []FieldDefinition{
					{
						Name:  "dataset",
						Value: "nginx.access",
					},
				},
			},
			FieldDefinition{
				Name: "data_stream",
				Fields: []FieldDefinition{
					{
						Name: "type",
						Type: "constant_keyword",
					},
					{
						Name: "dataset",
						Type: "constant_keyword",
					},
				},
			},
			FieldDefinition{
				Name: "data_stream",
				Fields: []FieldDefinition{
					{
						Name:  "dataset",
						Type:  "constant_keyword",
						Value: "nginx.access",
					},
					{
						Name: "type",
						Type: "constant_keyword",
					},
				},
			},
		},
	}

	for _, c := range cases {
		t.Run(c.title, func(t *testing.T) {
			def := c.original
			def.Update(c.updated)
			assert.EqualValues(t, c.result, def)
		})
	}
}
