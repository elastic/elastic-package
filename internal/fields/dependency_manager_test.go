// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package fields

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/elastic-package/internal/common"
)

func TestDependencyManagerInjectExternalFields(t *testing.T) {
	cases := []struct {
		title   string
		defs    []common.MapStr
		result  []common.MapStr
		changed bool
		valid   bool
	}{
		{
			title:   "empty defs",
			defs:    []common.MapStr{},
			changed: false,
			valid:   true,
		},
		{
			title: "dataset value override",
			defs: []common.MapStr{
				{
					"name":     "data_stream.dataset",
					"external": "test",
					"value":    "nginx.access",
				},
				{
					"name":     "data_stream.type",
					"external": "test",
					"value":    "logs",
				},
			},
			result: []common.MapStr{
				{
					"name":        "data_stream.dataset",
					"type":        "constant_keyword",
					"description": "Data stream dataset.",
					"value":       "nginx.access",
				},
				{
					"name":        "data_stream.type",
					"type":        "constant_keyword",
					"description": "Data stream type (logs, metrics).",
					"value":       "logs",
				},
			},
			changed: true,
			valid:   true,
		},
		{
			title: "external dimension",
			defs: []common.MapStr{
				{
					"name":      "container.id",
					"external":  "test",
					"dimension": "true",
				},
			},
			result: []common.MapStr{
				{
					"name":        "container.id",
					"type":        "keyword",
					"description": "Container identifier.",
					"dimension":   "true",
				},
			},
			changed: true,
			valid:   true,
		},
		{
			title: "external dimension",
			defs: []common.MapStr{
				{
					"name":      "container.id",
					"external":  "test",
					"dimension": "true",
				},
			},
			result: []common.MapStr{
				{
					"name":        "container.id",
					"type":        "keyword",
					"description": "Container identifier.",
					"dimension":   "true",
				},
			},
			changed: true,
			valid:   true,
		},
		{
			title: "type override",
			defs: []common.MapStr{
				{
					"name":     "container.id",
					"external": "test",
					"type":     "long",
				},
			},
			result: []common.MapStr{
				{
					"name":        "container.id",
					"type":        "keyword",
					"description": "Container identifier.",
				},
			},
			changed: true,
			valid:   true,
		},
		{
			title: "unknown field",
			defs: []common.MapStr{
				{
					"name":     "container.identifier",
					"external": "test",
				},
			},
			valid: false,
		},
	}

	schema := map[string][]FieldDefinition{"test": []FieldDefinition{
		{
			Name:        "container.id",
			Description: "Container identifier.",
			Type:        "keyword",
		},
		{
			Name:        "data_stream.type",
			Description: "Data stream type (logs, metrics).",
			Type:        "constant_keyword",
		},
		{
			Name:        "data_stream.dataset",
			Description: "Data stream dataset.",
			Type:        "constant_keyword",
		},
	}}
	dm := &DependencyManager{schema: schema}

	for _, c := range cases {
		t.Run(c.title, func(t *testing.T) {
			result, changed, err := dm.InjectFields(c.defs)
			if !c.valid {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, c.changed, changed)
			assert.EqualValues(t, c.result, result)
		})
	}
}
