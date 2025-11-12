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
		title            string
		preview          map[string]any
		actual           map[string]any
		schema           []FieldDefinition
		dynamicTemplates []map[string]any
		exceptionFields  []string
		expectedErrors   []string
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
			title: "validate fields with ECS",
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
				"foo": map[string]any{
					"type": "keyword",
				},
				"user": map[string]any{
					"type": "keyword",
				},
				"time": map[string]any{
					"type": "keyword",
					"fields": map[string]any{
						"text": map[string]any{
							"type": "match_only_text",
						},
						// there should be a dynamic template in order to exist this multi-field
						"other": map[string]any{
							"type": "match_only_text",
						},
					},
				},
				// Should this fail since it has no multi-fields as in the ECS definition?
				"name": map[string]any{
					"type": "keyword",
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
				{
					Name:     "time",
					Type:     "keyword",
					External: "ecs",
					MultiFields: []FieldDefinition{
						{
							Name:     "text",
							Type:     "match_only_text",
							External: "ecs",
						},
					},
				},
				{
					Name:     "name",
					Type:     "keyword",
					External: "ecs",
					MultiFields: []FieldDefinition{
						{
							Name:     "text",
							Type:     "match_only_text",
							External: "ecs",
						},
					},
				},
			},
			expectedErrors: []string{
				`field "metrics" is undefined: actual mapping type (long) does not match with ECS definition type: keyword`,
				`field "user" is undefined: field definition not found`,
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
						"name": map[string]any{
							"type": "text",
						},
						"os": map[string]any{
							"type": "text",
						},
					},
				},
			},
			schema: []FieldDefinition{},
			// If this skip is not present, `host.os` would be undefined
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
				`field "foo" is undefined: field definition not found`,
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
				`invalid value in field "foo": constant_keyword value in preview "example" does not match the actual mapping value "bar"`,
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
				"time": map[string]any{
					"type": "keyword",
					"fields": map[string]any{
						"text": map[string]any{
							"type": "match_only_text",
						},
					},
				},
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
							"type": "keyword",
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
				"time": map[string]any{
					"type": "keyword",
					"fields": map[string]any{
						"text": map[string]any{
							"type": "match_only_text",
						},
					},
				},
				"foo": map[string]any{
					"type": "keyword",
					"fields": map[string]any{
						"text": map[string]any{
							"type": "match_only_text",
						},
						"other": map[string]any{
							"type": "match_only_text",
						},
					},
				},
				"bar": map[string]any{
					"properties": map[string]any{
						"type": map[string]any{
							"type": "keyword",
							"fields": map[string]any{
								"text": map[string]any{
									"type": "match_only_text",
								},
							},
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
				`field "foo.text" is undefined: field definition not found`,
				`field "bar.type" is undefined: not found multi_fields definitions in preview mapping`,
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
				`field "foo" is undefined: not found multi_fields definitions in preview mapping`,
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
				`field "file.path" is undefined: field definition not found`,
				`field "bar" is undefined: field definition not found`,
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
				"nonexisting": map[string]any{
					"properties": map[string]any{
						"field": map[string]any{
							"type": "object",
						},
					},
				},
			},
			schema:         []FieldDefinition{},
			expectedErrors: []string{
				// TODO: there is an exception in the logic to not raise this error
				// `field "_tmp" is undefined: field definition not found`,
			},
		},
		{
			title: "validate fully dynamic objects in preview",
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
								"string": map[string]any{
									"type":    "object",
									"dynamic": "true",
								},
								"foo": map[string]any{
									"type": "keyword",
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
								"string": map[string]any{
									"dynamic": "true",
									"properties": map[string]any{
										"innodb_data_fsyncs": map[string]any{
											"type": "keyword",
										},
									},
								},
								"example": map[string]any{
									"type": "keyword",
								},
								"foo": map[string]any{
									"type": "keyword",
								},
							},
						},
					},
				},
			},
			dynamicTemplates: []map[string]any{
				{
					"sql.metrics.string.*": map[string]any{
						"path_match": "sql.metrics.string.*",
						"mapping": map[string]any{
							"type": "keyword",
						},
					},
				},
			},
			schema: []FieldDefinition{},
			expectedErrors: []string{
				`field "sql.metrics.numeric.innodb_data_fsyncs" is undefined: field definition not found`,
				`field "sql.metrics.example" is undefined: field definition not found`,
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
				"foo": map[string]any{
					"type":  "constant_keyword",
					"value": "example",
				},
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
			exceptionFields: []string{"access.field"},
			expectedErrors: []string{
				`field "error.field" is undefined: field definition not found`,
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
				"bar": map[string]any{
					"properties": map[string]any{
						"type": map[string]any{
							"type": "keyword",
						},
						"properties": map[string]any{
							"type": "keyword",
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
				"bar": map[string]any{
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
			schema: []FieldDefinition{},
			expectedErrors: []string{
				`field "bar.type.ignore_above" is undefined`,
				`field "bar.properties.ignore_above" is undefined`,
			},
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
		{
			title: "undefined parameter values within an object",
			preview: map[string]any{
				"foo": map[string]any{
					"properties": map[string]any{
						"type": map[string]any{
							"type": "keyword",
						},
					},
				},
			},
			actual: map[string]any{
				"foo": map[string]any{
					"properties": map[string]any{
						"type": map[string]any{
							"type":               "keyword",
							"time_series_matric": "counter",
						},
					},
				},
			},
			schema: []FieldDefinition{},
			expectedErrors: []string{
				`field "foo.type.time_series_matric" is undefined`,
			},
		},
		{
			title: "different number types",
			preview: map[string]any{
				"foo": map[string]any{
					"type": "float",
				},
			},
			actual: map[string]any{
				"foo": map[string]any{
					"type": "long",
				},
				"bar": map[string]any{
					"type": "long",
				},
			},
			schema: []FieldDefinition{
				{
					Name:     "bar",
					Type:     "float",
					External: "ecs",
				},
			},
			expectedErrors: []string{},
		},
		{
			title: "skip nested types before spec 3.0.1",
			preview: map[string]any{
				"foo": map[string]any{
					"type": "nested",
				},
			},
			actual: map[string]any{
				"foo": map[string]any{
					"type": "nested",
					"properties": map[string]any{
						"bar": map[string]any{
							"type": "long",
						},
					},
				},
			},
			// foo is added to the exception list because it is type nested
			exceptionFields: []string{"foo"},
			schema:          []FieldDefinition{},
			expectedErrors:  []string{},
		},
		{
			title: "validate nested types starting spec 3.0.1",
			preview: map[string]any{
				"foo": map[string]any{
					"type": "nested",
				},
			},
			actual: map[string]any{
				"foo": map[string]any{
					"type": "nested",
					"properties": map[string]any{
						"bar": map[string]any{
							"type": "long",
						},
					},
				},
			},
			exceptionFields: []string{},
			schema:          []FieldDefinition{},
			expectedErrors: []string{
				`undefined field mappings found in path: "foo"`,
			},
		},
		{
			title:   "fields matching dynamic templates",
			preview: map[string]any{},
			actual: map[string]any{
				"foo": map[string]any{
					"type": "keyword",
				},
				"foa": map[string]any{
					"type": "double",
				},
				"fob": map[string]any{
					"type":               "double",
					"time_series_metric": "gauge",
				},
				"bar": map[string]any{
					"type": "text",
					"fields": map[string]any{
						"text": map[string]any{
							"type": "keyword",
						},
					},
				},
				"bar_double": map[string]any{
					"type": "double",
				},
				"full_regex_1": map[string]any{
					"type": "double",
				},
			},
			dynamicTemplates: []map[string]any{
				{
					"fo*_keyword": map[string]any{
						"path_match":           "fo*",
						"path_unmatch":         []any{"foa", "fob"},
						"unmatch_mapping_type": []any{"long", "double"},
						"mapping": map[string]any{
							"type": "keyword",
						},
					},
				},
				{
					"fo*_number": map[string]any{
						"path_match":         "fo*",
						"path_unmatch":       "foo",
						"match_mapping_type": []any{"long", "double"},
						"mapping": map[string]any{
							"type":               "double",
							"time_series_metric": "counter",
						},
					},
				},
				{
					"bar_match": map[string]any{
						"unmatch":            []any{"foo", "foo42", "*42"},
						"match":              []any{"*ar", "bar42"},
						"match_mapping_type": "text",
						"mapping": map[string]any{
							"type": "text",
							"fields": map[string]any{
								"text": map[string]any{
									"type": "keyword",
								},
							},
						},
					},
				},
				{
					"bar_star_double": map[string]any{
						"match":                "*",
						"unmatch":              "full*",
						"unmatch_mapping_type": []any{"text"},
						"mapping": map[string]any{
							"type": "double",
						},
					},
				},
				{
					"full_regex_1": map[string]any{
						"match_pattern": "regex",
						"match":         "^full_.*\\d$",
						"mapping": map[string]any{
							"type": "double",
						},
					},
				},
			},
			exceptionFields: []string{},
			schema:          []FieldDefinition{},
			expectedErrors: []string{
				// Should it be considered this error in "foa" "missing time_series_metric bar"?
				// "fob" is failing because it does not have the expected value for the "time_series_metric" field
				`field "fob" is undefined: field definition not found`,
			},
		},
	}

	for _, c := range cases {
		t.Run(c.title, func(t *testing.T) {
			logger.EnableDebugMode()
			v, err := CreateValidatorForMappings(nil,
				WithMappingValidatorFallbackSchema(c.schema),
				WithMappingValidatorExceptionFields(c.exceptionFields),
			)
			require.NoError(t, err)

			errs := v.compareMappings("", false, c.preview, c.actual, c.dynamicTemplates)
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
