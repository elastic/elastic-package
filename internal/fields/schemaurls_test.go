// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package fields

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestSchemaURLs_UnmarshalYAML(t *testing.T) {
	tests := []struct {
		name     string
		yamlData string
		expected string
		wantErr  bool
	}{
		{
			name:     "unmarshal with ecs_base",
			yamlData: "ecs_base: https://example.com/ecs",
			expected: "https://example.com/ecs",
			wantErr:  false,
		},
		{
			name:     "unmarshal empty ecs_base",
			yamlData: "ecs_base: \"\"",
			expected: defaultECSSchemaBaseURL,
			wantErr:  false,
		},
		{
			name:     "unmarshal without ecs_base field",
			yamlData: "other_field: value",
			expected: defaultECSSchemaBaseURL,
			wantErr:  false,
		},
		{
			name:     "unmarshal empty yaml",
			yamlData: "{}",
			expected: defaultECSSchemaBaseURL,
			wantErr:  false,
		},
		{
			name:     "unmarshal invalid yaml",
			yamlData: "invalid: yaml: content",
			expected: "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var s SchemaURLs
			err := yaml.Unmarshal([]byte(tt.yamlData), &s)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			assert.Equal(t, tt.expected, s.ecsBase, "UnmarshalYAML() did not set ecsBase correctly")
		})
	}
}

func TestSchemaURLs_MarshalYAML(t *testing.T) {
	tests := []struct {
		name     string
		s        SchemaURLs
		expected string
		wantErr  bool
	}{
		{
			name:     "marshal with ecs_base",
			s:        SchemaURLs{ecsBase: "https://example.com/ecs"},
			expected: "ecs_base: https://example.com/ecs\n",
			wantErr:  false,
		},
		{
			name:     "marshal with empty ecs_base",
			s:        SchemaURLs{ecsBase: ""},
			expected: "ecs_base: " + defaultECSSchemaBaseURL + "\n",
			wantErr:  false,
		},
		{
			name:     "marshal with default ecs_base",
			s:        SchemaURLs{ecsBase: defaultECSSchemaBaseURL},
			expected: "ecs_base: " + defaultECSSchemaBaseURL + "\n",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := yaml.Marshal(tt.s)

			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			assert.Equal(t, tt.expected, string(data), "MarshalYAML() output mismatch")
		})
	}
}

func TestSchemaURLs_EcsBase(t *testing.T) {
	tests := []struct {
		name     string
		s        SchemaURLs
		expected string
	}{
		{
			name:     "ecs_base with custom value",
			s:        SchemaURLs{ecsBase: "https://custom.example.com/ecs"},
			expected: "https://custom.example.com/ecs",
		},
		{
			name:     "ecs_base with empty value returns default",
			s:        SchemaURLs{ecsBase: ""},
			expected: defaultECSSchemaBaseURL,
		},
		{
			name:     "ecs_base with default value",
			s:        SchemaURLs{ecsBase: defaultECSSchemaBaseURL},
			expected: defaultECSSchemaBaseURL,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.s.ECSBase()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNewSchemaURLs(t *testing.T) {
	tests := []struct {
		name     string
		opts     []schemaURLOption
		expected string
	}{
		{
			name:     "default constructor",
			opts:     nil,
			expected: defaultECSSchemaBaseURL,
		},
		{
			name:     "with custom ecs base url",
			opts:     []schemaURLOption{WithECSBaseURL("https://custom.example.com/ecs")},
			expected: "https://custom.example.com/ecs",
		},
		{
			name:     "with empty ecs base url",
			opts:     []schemaURLOption{WithECSBaseURL("")},
			expected: defaultECSSchemaBaseURL,
		},
		{
			name:     "multiple options (last wins)",
			opts:     []schemaURLOption{WithECSBaseURL("https://first.com"), WithECSBaseURL("https://second.com")},
			expected: "https://second.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewSchemaURLs(tt.opts...)
			assert.Equal(t, tt.expected, s.ecsBase, "NewSchemaURLs() did not set ecsBase correctly")
		})
	}
}

func TestSchemaURLs_YAMLTagIntegration(t *testing.T) {
	// Test that the struct works correctly when embedded in other YAML structures
	type TestConfig struct {
		Schema  SchemaURLs `yaml:"schema_urls"`
		Version string     `yaml:"version"`
	}

	yamlData := `
schema_urls:
  ecs_base: https://embedded.example.com/ecs
version: "1.0.0"
`

	var config TestConfig
	err := yaml.Unmarshal([]byte(yamlData), &config)
	require.NoError(t, err)

	assert.Equal(t, "https://embedded.example.com/ecs", config.Schema.ecsBase, "Embedded struct ecsBase mismatch")

	assert.Equal(t, "1.0.0", config.Version, "Embedded struct version mismatch")

	// Test marshal back
	marshaled, err := yaml.Marshal(config)
	require.NoError(t, err)

	expectedYAML := "schema_urls:\n    ecs_base: https://embedded.example.com/ecs\nversion: 1.0.0\n"
	assert.Equal(t, string(marshaled), expectedYAML)
}
