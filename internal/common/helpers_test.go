// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTrimStringSlice(t *testing.T) {
	strs := []string{"foo bar ", "  bar baz", "\tbaz qux\t\t", "qux foo"}
	expected := []string{"foo bar", "bar baz", "baz qux", "qux foo"}

	TrimStringSlice(strs)
	require.Equal(t, expected, strs)
}

func TestStringSlicesUnion(t *testing.T) {
	cases := []struct {
		slices   [][]string
		expected []string
	}{
		{nil, nil},
		{[][]string{{"foo", "bar"}, nil}, []string{"foo", "bar"}},
		{[][]string{nil, {"foo", "bar"}}, []string{"foo", "bar"}},
		{[][]string{{"foo", "bar"}, {"foo", "bar"}}, []string{"foo", "bar"}},
		{[][]string{{"foo", "baz"}, {"foo", "bar"}}, []string{"foo", "bar", "baz"}},
		{[][]string{{"foo", "bar"}, {"foo", "baz"}}, []string{"foo", "bar", "baz"}},
	}

	for _, c := range cases {
		result := StringSlicesUnion(c.slices...)
		assert.ElementsMatch(t, c.expected, result)
	}
}
