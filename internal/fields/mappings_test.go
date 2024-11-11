// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package fields

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/elastic-package/internal/logger"
)

func TestComparingMappings(t *testing.T) {
	cases := []struct {
		title          string
		preview        mappingDefinitions
		actual         mappingDefinitions
		ecsSchema      []FieldDefinition
		localSchema    []FieldDefinition
		expectedErrors []string
	}{
		{
			title: "same mappings",
			preview: mappingDefinitions{
				"@timestamp": map[string]string{
					"type": "keyword",
				},
				"host": map[string]any{
					"properties": map[string]any{
						"name": map[string]string{
							"type": "text",
						},
					},
				},
				"file": map[string]any{
					"properties": map[string]any{
						"path": map[string]any{
							"type": "text",
						},
					},
				},
				"foo": map[string]any{
					"type": "keyword",
					"fields": map[string]any{
						"text": map[string]any{
							"type": "match_only_text",
						},
					},
				},
			},
			actual: mappingDefinitions{
				"@timestamp": map[string]string{
					"type": "keyword",
				},
				"host": map[string]any{
					"properties": map[string]any{
						"name": map[string]string{
							"type": "text",
						},
					},
				},
				"file": map[string]any{
					"properties": map[string]any{
						"path": map[string]any{
							"type": "text",
						},
					},
				},
				"foo": map[string]any{
					"type": "keyword",
					"fields": map[string]any{
						"text": map[string]any{
							"type": "match_only_text",
						},
					},
				},
			},
			ecsSchema:      []FieldDefinition{},
			expectedErrors: []string{},
		},
		{
			title: "validate field with ECS",
			preview: mappingDefinitions{
				"foo": map[string]any{
					"type": "keyword",
				},
			},
			actual: mappingDefinitions{
				"bar": map[string]any{
					"type": "keyword",
				},
			},
			ecsSchema: []FieldDefinition{
				{
					Name: "bar",
					Type: "keyword",
				},
			},
			expectedErrors: []string{},
		},
		{
			title: "skip host group mappings",
			preview: mappingDefinitions{
				"@timestamp": map[string]any{
					"type": "keyword",
				},
				"host": map[string]any{
					"properties": map[string]any{
						"name": map[string]any{
							"type": "text",
						},
					},
				},
			},
			actual: mappingDefinitions{
				"@timestamp": map[string]any{
					"type": "keyword",
				},
				"host": map[string]any{
					"properties": map[string]any{
						"os": map[string]any{
							"type": "text",
						},
					},
				},
			},
			ecsSchema:      []FieldDefinition{},
			expectedErrors: []string{},
		},
		{
			title: "missing mappings",
			preview: mappingDefinitions{
				"@timestamp": map[string]any{
					"type": "keyword",
				},
			},
			actual: mappingDefinitions{
				"@timestamp": map[string]any{
					"type": "keyword",
				},
				"foo": map[string]any{
					"type": "keyword",
				},
			},
			ecsSchema: []FieldDefinition{},
			expectedErrors: []string{
				`field "foo" is undefined: missing definition for path`,
			},
		},
		{
			title: "validate constant_keyword value",
			preview: mappingDefinitions{
				"foo": map[string]any{
					"type":  "constant_keyword",
					"value": "example",
				},
			},
			actual: mappingDefinitions{
				"foo": map[string]any{
					"type":  "constant_keyword",
					"value": "bar",
				},
			},
			ecsSchema: []FieldDefinition{},
			expectedErrors: []string{
				`constant_keyword value in preview "example" does not match the actual mapping value "bar" for path: "foo"`,
			},
		},
		{
			title: "skip constant_keyword value",
			preview: mappingDefinitions{
				"foo": map[string]any{
					"type": "constant_keyword",
				},
			},
			actual: mappingDefinitions{
				"foo": map[string]any{
					"type":  "constant_keyword",
					"value": "bar",
				},
			},
			ecsSchema:      []FieldDefinition{},
			expectedErrors: []string{},
		},
		{
			title: "unexpected constant_keyword type",
			preview: mappingDefinitions{
				"foo": map[string]any{
					"type": "keyword",
				},
			},
			actual: mappingDefinitions{
				"foo": map[string]any{
					"type":  "constant_keyword",
					"value": "bar",
				},
			},
			ecsSchema: []FieldDefinition{},
			expectedErrors: []string{
				`invalid type for "foo": no constant_keyword type set in preview mapping`,
			},
		},
		{
			title: "validate multifields failure",
			preview: mappingDefinitions{
				"foo": map[string]any{
					"type": "keyword",
					"fields": map[string]any{
						"other": map[string]any{
							"type": "match_only_text",
						},
					},
				},
			},
			actual: mappingDefinitions{
				"foo": map[string]any{
					"type": "keyword",
					"fields": map[string]any{
						"text": map[string]any{
							"type": "match_only_text",
						},
					},
				},
			},
			ecsSchema: []FieldDefinition{},
			expectedErrors: []string{
				`field "foo.text" is undefined: missing definition for path`,
			},
		},
		{
			title: "missing multifields",
			preview: mappingDefinitions{
				"foo": map[string]any{
					"type": "keyword",
				},
			},
			actual: mappingDefinitions{
				"foo": map[string]any{
					"type": "keyword",
					"fields": map[string]any{
						"text": map[string]any{
							"type": "match_only_text",
						},
					},
				},
			},
			ecsSchema: []FieldDefinition{},
			expectedErrors: []string{
				`not found multi_fields in preview mappings for path: foo`,
			},
		},
		{
			title: "validate nested object",
			preview: mappingDefinitions{
				"foo": map[string]any{
					"type": "keyword",
				},
				"file": map[string]any{
					"properties": map[string]any{
						"size": map[string]any{
							"type": "double",
						},
					},
				},
			},
			actual: mappingDefinitions{
				"bar": map[string]any{
					"type": "keyword",
				},
				"file": map[string]any{
					"properties": map[string]any{
						"path": map[string]any{
							"type": "text",
						},
					},
				},
			},
			ecsSchema: []FieldDefinition{},
			expectedErrors: []string{
				`field "file.path" is undefined: missing definition for path`,
				`field "bar" is undefined: missing definition for path`,
			},
		},
		{
			title: "empty objects",
			preview: mappingDefinitions{
				"@timestamp": map[string]any{
					"type": "keyword",
				},
			},
			actual: mappingDefinitions{
				"@timestamp": map[string]any{
					"type": "keyword",
				},
				"_tmp": map[string]any{
					"type": "object",
				},
			},
			ecsSchema:      []FieldDefinition{},
			expectedErrors: []string{
				// TODO: there is an exception in the logic to not raise this error
				// `field "_tmp" is undefined: missing definition for path`,
			},
		},
		{
			title: "skip dynamic objects", // TODO: should this be checked using dynamic templates?
			preview: mappingDefinitions{
				"@timestamp": map[string]any{
					"type": "keyword",
				},
				"sql": map[string]any{
					"properties": map[string]any{
						"metrics": map[string]any{
							"properties": map[string]any{
								"dynamic": "true",
								"numeric": map[string]any{
									"type":    "object",
									"dynamic": "true",
								},
							},
						},
					},
				},
			},
			actual: mappingDefinitions{
				"@timestamp": map[string]any{
					"type": "keyword",
				},
				"sql": map[string]any{
					"properties": map[string]any{
						"metrics": map[string]any{
							"properties": map[string]any{
								"dynamic": "true",
								"numeric": map[string]any{
									"dynamic": "true",
									"properties": map[string]any{
										"innodb_data_fsyncs": map[string]any{
											"type": "long",
										},
									},
								},
							},
						},
					},
				},
			},
			ecsSchema:      []FieldDefinition{},
			expectedErrors: []string{},
		},
		{
			title: "compare all objects even dynamic true", // TODO: should this be checked using dynamic templates?
			preview: mappingDefinitions{
				"@timestamp": map[string]any{
					"type": "keyword",
				},
				"sql": map[string]any{
					"properties": map[string]any{
						"metrics": map[string]any{
							"properties": map[string]any{
								"dynamic": "true",
								"numeric": map[string]any{
									"type":    "object",
									"dynamic": "true",
								},
							},
						},
					},
				},
			},
			actual: mappingDefinitions{
				"@timestamp": map[string]any{
					"type": "keyword",
				},
				"sql": map[string]any{
					"properties": map[string]any{
						"metrics": map[string]any{
							"properties": map[string]any{
								"dynamic": "true",
								"numeric": map[string]any{
									"dynamic": "true",
									"properties": map[string]any{
										"innodb_data_fsyncs": map[string]any{
											"type": "long",
										},
									},
								},
								"example": map[string]any{
									"type": "keyword",
								},
							},
						},
					},
				},
			},
			ecsSchema: []FieldDefinition{},
			expectedErrors: []string{
				`field "sql.metrics.example" is undefined: missing definition for path`,
			},
		},
		{
			title: "ignore local type array objects",
			preview: mappingDefinitions{
				"foo": map[string]any{
					"type":  "constant_keyword",
					"value": "example",
				},
			},
			actual: mappingDefinitions{
				"access": map[string]any{
					"properties": map[string]any{
						"field": map[string]any{
							"type":         "keyword",
							"ignore_above": 1024,
						},
					},
				},
			},
			ecsSchema: []FieldDefinition{},
			localSchema: []FieldDefinition{
				{
					Name: "access.field",
					Type: "array",
				},
			},
			expectedErrors: []string{
				// `field \"access.field\" is undefined: missing definition for path`,
			},
		},
	}

	for _, c := range cases {
		t.Run(c.title, func(t *testing.T) {
			logger.EnableDebugMode()
			errs := compareMappings("", c.preview, c.actual, c.ecsSchema, c.localSchema)
			if len(c.expectedErrors) > 0 {
				assert.Len(t, errs, len(c.expectedErrors))
				for _, err := range errs {
					assert.Contains(t, c.expectedErrors, err.Error())
				}
			} else {
				assert.Len(t, errs, 0)
			}
		})
	}
}
