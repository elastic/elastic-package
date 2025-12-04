// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package filter

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/elastic-package/internal/packages"
)

func TestSplitAndTrim(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		delimiter string
		want      []string
	}{
		{"simple split", "a,b,c", ",", []string{"a", "b", "c"}},
		{"with spaces", " a , b , c ", ",", []string{"a", "b", "c"}},
		{"empty string", "", ",", nil},
		{"only delimiters", ",,", ",", []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitAndTrim(tt.input, tt.delimiter)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestHasAnyMatch(t *testing.T) {
	tests := []struct {
		name    string
		filters []string
		items   []string
		want    bool
	}{
		{"match found", []string{"a"}, []string{"a", "b"}, true},
		{"no match", []string{"c"}, []string{"a", "b"}, false},
		{"empty filters (match all)", []string{}, []string{"a", "b"}, true},
		{"empty items", []string{"a"}, []string{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasAnyMatch(tt.filters, tt.items)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestExtractInputs(t *testing.T) {
	manifest := &packages.PackageManifest{
		PolicyTemplates: []packages.PolicyTemplate{
			{Input: "input1"},
			{Inputs: []packages.Input{{Type: "input2"}, {Type: "input3"}}},
		},
	}

	got := extractInputs(manifest)
	assert.Contains(t, got, "input1")
	assert.Contains(t, got, "input2")
	assert.Contains(t, got, "input3")
	assert.Len(t, got, 3)
}
