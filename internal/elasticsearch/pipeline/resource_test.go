// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package pipeline

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResourceFileName(t *testing.T) {
	for _, tt := range []struct {
		title    string
		resource Resource
		expected string
	}{
		{
			title: "name with nonce",
			resource: Resource{
				Name:   "default-1234",
				Format: "yml",
			},
			expected: "default.yml",
		},
		{
			title: "name without nonce",
			resource: Resource{
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
			assert.Equal(t, tt.expected, tt.resource.FileName())
		})
	}
}

func TestResourceToJSON(t *testing.T) {
	for _, tt := range []struct {
		title    string
		resource Resource
		expected string
		isErr    bool
	}{
		{
			title: "JSON source",
			resource: Resource{
				Format:  "json",
				Content: []byte(`{"foo":["bar"]}`),
			},
			expected: `{"foo":["bar"]}`,
		},
		{
			title: "Yaml source",
			resource: Resource{
				Format:  "yaml",
				Content: []byte(`foo: ["bar"]`),
			},
			expected: `{"foo":["bar"]}`,
		},
		{
			title: "bad Yaml",
			resource: Resource{
				Format:  "yaml",
				Content: []byte(`broken"`),
			},
			isErr: true,
		},
	} {
		t.Run(tt.title, func(t *testing.T) {
			got, err := tt.resource.ToJSON()
			if tt.isErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, []byte(tt.expected), got)
			}
		})
	}
}
