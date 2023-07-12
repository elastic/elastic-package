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
		options InjectFieldsOptions
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
			title: "keyword to constant_keyword override",
			defs: []common.MapStr{
				{
					"name":     "event.dataset",
					"type":     "constant_keyword",
					"external": "test",
					"value":    "nginx.access",
				},
			},
			result: []common.MapStr{
				{
					"name":        "event.dataset",
					"type":        "constant_keyword",
					"description": "Dataset that collected this event",
					"value":       "nginx.access",
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
			title: "external with pattern",
			defs: []common.MapStr{
				{
					"name":     "source.mac",
					"external": "test",
				},
			},
			result: []common.MapStr{
				{
					"name":        "source.mac",
					"type":        "keyword",
					"description": "MAC address of the source.",
					"pattern":     "^[A-F0-9]{2}(-[A-F0-9]{2}){5,}$",
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
		{
			title: "import nested fields",
			defs: []common.MapStr{
				{
					"name":     "host.id",
					"external": "test",
				},
				{
					"name":     "host.hostname",
					"external": "test",
				},
			},
			result: []common.MapStr{
				{
					"name":        "host.id",
					"description": "Unique host id",
					"type":        "keyword",
				},
				{
					"name":        "host.hostname",
					"description": "Hostname of the host",
					"type":        "keyword",
				},
			},
			valid:   true,
			changed: true,
		},
		{
			title: "import nested definitions",
			defs: []common.MapStr{
				{
					"name": "host",
					"type": "group",
					"fields": []interface{}{
						common.MapStr{
							"name":     "id",
							"external": "test",
						},
						common.MapStr{
							"name":     "hostname",
							"external": "test",
						},
					},
				},
			},
			result: []common.MapStr{
				{
					"name": "host",
					"type": "group",
					"fields": []common.MapStr{
						{
							"name":        "id",
							"description": "Unique host id",
							"type":        "keyword",
						},
						{
							"name":        "hostname",
							"description": "Hostname of the host",
							"type":        "keyword",
						},
					},
				},
			},
			valid:   true,
			changed: true,
		},
		{
			title: "keep group for docs but not for fields",
			defs: []common.MapStr{
				{
					"name":     "host",
					"external": "test",
				},
				{
					"name":     "host.hostname",
					"external": "test",
				},
			},
			options: InjectFieldsOptions{
				// Options used for fields injection for docs.
				SkipEmptyFields: true,
				KeepExternal:    true,
			},
			result: []common.MapStr{
				{
					"name":        "host",
					"description": "A general computing instance",
					"external":    "test",
					"type":        "group",
				},
				{
					"name":        "host.hostname",
					"description": "Hostname of the host",
					"external":    "test",
					"type":        "keyword",
				},
			},
			valid:   true,
			changed: true,
		},
		{
			title: "skip empty group for docs",
			defs: []common.MapStr{
				{
					"name": "host",
					"type": "group",
				},
			},
			options: InjectFieldsOptions{
				// Options used for fields injection for docs.
				SkipEmptyFields: true,
				KeepExternal:    true,
			},
			result:  nil,
			valid:   true,
			changed: true,
		},
		{
			title: "keep empty group for package validation",
			defs: []common.MapStr{
				{
					"name": "host",
					"type": "group",
				},
			},
			result: []common.MapStr{
				{
					"name": "host",
					"type": "group",
				},
			},
			valid:   true,
			changed: false,
		},
		{
			title: "sequence of nested definitions to ensure recursion does not have side effects",
			defs: []common.MapStr{
				{
					"name": "container",
					"type": "group",
					"fields": []interface{}{
						common.MapStr{
							"name":     "id",
							"external": "test",
						},
					},
				},
				{
					"name": "host",
					"type": "group",
					"fields": []interface{}{
						common.MapStr{
							"name":     "id",
							"external": "test",
						},
					},
				},
			},
			result: []common.MapStr{
				{
					"name": "container",
					"type": "group",
					"fields": []common.MapStr{
						{
							"name":        "id",
							"description": "Container identifier.",
							"type":        "keyword",
						},
					},
				},
				{
					"name": "host",
					"type": "group",
					"fields": []common.MapStr{
						{
							"name":        "id",
							"description": "Unique host id",
							"type":        "keyword",
						},
					},
				},
			},
			valid:   true,
			changed: true,
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
			Name:        "event.dataset",
			Description: "Dataset that collected this event",
			Type:        "keyword",
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
		{
			Name:        "source.mac",
			Description: "MAC address of the source.",
			Pattern:     "^[A-F0-9]{2}(-[A-F0-9]{2}){5,}$",
			Type:        "keyword",
		},
		{
			Name:        "host",
			Description: "A general computing instance",
			Type:        "group",
			Fields: []FieldDefinition{
				{
					Name:        "id",
					Description: "Unique host id",
					Type:        "keyword",
				},
				{
					Name:        "hostname",
					Description: "Hostname of the host",
					Type:        "keyword",
				},
			},
		},
	}}
	dm := &DependencyManager{schema: schema}

	for _, c := range cases {
		t.Run(c.title, func(t *testing.T) {
			result, changed, err := dm.InjectFieldsWithOptions(c.defs, c.options)
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
