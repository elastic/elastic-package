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
			title: "multi fields",
			defs: []common.MapStr{
				{
					"name":     "process.command_line",
					"external": "test",
				},
			},
			result: []common.MapStr{
				{
					"name":        "process.command_line",
					"type":        "wildcard",
					"description": "Full command line that started the process.",
					"multi_fields": []common.MapStr{
						{
							"name": "text",
							"type": "match_only_text",
						},
					},
				},
			},
			changed: true,
			valid:   true,
		},
		{
			title: "not indexed external",
			defs: []common.MapStr{
				{
					"name":     "event.original",
					"external": "test",
				},
			},
			result: []common.MapStr{
				{
					"name":        "event.original",
					"type":        "text",
					"description": "Original event.",
					"index":       false,
					"doc_values":  false,
				},
			},
			changed: true,
			valid:   true,
		},
		{
			title: "override not indexed external",
			defs: []common.MapStr{
				{
					"name":     "event.original",
					"index":    true,
					"external": "test",
				},
			},
			result: []common.MapStr{
				{
					"name":        "event.original",
					"type":        "text",
					"description": "Original event.",
					"index":       true,
					"doc_values":  false,
				},
			},
			changed: true,
			valid:   true,
		},
		{
			title: "array field",
			defs: []common.MapStr{
				{
					"name":     "host.ip",
					"external": "test",
				},
			},
			result: []common.MapStr{
				{
					"name":        "host.ip",
					"type":        "ip",
					"description": "Host ip addresses.",
					"normalize": []string{
						"array",
					},
				},
			},
			changed: true,
			valid:   true,
		},
		{
			title: "array field override",
			defs: []common.MapStr{
				{
					"name":     "container.id",
					"external": "test",
					"normalize": []string{
						"array",
					},
				},
			},
			result: []common.MapStr{
				{
					"name":        "container.id",
					"type":        "keyword",
					"description": "Container identifier.",
					"normalize": []string{
						"array",
					},
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

	indexFalse := false
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
		{
			Name:        "process.command_line",
			Description: "Full command line that started the process.",
			Type:        "wildcard",
			MultiFields: []FieldDefinition{
				{
					Name: "text",
					Type: "match_only_text",
				},
			},
		},
		{
			Name:        "event.original",
			Description: "Original event.",
			Type:        "text",
			Index:       &indexFalse,
			DocValues:   &indexFalse,
		},
		{
			Name:        "host.ip",
			Description: "Host ip addresses.",
			Type:        "ip",
			Normalize: []string{
				"array",
			},
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
