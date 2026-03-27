// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package fleetpkg

import (
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/parser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProcessor_UnmarshalYAML(t *testing.T) {
	src := []byte(`
set:
  field: key
  value: some_value
  on_failure:
    - set:
        field: event.kind
        value: pipeline_error
`)

	want := Processor{
		Type: "set",
		Attributes: map[string]interface{}{
			"field": "key",
			"value": "some_value",
		},
		OnFailure: []*Processor{
			{
				Type: "set",
				Attributes: map[string]interface{}{
					"field": "event.kind",
					"value": "pipeline_error",
				},
			},
		},
	}

	f, err := parser.ParseBytes(src, parser.ParseComments)
	require.NoError(t, err)

	var got Processor
	gotErr := yaml.NodeToValue(f.Docs[0].Body, &got)
	require.NoError(t, gotErr)

	assert.Equal(t, want.Type, got.Type)
	assert.Equal(t, want.Attributes, got.Attributes)
	assert.Len(t, got.OnFailure, 1)
	assert.Equal(t, want.OnFailure[0].Type, got.OnFailure[0].Type)
	assert.Equal(t, want.OnFailure[0].Attributes, got.OnFailure[0].Attributes)
	assert.Empty(t, want.OnFailure[0].OnFailure)

	assert.Equal(t, f.Docs[0].Body, got.Node)

	onFailurePath, err := yaml.PathString("$.set.on_failure[0]")
	require.NoError(t, err)
	onFailureNode, err := onFailurePath.FilterFile(f)
	require.NoError(t, err)
	assert.Equal(t, onFailureNode, got.OnFailure[0].Node)
}

func TestProcessor_GetAttribute(t *testing.T) {
	p := Processor{
		Attributes: map[string]any{
			"string": "test",
			"int":    1,
			"bool":   true,
			"float":  23.4,
		},
	}

	testCases := []struct {
		name      string
		key       string
		want      any
		wantFound bool
	}{
		{
			name:      "ok",
			key:       "string",
			want:      "test",
			wantFound: true,
		},
		{
			name:      "not-found",
			key:       "missing",
			wantFound: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, gotFound := p.GetAttribute(tc.key)

			if tc.wantFound {
				assert.True(t, gotFound)
				assert.Equal(t, tc.want, got)
			} else {
				assert.False(t, gotFound)
			}
		})
	}
}

func TestProcessor_GetAttributeString(t *testing.T) {
	p := Processor{
		Attributes: map[string]any{
			"string": "test",
			"int":    1,
			"bool":   true,
			"float":  23.4,
		},
	}

	testCases := []struct {
		name      string
		key       string
		want      any
		wantFound bool
	}{
		{
			name:      "ok",
			key:       "string",
			want:      "test",
			wantFound: true,
		},
		{
			name:      "wrong-type",
			key:       "int",
			wantFound: false,
		},
		{
			name:      "not-found",
			key:       "missing",
			wantFound: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, gotFound := p.GetAttributeString(tc.key)

			if tc.wantFound {
				assert.True(t, gotFound)
				assert.Equal(t, tc.want, got)
			} else {
				assert.False(t, gotFound)
			}
		})
	}
}

func TestProcessor_GetAttributeFloat(t *testing.T) {
	p := Processor{
		Attributes: map[string]any{
			"string": "test",
			"int":    1,
			"bool":   true,
			"float":  23.4,
		},
	}

	testCases := []struct {
		name      string
		key       string
		want      any
		wantFound bool
	}{
		{
			name:      "ok",
			key:       "float",
			want:      23.4,
			wantFound: true,
		},
		{
			name:      "wrong-type",
			key:       "string",
			wantFound: false,
		},
		{
			name:      "not-found",
			key:       "missing",
			wantFound: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, gotFound := p.GetAttributeFloat(tc.key)

			if tc.wantFound {
				assert.True(t, gotFound)
				assert.Equal(t, tc.want, got)
			} else {
				assert.False(t, gotFound)
			}
		})
	}
}

func TestProcessor_GetAttributeInt(t *testing.T) {
	p := Processor{
		Attributes: map[string]any{
			"string": "test",
			"int":    1,
			"bool":   true,
			"float":  23.4,
		},
	}

	testCases := []struct {
		name      string
		key       string
		want      any
		wantFound bool
	}{
		{
			name:      "ok",
			key:       "int",
			want:      1,
			wantFound: true,
		},
		{
			name:      "wrong-type",
			key:       "string",
			wantFound: false,
		},
		{
			name:      "not-found",
			key:       "missing",
			wantFound: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, gotFound := p.GetAttributeInt(tc.key)

			if tc.wantFound {
				assert.True(t, gotFound)
				assert.Equal(t, tc.want, got)
			} else {
				assert.False(t, gotFound)
			}
		})
	}
}

func TestProcessor_GetAttributeBool(t *testing.T) {
	p := Processor{
		Attributes: map[string]any{
			"string": "test",
			"int":    1,
			"bool":   true,
			"float":  23.4,
		},
	}

	testCases := []struct {
		name      string
		key       string
		want      any
		wantFound bool
	}{
		{
			name:      "ok",
			key:       "bool",
			want:      true,
			wantFound: true,
		},
		{
			name:      "wrong-type",
			key:       "string",
			wantFound: false,
		},
		{
			name:      "not-found",
			key:       "missing",
			wantFound: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, gotFound := p.GetAttributeBool(tc.key)

			if tc.wantFound {
				assert.True(t, gotFound)
				assert.Equal(t, tc.want, got)
			} else {
				assert.False(t, gotFound)
			}
		})
	}
}
