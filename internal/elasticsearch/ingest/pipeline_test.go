// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package ingest

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPipelineFileName(t *testing.T) {
	for _, tt := range []struct {
		title    string
		pipeline Pipeline
		expected string
	}{
		{
			title: "name with nonce",
			pipeline: Pipeline{
				Name:   "default-1234",
				Format: "yml",
			},
			expected: "default.yml",
		},
		{
			title: "name without nonce",
			pipeline: Pipeline{
				Name:   "mypipeline",
				Format: "json",
			},
			expected: "mypipeline.json",
		},
		{
			title:    "empty resource",
			expected: ".",
		},
	} {
		t.Run(tt.title, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.pipeline.Filename())
		})
	}
}

func TestPipelineMarshalJSON(t *testing.T) {
	for _, tt := range []struct {
		title    string
		pipeline Pipeline
		expected string
		isErr    bool
	}{
		{
			title: "JSON source",
			pipeline: Pipeline{
				Format:  "json",
				Content: []byte(`{"foo":["bar"]}`),
			},
			expected: `{"foo":["bar"]}`,
		},
		{
			title: "Yaml source",
			pipeline: Pipeline{
				Format:  "yaml",
				Content: []byte(`foo: ["bar"]`),
			},
			expected: `{"foo":["bar"]}`,
		},
		{
			title: "bad Yaml",
			pipeline: Pipeline{
				Format:  "yaml",
				Content: []byte(`broken"`),
			},
			isErr: true,
		},
	} {
		t.Run(tt.title, func(t *testing.T) {
			got, err := tt.pipeline.MarshalJSON()
			if tt.isErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, []byte(tt.expected), got)
			}
		})
	}
}
