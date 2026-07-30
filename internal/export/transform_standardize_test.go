// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package export

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/elastic-package/internal/common"
)

func TestStandardizeObjectProperties_Title(t *testing.T) {
	ctx := &transformationContext{packageName: "mypackage"}

	tests := []struct {
		name          string
		input         interface{}
		expectedTitle interface{}
	}{
		{
			name:          "string with ECS suffix is trimmed",
			input:         "My Dashboard ECS",
			expectedTitle: "My Dashboard",
		},
		{
			name:          "string without ECS suffix is unchanged",
			input:         "My Dashboard",
			expectedTitle: "My Dashboard",
		},
		{
			name:          "empty string is unchanged",
			input:         "",
			expectedTitle: "",
		},
		{
			name:          "map value (axis config) is preserved without panic",
			input:         map[string]interface{}{"visible": false},
			expectedTitle: common.MapStr{"visible": false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := common.MapStr{"title": tt.input}
			result, err := standardizeObjectProperties(ctx, obj)
			require.NoError(t, err)
			got, err := result.GetValue("title")
			require.NoError(t, err)
			assert.Equal(t, tt.expectedTitle, got)
		})
	}
}

func TestStandardizeObjectProperties_TitleNestedInMap(t *testing.T) {
	ctx := &transformationContext{packageName: "mypackage"}

	obj := common.MapStr{
		"attributes": map[string]interface{}{
			"title": "Nested Panel ECS",
		},
	}
	result, err := standardizeObjectProperties(ctx, obj)
	require.NoError(t, err)

	got, err := result.GetValue("attributes.title")
	require.NoError(t, err)
	assert.Equal(t, "Nested Panel", got)
}

func TestStandardizeObjectProperties_AxisTitleMapNotPanics(t *testing.T) {
	// Reproduces the axis config case: embeddableConfig.axis.{x,y,y2}.title is a
	// map like {"visible": false} rather than a string.
	ctx := &transformationContext{packageName: "mypackage"}

	obj := common.MapStr{
		"embeddableConfig": map[string]interface{}{
			"title": "Panel ECS",
			"axis": map[string]interface{}{
				"x": map[string]interface{}{
					"title": map[string]interface{}{"visible": false},
				},
				"y": map[string]interface{}{
					"title": map[string]interface{}{"visible": false},
				},
			},
		},
	}

	result, err := standardizeObjectProperties(ctx, obj)
	require.NoError(t, err)

	// Top-level string title inside embeddableConfig is still transformed.
	got, err := result.GetValue("embeddableConfig.title")
	require.NoError(t, err)
	assert.Equal(t, "Panel", got)

	// Axis titles (maps) are left untouched.
	gotX, err := result.GetValue("embeddableConfig.axis.x.title")
	require.NoError(t, err)
	assert.Equal(t, common.MapStr{"visible": false}, gotX)

	gotY, err := result.GetValue("embeddableConfig.axis.y.title")
	require.NoError(t, err)
	assert.Equal(t, common.MapStr{"visible": false}, gotY)
}

func TestStandardizeObjectProperties_TitleInArrayOfMaps(t *testing.T) {
	ctx := &transformationContext{packageName: "mypackage"}

	obj := common.MapStr{
		"panels": []map[string]interface{}{
			{"title": "Panel One ECS"},
			{"title": "Panel Two"},
		},
	}

	result, err := standardizeObjectProperties(ctx, obj)
	require.NoError(t, err)

	panels, err := result.GetValue("panels")
	require.NoError(t, err)

	arr, ok := panels.([]map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "Panel One", arr[0]["title"])
	assert.Equal(t, "Panel Two", arr[1]["title"])
}

func TestStandardizeObjectProperties_Markdown(t *testing.T) {
	ctx := &transformationContext{packageName: "mypackage"}

	content := "[link](#/dashboard/mypackage-abc123)"
	obj := common.MapStr{"markdown": content}

	result, err := standardizeObjectProperties(ctx, obj)
	require.NoError(t, err)

	got, err := result.GetValue("markdown")
	require.NoError(t, err)
	assert.Equal(t, content, got)
}

func TestStandardizeObjectProperties_MarkdownAdjustsObjectID(t *testing.T) {
	ctx := &transformationContext{packageName: "mypackage"}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "ID without package prefix gets it added",
			input:    "See [this](#/dashboard/abc123)",
			expected: "See [this](#/dashboard/mypackage-abc123)",
		},
		{
			name:     "ID with ECS suffix has it removed",
			input:    "See [this](#/dashboard/abc123-ecs)",
			expected: "See [this](#/dashboard/mypackage-abc123)",
		},
		{
			name:     "ID already prefixed is not double-prefixed",
			input:    "See [this](#/dashboard/mypackage-abc123)",
			expected: "See [this](#/dashboard/mypackage-abc123)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := common.MapStr{"markdown": tt.input}
			result, err := standardizeObjectProperties(ctx, obj)
			require.NoError(t, err)
			got, err := result.GetValue("markdown")
			require.NoError(t, err)
			assert.Equal(t, tt.expected, got)
		})
	}
}
