// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package fields

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/elastic-package/internal/common"
	"github.com/elastic/elastic-package/internal/packages/buildmanifest"
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
			options: InjectFieldsOptions{
				IncludeValidationSettings: true,
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
		{
			title: "disallowed reusable field at lop level",
			defs: []common.MapStr{
				{
					"name":     "geo.city_name",
					"external": "test",
				},
			},
			options: InjectFieldsOptions{
				DisallowReusableECSFieldsAtTopLevel: true,
			},
			valid: false,
		},
		{
			title: "legacy support to reuse field at lop level",
			defs: []common.MapStr{
				{
					"name":     "geo.city_name",
					"external": "test",
				},
			},
			options: InjectFieldsOptions{
				DisallowReusableECSFieldsAtTopLevel: false,
			},
			result: []common.MapStr{
				{
					"name":        "geo.city_name",
					"description": "City name",
					"type":        "keyword",
				},
			},
			changed: true,
			valid:   true,
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
				{
					Name:        "geo.city_name",
					Description: "City name",
					Type:        "keyword",
				},
			},
		},
		{
			Name:        "geo",
			Description: "Location info",
			Type:        "group",
			Fields: []FieldDefinition{
				{
					Name:               "city_name",
					Description:        "City name",
					Type:               "keyword",
					disallowAtTopLevel: true,
				},
			},
			Reusable: &ReusableConfig{
				TopLevel: false,
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

func TestDependencyManagerWithECS(t *testing.T) {
	const ecsNestedPath8_10_0 = "./testdata/ecs_nested_v8.10.0.yml"
	deps := buildmanifest.Dependencies{
		ECS: buildmanifest.ECSDependency{
			Reference: "file://" + ecsNestedPath8_10_0,
		},
	}
	urls := SchemaURLs{
		ECSBase: DefaultECSSchemaBaseURL,
	}
	dm, err := CreateFieldDependencyManager(deps, urls)
	require.NoError(t, err)

	cases := []struct {
		title   string
		defs    []common.MapStr
		result  []common.MapStr
		options InjectFieldsOptions
		checkFn func(*testing.T, []common.MapStr)
		valid   bool
	}{
		{
			title: "disallowed reusable field at lop level",
			defs: []common.MapStr{
				{
					"name":     "geo.city_name",
					"external": "ecs",
				},
			},
			options: InjectFieldsOptions{
				DisallowReusableECSFieldsAtTopLevel: true,
			},
			valid: false,
		},
		{
			title: "legacy support to reuse field at lop level",
			defs: []common.MapStr{
				{
					"name":     "geo.city_name",
					"external": "ecs",
				},
			},
			options: InjectFieldsOptions{
				DisallowReusableECSFieldsAtTopLevel: false,
			},
			result: []common.MapStr{
				{
					"name":        "geo.city_name",
					"description": "City name.",
					"type":        "keyword",
				},
			},
			valid: true,
		},
		{
			title: "allowed values are injected for validation",
			defs: []common.MapStr{
				{
					"name":     "event.type",
					"external": "ecs",
				},
			},
			options: InjectFieldsOptions{
				IncludeValidationSettings: true,
			},
			valid: true,
			checkFn: func(t *testing.T, result []common.MapStr) {
				require.Len(t, result, 1)
				_, ok := result[0]["allowed_values"]
				if !assert.True(t, ok) {
					d, _ := json.MarshalIndent(result[0], "", "  ")
					t.Logf("expected to find allowed_values in %s", string(d))
				}
			},
		},
		{
			title: "allowed values are not injected when not intended for validation",
			defs: []common.MapStr{
				{
					"name":     "event.type",
					"external": "ecs",
				},
			},
			options: InjectFieldsOptions{
				IncludeValidationSettings: false,
			},
			valid: true,
			checkFn: func(t *testing.T, result []common.MapStr) {
				require.Len(t, result, 1)
				_, ok := result[0]["allowed_values"]
				assert.False(t, ok)
			},
		},
		{
			title: "object type is imported",
			defs: []common.MapStr{
				{
					"name":     "container.labels",
					"external": "ecs",
				},
			},
			options: InjectFieldsOptions{
				IncludeValidationSettings: true,
			},
			valid: true,
			result: []common.MapStr{
				{
					"name":        "container.labels",
					"description": "Image labels.",
					"type":        "object",
					"object_type": "keyword",
				},
			},
		},
		{
			title: "object to nested override",
			defs: []common.MapStr{
				{
					"name":     "dns.answers",
					"external": "ecs",
					"type":     "nested",
				},
			},
			options: InjectFieldsOptions{},
			valid:   true,
			result: []common.MapStr{
				{
					"name":        "dns.answers",
					"description": "An array containing an object for each answer section returned by the server.\nThe main keys that should be present in these objects are defined by ECS. Records that have more information may contain more keys than what ECS defines.\nNot all DNS data sources give all details about DNS answers. At minimum, answer objects must contain the `data` key. If more information is available, map as much of it to ECS as possible, and add any additional fields to the answer objects as custom fields.",
					"type":        "nested",
				},
			},
		},
		{
			title: "object to group override",
			defs: []common.MapStr{
				{
					"name":     "dns.answers",
					"external": "ecs",
					"type":     "group",
				},
			},
			options: InjectFieldsOptions{},
			valid:   true,
			result: []common.MapStr{
				{
					"name":        "dns.answers",
					"description": "An array containing an object for each answer section returned by the server.\nThe main keys that should be present in these objects are defined by ECS. Records that have more information may contain more keys than what ECS defines.\nNot all DNS data sources give all details about DNS answers. At minimum, answer objects must contain the `data` key. If more information is available, map as much of it to ECS as possible, and add any additional fields to the answer objects as custom fields.",
					"type":        "group",
				},
			},
		},
	}

	for _, c := range cases {
		t.Run(c.title, func(t *testing.T) {
			result, _, err := dm.InjectFieldsWithOptions(c.defs, c.options)
			if !c.valid {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			if len(c.result) > 0 {
				assert.EqualValues(t, c.result, result)
			}
			if c.checkFn != nil {
				t.Run("checkFn", func(t *testing.T) {
					c.checkFn(t, result)
				})
			}
		})
	}
}

func TestValidate_SetExternalECS(t *testing.T) {
	urls := SchemaURLs{
		ECSBase: DefaultECSSchemaBaseURL,
	}
	repositoryRoot, packageRoot, fieldsDir := pathsForValidator(t, "other", "imported_mappings_tests", "first")
	validator, err := CreateValidator(repositoryRoot, packageRoot, fieldsDir,
		WithSpecVersion("2.3.0"),
		WithEnabledImportAllECSSChema(true),
		WithSchemaURLs(urls),
	)
	require.NoError(t, err)
	require.NotNil(t, validator)

	require.NotEmpty(t, validator.Schema)

	cases := []struct {
		title    string
		field    string
		external string
		exists   bool
	}{
		{
			title:    "field defined just in ECS",
			field:    "ecs.version",
			external: "ecs",
			exists:   true,
		},
		{
			title:    "field defined fields directory package",
			field:    "service.status.duration.histogram",
			external: "",
			exists:   true,
		},
		{
			title:    "undefined field",
			field:    "foo",
			external: "",
			exists:   false,
		},
	}

	for _, c := range cases {
		t.Run(c.title, func(t *testing.T) {
			found := FindElementDefinition(c.field, validator.Schema)
			if !c.exists {
				assert.Nil(t, found)
				return
			}

			require.NotNil(t, found)
			assert.Equal(t, c.external, found.External)
		})
	}
}

func TestECSSchemaURL(t *testing.T) {
	cases := []struct {
		title        string
		baseURL      string
		gitReference string
		schemaFile   string
		expected     string
		expectedErr  bool
	}{
		{
			title:        "default",
			baseURL:      "https://raw.githubusercontent.com/elastic/ecs",
			gitReference: "v8.11.0",
			schemaFile:   ecsSchemaFile,
			expected:     "https://raw.githubusercontent.com/elastic/ecs/v8.11.0/generated/ecs/ecs_nested.yml",
		},
		{
			title:        "fork in github",
			baseURL:      "https://raw.githubusercontent.com/jsoriano/ecs",
			gitReference: "v8.11.0",
			schemaFile:   ecsSchemaFile,
			expected:     "https://raw.githubusercontent.com/jsoriano/ecs/v8.11.0/generated/ecs/ecs_nested.yml",
		},
		{
			title:        "fork in forgejo",
			baseURL:      "https://somehost.org/raw/jsoriano/ecs",
			gitReference: "v8.11.0",
			schemaFile:   ecsSchemaFile,
			expected:     "https://somehost.org/raw/jsoriano/ecs/v8.11.0/generated/ecs/ecs_nested.yml",
		},
		{
			title:        "invalid URL",
			baseURL:      "/somehost.org/raw/elastic/ecs",
			gitReference: "v8.11.0",
			schemaFile:   ecsSchemaFile,
			expectedErr:  true,
		},
		{
			title:        "invalid scheme",
			baseURL:      "file://../../..",
			gitReference: "v8.11.0",
			schemaFile:   ecsSchemaFile,
			expectedErr:  true,
		},
		{
			title:        "no URL",
			baseURL:      "",
			gitReference: "v8.11.0",
			schemaFile:   ecsSchemaFile,
			expectedErr:  true,
		},
	}

	for _, c := range cases {
		t.Run(c.title, func(t *testing.T) {
			found, err := ecsSchemaURL(c.baseURL, c.gitReference, c.schemaFile)
			t.Log(found)
			if c.expectedErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, c.expected, found)
		})
	}
}
