// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package yamledit

import (
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mustNewDocumentFile(filename string) *Document {
	d, err := NewDocumentFile(filename)
	if err != nil {
		panic(err)
	}

	return d
}

func mustYAMLPathString(path string) *yaml.Path {
	p, err := yaml.PathString(path)
	if err != nil {
		panic(err)
	}

	return p
}

func mustNodeFromString(s string) ast.Node {
	f, err := parser.ParseBytes([]byte(s), parser.ParseComments)
	if err != nil {
		panic(err)
	}

	return f.Docs[0].Body
}

func TestDocument_GetNode(t *testing.T) {
	testCases := []struct {
		name    string
		doc     *Document
		path    string
		wantErr bool
	}{
		{
			name: "ok",
			doc:  mustNewDocumentFile("testdata/valid.yml"),
			path: "$.string",
		},
		{
			name:    "not-found",
			doc:     mustNewDocumentFile("testdata/valid.yml"),
			path:    "$.missing",
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.doc.GetNode(tc.path)

			if tc.wantErr {
				assert.Error(t, err)
			} else {
				want, lookupErr := mustYAMLPathString(tc.path).FilterFile(tc.doc.f)
				require.NoError(t, lookupErr)

				assert.NoError(t, err)
				assert.Equal(t, want, got)
			}
		})
	}
}

func TestDocument_GetMappingNode(t *testing.T) {
	testCases := []struct {
		name    string
		doc     *Document
		path    string
		wantErr bool
	}{
		{
			name: "ok",
			doc:  mustNewDocumentFile("testdata/valid.yml"),
			path: "$.map",
		},
		{
			name:    "wrong-type",
			doc:     mustNewDocumentFile("testdata/valid.yml"),
			path:    "$.list",
			wantErr: true,
		},
		{
			name:    "not-found",
			doc:     mustNewDocumentFile("testdata/valid.yml"),
			path:    "$.missing",
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.doc.GetMappingNode(tc.path)

			if tc.wantErr {
				assert.Error(t, err)
			} else {
				wantRaw, lookupErr := mustYAMLPathString(tc.path).FilterFile(tc.doc.f)
				require.NoError(t, lookupErr)
				want, ok := wantRaw.(*ast.MappingNode)
				require.True(t, ok)
				require.NotNil(t, want)

				assert.NoError(t, err)
				assert.Equal(t, want, got)
			}
		})
	}
}

func TestDocument_GetSequenceNode(t *testing.T) {
	testCases := []struct {
		name    string
		doc     *Document
		path    string
		wantErr bool
	}{
		{
			name: "ok",
			doc:  mustNewDocumentFile("testdata/valid.yml"),
			path: "$.list",
		},
		{
			name:    "wrong-type",
			doc:     mustNewDocumentFile("testdata/valid.yml"),
			path:    "$.map",
			wantErr: true,
		},
		{
			name:    "not-found",
			doc:     mustNewDocumentFile("testdata/valid.yml"),
			path:    "$.missing",
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.doc.GetSequenceNode(tc.path)

			if tc.wantErr {
				assert.Error(t, err)
			} else {
				wantRaw, lookupErr := mustYAMLPathString(tc.path).FilterFile(tc.doc.f)
				require.NoError(t, lookupErr)
				want, ok := wantRaw.(*ast.SequenceNode)
				require.True(t, ok)
				require.NotNil(t, want)

				assert.NoError(t, err)
				assert.Equal(t, want, got)
			}
		})
	}
}

func TestDocument_GetParentNode(t *testing.T) {
	testCases := []struct {
		name     string
		doc      *Document
		path     string
		wantNode *yaml.Path
		wantErr  bool
	}{
		{
			name:     "ok-mapping",
			doc:      mustNewDocumentFile("testdata/valid.yml"),
			path:     "$.string",
			wantNode: mustYAMLPathString("$"),
		},
		{
			name:     "ok-sequence",
			doc:      mustNewDocumentFile("testdata/valid.yml"),
			path:     "$.list[1]",
			wantNode: mustYAMLPathString("$.list"),
		},
		{
			name: "ok-root",
			doc:  mustNewDocumentFile("testdata/valid.yml"),
			path: "$",
		},
		{
			name:     "ok-not-found-parent-exists",
			doc:      mustNewDocumentFile("testdata/valid.yml"),
			path:     "$.missing",
			wantNode: mustYAMLPathString("$"),
		},
		{
			name:    "bad-not-found",
			doc:     mustNewDocumentFile("testdata/valid.yml"),
			path:    "$.missing.gone",
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.doc.GetParentNode(tc.path)

			if tc.wantErr {
				assert.Error(t, err)
			} else {
				var want ast.Node
				if tc.wantNode != nil {
					want, err = tc.wantNode.FilterFile(tc.doc.f)
					require.NoError(t, err)
					require.NotNil(t, want)
				}

				assert.NoError(t, err)
				assert.Equal(t, want, got)
			}
		})
	}
}

func TestDocument_AddValue(t *testing.T) {
	testCases := []struct {
		name    string
		doc     *Document
		path    string
		value   any
		index   int
		replace bool
		want    ast.Node
		wantMod bool
		wantErr bool
	}{
		{
			name:    "ok-append",
			doc:     mustNewDocumentFile("testdata/valid.yml"),
			path:    "$.list",
			index:   IndexAppend,
			value:   "four",
			want:    mustNodeFromString("[one, two, three, four]"),
			wantMod: true,
		},
		{
			name:    "ok-prepend",
			doc:     mustNewDocumentFile("testdata/valid.yml"),
			path:    "$.list",
			index:   IndexPrepend,
			value:   "four",
			want:    mustNodeFromString("[four, one, two, three]"),
			wantMod: true,
		},
		{
			name:    "ok-replace",
			doc:     mustNewDocumentFile("testdata/valid.yml"),
			path:    "$.list",
			index:   1,
			replace: true,
			value:   "four",
			want:    mustNodeFromString("[one, four, three]"),
			wantMod: true,
		},
		{
			name:    "ok-replace-equal",
			doc:     mustNewDocumentFile("testdata/valid.yml"),
			path:    "$.list",
			index:   1,
			replace: true,
			value:   "two",
			want:    mustNodeFromString("[one, two, three]"),
		},
		{
			name:    "bad-not-a-sequence",
			doc:     mustNewDocumentFile("testdata/valid.yml"),
			path:    "$.map",
			index:   1,
			replace: true,
			value:   "two",
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			gotMod, gotErr := tc.doc.AddValue(tc.path, tc.value, tc.index, tc.replace)

			if tc.wantErr {
				assert.Error(t, gotErr)
				assert.False(t, gotMod)
				assert.False(t, tc.doc.Modified())
			} else {
				assert.NoError(t, gotErr)
				assert.Equal(t, tc.wantMod, gotMod)
				assert.Equal(t, tc.wantMod, tc.doc.Modified())

				gotNode, lookupErr := mustYAMLPathString(tc.path).FilterFile(tc.doc.f)
				require.NoError(t, lookupErr)
				require.NotNil(t, gotNode)

				var gotV any
				require.NoError(t, yaml.NodeToValue(gotNode, &gotV))
				var wantV any
				require.NoError(t, yaml.NodeToValue(tc.want, &wantV))

				assert.Equal(t, wantV, gotV)
			}
		})
	}
}

func TestDocument_PrependValue(t *testing.T) {
	testCases := []struct {
		name    string
		doc     *Document
		path    string
		value   any
		want    ast.Node
		wantMod bool
		wantErr bool
	}{
		{
			name:    "ok",
			doc:     mustNewDocumentFile("testdata/valid.yml"),
			path:    "$.list",
			value:   "four",
			want:    mustNodeFromString("[four, one, two, three]"),
			wantMod: true,
		},
		{
			name:    "bad-not-a-sequence",
			doc:     mustNewDocumentFile("testdata/valid.yml"),
			path:    "$.map",
			value:   "two",
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			gotMod, gotErr := tc.doc.PrependValue(tc.path, tc.value)

			if tc.wantErr {
				assert.Error(t, gotErr)
				assert.False(t, gotMod)
				assert.False(t, tc.doc.Modified())
			} else {
				assert.NoError(t, gotErr)
				assert.Equal(t, tc.wantMod, gotMod)
				assert.Equal(t, tc.wantMod, tc.doc.Modified())

				gotNode, lookupErr := mustYAMLPathString(tc.path).FilterFile(tc.doc.f)
				require.NoError(t, lookupErr)
				require.NotNil(t, gotNode)

				var gotV any
				require.NoError(t, yaml.NodeToValue(gotNode, &gotV))
				var wantV any
				require.NoError(t, yaml.NodeToValue(tc.want, &wantV))

				assert.Equal(t, wantV, gotV)
			}
		})
	}
}

func TestDocument_AppendValue(t *testing.T) {
	testCases := []struct {
		name    string
		doc     *Document
		path    string
		value   any
		want    ast.Node
		wantMod bool
		wantErr bool
	}{
		{
			name:    "ok",
			doc:     mustNewDocumentFile("testdata/valid.yml"),
			path:    "$.list",
			value:   "four",
			want:    mustNodeFromString("[one, two, three, four]"),
			wantMod: true,
		},
		{
			name:    "bad-not-a-sequence",
			doc:     mustNewDocumentFile("testdata/valid.yml"),
			path:    "$.map",
			value:   "two",
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			gotMod, gotErr := tc.doc.AppendValue(tc.path, tc.value)

			if tc.wantErr {
				assert.Error(t, gotErr)
				assert.False(t, gotMod)
				assert.False(t, tc.doc.Modified())
			} else {
				assert.NoError(t, gotErr)
				assert.Equal(t, tc.wantMod, gotMod)
				assert.Equal(t, tc.wantMod, tc.doc.Modified())

				gotNode, lookupErr := mustYAMLPathString(tc.path).FilterFile(tc.doc.f)
				require.NoError(t, lookupErr)
				require.NotNil(t, gotNode)

				var gotV any
				require.NoError(t, yaml.NodeToValue(gotNode, &gotV))
				var wantV any
				require.NoError(t, yaml.NodeToValue(tc.want, &wantV))

				assert.Equal(t, wantV, gotV)
			}
		})
	}
}

func TestDocument_SetKeyValue(t *testing.T) {
	testCases := []struct {
		name    string
		doc     *Document
		path    string
		key     string
		value   any
		index   int
		wantMod bool
		wantErr bool
	}{
		{
			name:    "ok-append-new",
			doc:     mustNewDocumentFile("testdata/valid.yml"),
			path:    "$.map",
			key:     "new_item",
			index:   IndexAppend,
			value:   "foobar",
			wantMod: true,
		},
		{
			name:    "ok-prepend-new",
			doc:     mustNewDocumentFile("testdata/valid.yml"),
			path:    "$.map",
			key:     "new_item",
			index:   IndexPrepend,
			value:   "foobar",
			wantMod: true,
		},
		{
			name:    "ok-prepend-new",
			doc:     mustNewDocumentFile("testdata/valid.yml"),
			path:    "$.map",
			key:     "string",
			index:   IndexAppend,
			value:   "foobar",
			wantMod: true,
		},
		{
			name:    "bad-not-a-mapping",
			doc:     mustNewDocumentFile("testdata/valid.yml"),
			path:    "$.list",
			index:   IndexAppend,
			value:   "foobar",
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			gotMod, gotErr := tc.doc.SetKeyValue(tc.path, tc.key, tc.value, tc.index)

			if tc.wantErr {
				assert.Error(t, gotErr)
				assert.False(t, gotMod)
				assert.False(t, tc.doc.Modified())
			} else {
				assert.NoError(t, gotErr)
				assert.Equal(t, tc.wantMod, gotMod)
				assert.Equal(t, tc.wantMod, tc.doc.Modified())

				gotNode, lookupErr := mustYAMLPathString(tc.path + "." + tc.key).FilterFile(tc.doc.f)
				require.NoError(t, lookupErr)
				require.NotNil(t, gotNode)

				var gotV any
				require.NoError(t, yaml.NodeToValue(gotNode, &gotV))

				assert.Equal(t, tc.value, gotV)
			}
		})
	}
}

func TestDocument_DeleteNode(t *testing.T) {
	testCases := []struct {
		name    string
		doc     *Document
		path    string
		wantMod bool
		wantErr bool
	}{
		{
			name:    "ok-sequence",
			doc:     mustNewDocumentFile("testdata/valid.yml"),
			path:    "$.list[1]",
			wantMod: true,
		},
		{
			name:    "ok-mapping",
			doc:     mustNewDocumentFile("testdata/valid.yml"),
			path:    "$.map",
			wantMod: true,
		},
		{
			name:    "bad-missing-sequence",
			doc:     mustNewDocumentFile("testdata/valid.yml"),
			path:    "$.list[4]",
			wantErr: true,
		},
		{
			name:    "bad-missing-mapping",
			doc:     mustNewDocumentFile("testdata/valid.yml"),
			path:    "$.map.missing",
			wantErr: true,
		},
		{
			name:    "bad-invalid-path",
			doc:     mustNewDocumentFile("testdata/valid.yml"),
			path:    ".map.missing",
			wantErr: true,
		},
		{
			name:    "bad-root",
			doc:     mustNewDocumentFile("testdata/valid.yml"),
			path:    "$",
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			gotMod, gotErr := tc.doc.DeleteNode(tc.path)

			if tc.wantErr {
				assert.Error(t, gotErr)
				assert.False(t, gotMod)
				assert.False(t, tc.doc.Modified())
			} else {
				assert.NoError(t, gotErr)
				assert.Equal(t, tc.wantMod, gotMod)
				assert.Equal(t, tc.wantMod, tc.doc.Modified())
			}
		})
	}
}

func TestNewDocumentFile(t *testing.T) {
	testCases := []struct {
		name     string
		filename string
		wantErr  bool
	}{
		{
			name:     "ok",
			filename: "testdata/valid.yml",
		},
		{
			name:     "bad",
			filename: "testdata/invalid.yml",
			wantErr:  true,
		},
		{
			name:     "missing",
			filename: "testdata/missing.yml",
			wantErr:  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NewDocumentFile(tc.filename)

			if tc.wantErr {
				assert.Error(t, err)
				assert.Nil(t, got)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, got)
				assert.Equal(t, tc.filename, got.Filename())
			}
		})
	}
}

func TestParseDocumentFile(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		type TestStruct struct {
			String string         `yaml:"string"`
			Int    int            `yaml:"int"`
			Bool   bool           `yaml:"bool"`
			List   []string       `yaml:"list"`
			Map    map[string]any `yaml:"map"`
		}

		want := TestStruct{
			String: "value",
			Int:    1,
			Bool:   true,
			List:   []string{"one", "two", "three"},
			Map: map[string]any{
				"string": "value",
				"int":    uint64(1),
				"bool":   true,
				"list":   []any{"one", "two", "three"},
				"map": map[string]any{
					"string": "value",
					"int":    uint64(1),
					"bool":   true,
					"list":   []any{"one", "two", "three"},
				},
			},
		}

		var v TestStruct
		d, err := ParseDocumentFile("testdata/valid.yml", &v)

		require.NoError(t, err)
		require.NotNil(t, d)

		assert.Equal(t, want, v)
	})

	t.Run("bad-file", func(t *testing.T) {
		type TestStruct struct {
			String int `yaml:"string"`
		}

		var v TestStruct
		_, err := ParseDocumentFile("testdata/bad.yml", &v)

		require.Error(t, err)
	})

	t.Run("bad-parse", func(t *testing.T) {
		type TestStruct struct {
			String int `yaml:"string"`
		}

		var v TestStruct
		_, err := ParseDocumentFile("testdata/valid.yml", &v)

		require.Error(t, err)
	})
}

func TestNewDocumentBytes(t *testing.T) {
	testCases := []struct {
		name    string
		in      []byte
		wantErr bool
	}{
		{
			name: "ok",
			in: []byte(`---
string: value
int: 1
bool: true
list:
  - one
  - two
  - three
map:
  string: value
  int: 1
  bool: true
  list:
    - one
    - two
    - three
  map:
    string: value
    int: 1
    bool: true
    list:
      - one
      - two
      - three
`),
		},
		{
			name:    "bad",
			in:      []byte(`:`),
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NewDocumentBytes(tc.in)

			if tc.wantErr {
				assert.Error(t, err)
				assert.Nil(t, got)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, got)
			}
		})
	}
}

func TestParseDocumentBytes(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		type TestStruct struct {
			String string         `yaml:"string"`
			Int    int            `yaml:"int"`
			Bool   bool           `yaml:"bool"`
			List   []string       `yaml:"list"`
			Map    map[string]any `yaml:"map"`
		}

		want := TestStruct{
			String: "value",
			Int:    1,
			Bool:   true,
			List:   []string{"one", "two", "three"},
			Map: map[string]any{
				"string": "value",
				"int":    uint64(1),
				"bool":   true,
				"list":   []any{"one", "two", "three"},
				"map": map[string]any{
					"string": "value",
					"int":    uint64(1),
					"bool":   true,
					"list":   []any{"one", "two", "three"},
				},
			},
		}

		var v TestStruct
		d, err := ParseDocumentBytes([]byte(`---
string: value
int: 1
bool: true
list:
  - one
  - two
  - three
map:
  string: value
  int: 1
  bool: true
  list:
    - one
    - two
    - three
  map:
    string: value
    int: 1
    bool: true
    list:
      - one
      - two
      - three
`), &v)

		require.NoError(t, err)
		require.NotNil(t, d)

		assert.Equal(t, want, v)
	})

	t.Run("bad-file", func(t *testing.T) {
		type TestStruct struct {
			String int `yaml:"string"`
		}

		var v TestStruct
		_, err := ParseDocumentBytes([]byte(`:`), &v)

		require.Error(t, err)
	})

	t.Run("bad-parse", func(t *testing.T) {
		type TestStruct struct {
			String int `yaml:"string"`
		}

		var v TestStruct
		_, err := ParseDocumentBytes([]byte(`---
string: value
`), &v)

		require.Error(t, err)
	})
}

func Test_getPathIndex(t *testing.T) {
	testCases := []struct {
		name string
		path string
		want int
	}{
		{
			name: "single-index",
			path: "$.test[1]",
			want: 1,
		},
		{
			name: "multiple-indices",
			path: "$.test[1].attributes[3]",
			want: 3,
		},
		{
			name: "bad-malformed-index",
			path: "$.test[1",
			want: -1,
		},
		{
			name: "bad-no-index",
			path: "$.test",
			want: -1,
		},
		{
			name: "bad-index-attribute",
			path: "$.test[1].attribute",
			want: -1,
		},
		{
			name: "bad-index-all",
			path: "$.test[*]",
			want: -1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := getPathIndex(tc.path)

			assert.Equal(t, tc.want, got)
		})
	}
}

func Test_cutPath(t *testing.T) {
	testCases := []struct {
		name       string
		path       string
		wantBefore string
		wantAfter  string
		wantErr    bool
	}{
		{
			name:       "ok",
			path:       "$.test.attribute",
			wantBefore: "$.test",
			wantAfter:  "attribute",
		},
		{
			name:    "bad-split-root",
			path:    "$",
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			gotBefore, gotAfter, err := cutPath(tc.path)

			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.wantBefore, gotBefore)
				assert.Equal(t, tc.wantAfter, gotAfter)
			}
		})
	}
}

func Test_nodeEqual(t *testing.T) {
	t.Run("equal", func(t *testing.T) {
		a, err := parser.ParseBytes([]byte(`string: "foobar"`), parser.ParseComments)
		require.NoError(t, err)
		b, err := parser.ParseBytes([]byte(`string: "foobar"`), parser.ParseComments)
		require.NoError(t, err)

		assert.True(t, nodeEqual(a.Docs[0].Body, b.Docs[0].Body))
	})
	t.Run("not-equal", func(t *testing.T) {
		a, err := parser.ParseBytes([]byte(`string: "foo"`), parser.ParseComments)
		require.NoError(t, err)
		b, err := parser.ParseBytes([]byte(`string: "bar"`), parser.ParseComments)
		require.NoError(t, err)

		assert.False(t, nodeEqual(a.Docs[0].Body, b.Docs[0].Body))
	})
}
