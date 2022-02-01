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

func TestStringSliceContains(t *testing.T) {
	cases := []struct {
		slice    []string
		s        string
		expected bool
	}{
		{nil, "", false},
		{nil, "foo", false},
		{[]string{"foo"}, "foo", true},
		{[]string{"foo", "bar"}, "foo", true},
		{[]string{"foo", "bar"}, "bar", true},
		{[]string{"foo", "bar"}, "foobar", false},
		{[]string{"foo", "bar"}, "fo", false},
	}

	for _, c := range cases {
		found := StringSliceContains(c.slice, c.s)
		assert.Equalf(t, c.expected, found, "checking if slice %v contains '%s'", c.slice, c.s)
	}
}
