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
		title  string
		doc    string
		nested string
		quoted string
	}{
		{
			title: "one-level nested setting",
			doc:   `foo.bar: 3`,
			nested: `foo:
  bar: 3
`,
			quoted: "'foo.bar': 3\n",
		},
		{
			title: "two-level nested setting",
			doc:   `foo.bar.baz: 3`,
			nested: `foo:
  bar:
    baz: 3
`,
			quoted: "'foo.bar.baz': 3\n",
		},
		{
			title: "nested setting at second level",
			doc: `foo:
  bar.baz: 3`,
			nested: `foo:
  bar:
    baz: 3
`,
			quoted: `foo:
  'bar.baz': 3
`,
		},
		{
			title: "two two-level nested settings",
			doc: `foo.bar.baz: 3
a.b.c: 42`,
			nested: `foo:
  bar:
    baz: 3
a:
  b:
    c: 42
`,
			quoted: `'foo.bar.baz': 3
'a.b.c': 42
`,
		},
		{
			title: "keep comments with the leaf value",
			doc: `foo.bar.baz: 3 # baz
# Mistery of life and everything else.
a.b.c: 42`,
			nested: `foo:
  bar:
    baz: 3 # baz
a:
  b:
    # Mistery of life and everything else.
    c: 42
`,
			quoted: `'foo.bar.baz': 3 # baz
# Mistery of life and everything else.
'a.b.c': 42
`,
		},
		{
			title:  "keep double-quoted keys",
			doc:    `"foo.bar.baz": 3`,
			nested: "\"foo.bar.baz\": 3\n",
			quoted: "\"foo.bar.baz\": 3\n",
		},
		{
			title:  "keep single-quoted keys",
			doc:    `'foo.bar.baz': 3`,
			nested: "'foo.bar.baz': 3\n",
			quoted: "'foo.bar.baz': 3\n",
		},
		{
			title: "array of maps",
			doc: `foo:
  - foo.bar: 1
  - foo.bar: 2`,
			nested: `foo:
  - foo:
      bar: 1
  - foo:
      bar: 2
`,
			quoted: `foo:
  - 'foo.bar': 1
  - 'foo.bar': 2
`,
		},
		{
			title: "merge keys",
			doc: `es.something: true
es.other.thing: false
es.other.level: 13`,
			nested: `es:
  something: true
  other:
    thing: false
    level: 13
`,
			quoted: `'es.something': true
'es.other.thing': false
'es.other.level': 13
`,
		},
	}

	for _, c := range cases {
		t.Run(c.title, func(t *testing.T) {
			t.Run("nested", func(t *testing.T) {
				formatter := NewYAMLFormatter(KeysWithDotActionNested).Format
				result, _, err := formatter([]byte(c.doc))
				require.NoError(t, err)
				assert.Equal(t, c.nested, string(result))
			})

			t.Run("quoted", func(t *testing.T) {
				formatter := NewYAMLFormatter(KeysWithDotActionQuote).Format
				result, _, err := formatter([]byte(c.doc))
				require.NoError(t, err)
				assert.Equal(t, c.quoted, string(result))
			})
		})
	}

}
