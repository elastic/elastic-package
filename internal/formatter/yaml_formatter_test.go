// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package formatter

import (
	"strconv"
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestYAMLFormatterNestedObjects(t *testing.T) {
	cases := []struct {
		doc      string
		expected string
	}{
		{
			doc: `foo.bar: 3`,
			expected: `foo:
  bar: 3
`,
		},
		{
			doc: `foo.bar.baz: 3`,
			expected: `foo:
  bar:
    baz: 3
`,
		},
		{
			doc: `foo:
  bar.baz: 3`,
			expected: `foo:
  bar:
    baz: 3
`,
		},
		{
			doc: `foo.bar.baz: 3
a.b.c: 42`,
			expected: `foo:
  bar:
    baz: 3
a:
  b:
    c: 42
`,
		},
		{
			doc: `foo.bar.baz: 3 # baz
# Mistery of life and everything else.
a.b.c: 42`,
			expected: `foo:
  bar:
    baz: 3 # baz
a:
  b:
    # Mistery of life and everything else.
    c: 42
`,
		},
		{
			doc:      `"foo.bar.baz": 3`,
			expected: "\"foo.bar.baz\": 3\n",
		},
	}

	sv := semver.MustParse("3.0.0")
	formatter := NewYAMLFormatter(*sv).Format

	for i, c := range cases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			result, _, err := formatter([]byte(c.doc))
			require.NoError(t, err)
			assert.Equal(t, c.expected, string(result))
		})
	}

}
