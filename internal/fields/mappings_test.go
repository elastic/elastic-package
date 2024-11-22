// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package fields

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/elastic-package/internal/logger"
)

func TestComparingMappings(t *testing.T) {
	cases := []struct {
		title          string
		preview        map[string]any
		actual         map[string]any
		schema         []FieldDefinition
		expectedErrors []string
	}{
		{
			title: "same mappings",
			preview: map[string]any{
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
			actual: map[string]any{
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
			schema:         []FieldDefinition{},
			expectedErrors: []string{},
		},
		{
			title: "validate field with ECS",
			preview: map[string]any{
				"foo": map[string]any{
					"type": "keyword",
				},
			},
			actual: map[string]any{
				"bar": map[string]any{
					"type": "keyword",
				},
				"metrics": map[string]any{
					"type": "long",
				},
				"user": map[string]any{
					"type": "text",
				},
			},
			schema: []FieldDefinition{
				{
					Name:     "bar",
					Type:     "keyword",
					External: "ecs",
				},
				{
					Name:     "metrics",
					Type:     "keyword",
					External: "ecs",
				},
				{
					Name:     "user",
					Type:     "keyword",
					External: "",
				},
			},
			expectedErrors: []string{
				`field "user" is undefined: missing definition for path`,
				`field "metrics" is undefined: actual mapping type (long) does not match with ECS definition type: keyword`,
			},
		},
		{
			title: "skip host group mappings",
			preview: map[string]any{
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
			actual: map[string]any{
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
			schema:         []FieldDefinition{},
			expectedErrors: []string{},
		},
		{
			title: "missing mappings",
			preview: map[string]any{
				"@timestamp": map[string]any{
					"type": "keyword",
				},
			},
			actual: map[string]any{
				"@timestamp": map[string]any{
					"type": "keyword",
				},
				"foo": map[string]any{
					"type": "keyword",
				},
			},
			schema: []FieldDefinition{},
			expectedErrors: []string{
				`field "foo" is undefined: missing definition for path`,
			},
		},
		{
			title: "validate constant_keyword value",
			preview: map[string]any{
				"foo": map[string]any{
					"type":  "constant_keyword",
					"value": "example",
				},
			},
			actual: map[string]any{
				"foo": map[string]any{
					"type":  "constant_keyword",
					"value": "bar",
				},
			},
			schema: []FieldDefinition{},
			expectedErrors: []string{
				`constant_keyword value in preview "example" does not match the actual mapping value "bar" for path: "foo"`,
			},
		},
		{
			title: "skip constant_keyword value",
			preview: map[string]any{
				"foo": map[string]any{
					"type": "constant_keyword",
				},
			},
			actual: map[string]any{
				"foo": map[string]any{
					"type":  "constant_keyword",
					"value": "bar",
				},
			},
			schema:         []FieldDefinition{},
			expectedErrors: []string{},
		},
		{
			title: "unexpected constant_keyword type",
			preview: map[string]any{
				"foo": map[string]any{
					"type": "keyword",
				},
			},
			actual: map[string]any{
				"foo": map[string]any{
					"type":  "constant_keyword",
					"value": "bar",
				},
			},
			schema: []FieldDefinition{},
			expectedErrors: []string{
				`invalid type for "foo": no constant_keyword type set in preview mapping`,
			},
		},
		{
			title: "validate multifields failure",
			preview: map[string]any{
				"foo": map[string]any{
					"type": "keyword",
					"fields": map[string]any{
						"other": map[string]any{
							"type": "match_only_text",
						},
					},
				},
				"bar": map[string]any{
					"properties": map[string]any{
						"type": map[string]any{
							"type": "constant_keyword",
						},
						"fields": map[string]any{
							"type": "text",
							"fields": map[string]any{
								"text": map[string]any{
									"type": "match_only_text",
								},
							},
						},
					},
				},
			},
			actual: map[string]any{
				"foo": map[string]any{
					"type": "keyword",
					"fields": map[string]any{
						"text": map[string]any{
							"type": "match_only_text",
						},
					},
				},
				"bar": map[string]any{
					"properties": map[string]any{
						"type": map[string]any{
							"type": "constant_keyword",
						},
						"fields": map[string]any{
							"type": "text",
							"fields": map[string]any{
								"text": map[string]any{
									"type": "match_only_text",
								},
							},
						},
					},
				},
			},
			schema: []FieldDefinition{},
			expectedErrors: []string{
				`field "foo.text" is undefined: missing definition for path`,
			},
		},
		{
			title: "missing multifields",
			preview: map[string]any{
				"foo": map[string]any{
					"type": "keyword",
				},
			},
			actual: map[string]any{
				"foo": map[string]any{
					"type": "keyword",
					"fields": map[string]any{
						"text": map[string]any{
							"type": "match_only_text",
						},
					},
				},
			},
			schema: []FieldDefinition{},
			expectedErrors: []string{
				`not found multi_fields in preview mappings for path: foo`,
			},
		},
		{
			title: "validate nested object",
			preview: map[string]any{
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
			actual: map[string]any{
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
			schema: []FieldDefinition{},
			expectedErrors: []string{
				`field "file.path" is undefined: missing definition for path`,
				`field "bar" is undefined: missing definition for path`,
			},
		},
		{
			title: "empty objects",
			preview: map[string]any{
				"@timestamp": map[string]any{
					"type": "keyword",
				},
			},
			actual: map[string]any{
				"@timestamp": map[string]any{
					"type": "keyword",
				},
				"_tmp": map[string]any{
					"type": "object",
				},
			},
			schema:         []FieldDefinition{},
			expectedErrors: []string{
				// TODO: there is an exception in the logic to not raise this error
				// `field "_tmp" is undefined: missing definition for path`,
			},
		},
		{
			title: "skip dynamic objects", // TODO: should this be checked using dynamic templates?
			preview: map[string]any{
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
			actual: map[string]any{
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
			schema:         []FieldDefinition{},
			expectedErrors: []string{},
		},
		{
			title: "compare all objects even dynamic true", // TODO: should this be checked using dynamic templates?
			preview: map[string]any{
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
			actual: map[string]any{
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
			schema: []FieldDefinition{},
			expectedErrors: []string{
				`field "sql.metrics.example" is undefined: missing definition for path`,
			},
		},
		{
			title: "ignore local type array objects",
			preview: map[string]any{
				"foo": map[string]any{
					"type":  "constant_keyword",
					"value": "example",
				},
			},
			actual: map[string]any{
				"access": map[string]any{
					"properties": map[string]any{
						"field": map[string]any{
							"type":         "keyword",
							"ignore_above": 1024,
						},
					},
				},
				"error": map[string]any{
					"properties": map[string]any{
						"field": map[string]any{
							"type":         "keyword",
							"ignore_above": 1024,
						},
					},
				},
				"status": map[string]any{
					"properties": map[string]any{
						"field": map[string]any{
							"type":         "keyword",
							"ignore_above": 1024,
						},
					},
				},
			},
			schema: []FieldDefinition{
				{
					Name:     "access.field",
					Type:     "array",
					External: "",
				},
				{
					Name:     "status.field",
					Type:     "array",
					External: "ecs",
				},
			},
			expectedErrors: []string{
				`field "error.field" is undefined: missing definition for path`,
				// should status.field return error ? or should it be ignored?
				`field "status.field" is undefined: actual mapping type (keyword) does not match with ECS definition type: array`,
			},
		},
		{
			title: "properties and type as a fields",
			preview: map[string]any{
				"foo": map[string]any{
					"properties": map[string]any{
						"type": map[string]any{
							"type":         "keyword",
							"ignore_above": 1024,
						},
						"properties": map[string]any{
							"type":         "keyword",
							"ignore_above": 1024,
						},
					},
				},
			},
			actual: map[string]any{
				"foo": map[string]any{
					"properties": map[string]any{
						"type": map[string]any{
							"type":         "keyword",
							"ignore_above": 1024,
						},
						"properties": map[string]any{
							"type":         "keyword",
							"ignore_above": 1024,
						},
					},
				},
			},
			schema:         []FieldDefinition{},
			expectedErrors: []string{},
		},
		{
			title: "different parameter values within an object",
			preview: map[string]any{
				"foo": map[string]any{
					"properties": map[string]any{
						"type": map[string]any{
							"type":         "keyword",
							"ignore_above": 1024,
						},
					},
				},
			},
			actual: map[string]any{
				"foo": map[string]any{
					"properties": map[string]any{
						"type": map[string]any{
							"type":         "long",
							"ignore_above": 2048,
						},
					},
				},
			},
			schema: []FieldDefinition{},
			expectedErrors: []string{
				`unexpected value found in mapping for field "foo.type.type": preview mappings value ("keyword") different from the actual mappings value ("long")`,
				`unexpected value found in mapping for field "foo.type.ignore_above": preview mappings value (1024) different from the actual mappings value (2048)`,
			},
		},
	}

	for _, c := range cases {
		t.Run(c.title, func(t *testing.T) {
			logger.EnableDebugMode()
			v, err := CreateValidatorForMappings("", nil,
				WithMappingValidatorFallbackSchema(c.schema),
				WithMappingValidatorDisabledDependencyManagement(),
			)
			require.NoError(t, err)

			errs := v.compareMappings("", c.preview, c.actual)
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
