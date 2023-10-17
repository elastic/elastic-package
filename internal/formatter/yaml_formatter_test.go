// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package formatter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestYAMLFormatterNestedObjects(t *testing.T) {
	cases := []struct {
		title    string
		doc      string
		expected string
	}{
		{
			title: "one-level nested setting",
			doc:   `foo.bar: 3`,
			expected: `foo:
  bar: 3
`,
		},
		{
			title: "two-level nested setting",
			doc:   `foo.bar.baz: 3`,
			expected: `foo:
  bar:
    baz: 3
`,
		},
		{
			title: "nested setting at second level",
			doc: `foo:
  bar.baz: 3`,
			expected: `foo:
  bar:
    baz: 3
`,
		},
		{
			title: "two two-level nested settings",
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
			title: "keep comments with the leaf value",
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
			title:    "keep double-quoted keys",
			doc:      `"foo.bar.baz": 3`,
			expected: "\"foo.bar.baz\": 3\n",
		},
		{
			title:    "keep single-quoted keys",
			doc:      `"foo.bar.baz": 3`,
			expected: "\"foo.bar.baz\": 3\n",
		},
		{
			title: "array of maps",
			doc: `foo:
  - foo.bar: 1
  - foo.bar: 2`,
			expected: `foo:
  - foo:
      bar: 1
  - foo:
      bar: 2
`,
		},
		{
			title: "merge keys",
			doc: `es.something: true
es.other.thing: false
es.other.level: 13`,
			expected: `es:
  something: true
  other:
    thing: false
    level: 13
`,
		},
	}

	formatter := NewYAMLFormatter(KeysWithDotActionNested).Format
	for _, c := range cases {
		t.Run(c.title, func(t *testing.T) {
			result, _, err := formatter([]byte(c.doc))
			require.NoError(t, err)
			assert.Equal(t, c.expected, string(result))
		})
	}

}
