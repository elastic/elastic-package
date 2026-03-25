// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package lsp

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	protocol "github.com/tliron/glsp/protocol_3_16"
)

func TestCompletionHelpers(t *testing.T) {
	packageRoot := "/tmp/apache"
	manifestPath := filepath.Join(packageRoot, "manifest.yml")
	dataStreamManifestPath := filepath.Join(packageRoot, "data_stream", "access", "manifest.yml")
	fieldsPath := filepath.Join(packageRoot, "data_stream", "access", "fields", "fields.yml")

	assert.True(t, isFieldValueContext("  field: apache.access.ssl.protocol"))
	assert.True(t, isFieldValueContext("  - field: apache.access.ssl.protocol"))
	assert.False(t, isFieldValueContext("title: Apache"))
	assert.Equal(t, "apache.access.ssl.", extractFieldPrefix("field: apache.access.ssl."))
	assert.Equal(t, "apache.access.ssl.", extractFieldPrefix(`field: "apache.access.ssl.`))
	assert.True(t, isManifestFile(manifestPath, packageRoot))
	assert.True(t, isDataStreamManifest(dataStreamManifestPath, packageRoot))
	assert.True(t, isFieldsDefinitionFile(fieldsPath))
	assert.Equal(t, "access", dataStreamFromPath(fieldsPath, packageRoot))
	assert.Equal(t, "long, unit: byte, metric: counter", formatFieldDetail(FieldInfo{
		Type:       "long",
		Unit:       "byte",
		MetricType: "counter",
	}))
	assert.Equal(t, "ty", extractManifestKeyPrefix("  - ty"))
}

func TestResolveManifestCompletionContext(t *testing.T) {
	documentText, pos := completionDocument(t, `policy_templates:
  |
`)
	context, ok := resolveManifestCompletionContext(documentText, pos)
	require.True(t, ok)
	assert.Equal(t, manifestCompletionModeKey, context.mode)
	assert.Equal(t, "policy_templates", context.path)
	assert.Equal(t, "", context.prefix)
	assert.Equal(t, 2, context.currentIndent)
	assert.Equal(t, 0, context.parentIndent)
	assert.False(t, context.listItemPrefix)

	documentText, pos = completionDocument(t, `policy_templates:
  - name: demo
    inputs:
      - ty|
`)
	context, ok = resolveManifestCompletionContext(documentText, pos)
	require.True(t, ok)
	assert.Equal(t, manifestCompletionModeKey, context.mode)
	assert.Equal(t, "policy_templates.inputs", context.path)
	assert.Equal(t, "ty", context.prefix)
	assert.True(t, context.listItemPrefix)

	documentText, pos = completionDocument(t, `owner:
  type: el|
`)
	context, ok = resolveManifestCompletionContext(documentText, pos)
	require.True(t, ok)
	assert.Equal(t, manifestCompletionModeValue, context.mode)
	assert.Equal(t, "owner.type", context.path)
	assert.Equal(t, "el", context.prefix)
}

func TestCompleteFieldNamesAndManifestItems(t *testing.T) {
	packageRoot := copyFixturePackage(t, fixturePackagePath(t, "test", "packages", "parallel", "apache"))
	server := NewServer()

	accessManifestPath := filepath.Join(packageRoot, "data_stream", "access", "manifest.yml")
	fieldItems := server.completeFieldNames(packageRoot, accessManifestPath, "field: apache.access.ssl.")
	require.NotEmpty(t, fieldItems)
	assert.NotNil(t, findCompletionItem(fieldItems, "apache.access.ssl.protocol"))
	assert.NotNil(t, findCompletionItem(fieldItems, "apache.access.ssl.cipher"))
	assert.NotNil(t, findCompletionItem(server.completeFieldNames(packageRoot, accessManifestPath, "- field: apache.access.ssl."), "apache.access.ssl.protocol"))

	statusManifestPath := filepath.Join(packageRoot, "data_stream", "status", "manifest.yml")
	statusFieldItems := server.completeFieldNames(packageRoot, statusManifestPath, `field: "apache.status.total_`)
	statusItem := findCompletionItem(statusFieldItems, "apache.status.total_bytes")
	require.NotNil(t, statusItem)
	require.NotNil(t, statusItem.Detail)
	assert.Equal(t, "long, unit: byte, metric: counter", *statusItem.Detail)

	manifestPath := filepath.Join(packageRoot, "manifest.yml")
	documentText, pos := completionDocument(t, "pol|")
	manifestItems := completeManifestItems(manifestPath, packageRoot, documentText, pos)
	require.NotEmpty(t, manifestItems)
	item := findCompletionItem(manifestItems, "policy_templates")
	require.NotNil(t, item)
	require.NotNil(t, item.InsertText)
	assert.Equal(t, "policy_templates: ", *item.InsertText)
	assert.Nil(t, findCompletionItem(manifestItems, "title"))

	documentText, pos = completionDocument(t, "title: Apache|")
	assert.Nil(t, completeManifestItems(manifestPath, packageRoot, documentText, pos))
}

func TestCompleteManifestItemsSupportsNestedKeys(t *testing.T) {
	packageRoot := "/tmp/pkg"
	manifestPath := filepath.Join(packageRoot, "manifest.yml")
	dataStreamManifestPath := filepath.Join(packageRoot, "data_stream", "logs", "manifest.yml")

	documentText, pos := completionDocument(t, `type: input
policy_templates:
  |
`)
	items := completeManifestItems(manifestPath, packageRoot, documentText, pos)
	item := findCompletionItem(items, "name")
	require.NotNil(t, item)
	require.NotNil(t, item.InsertText)
	assert.Equal(t, "- name: ", *item.InsertText)

	documentText, pos = completionDocument(t, `type: integration
policy_templates:
  - name: demo
    title: Demo
    description: Demo
    inputs:
      - ty|
`)
	items = completeManifestItems(manifestPath, packageRoot, documentText, pos)
	item = findCompletionItem(items, "type")
	require.NotNil(t, item)
	require.NotNil(t, item.InsertText)
	assert.Equal(t, "type: ", *item.InsertText)

	documentText, pos = completionDocument(t, `type: integration
policy_templates:
  - name: demo
    title: Demo
    description: Demo
    inputs:
      - |
`)
	items = completeManifestItems(manifestPath, packageRoot, documentText, pos)
	assert.NotNil(t, findCompletionItem(items, "vars"))

	documentText, pos = completionDocument(t, `type: input
vars:
  - req|
`)
	items = completeManifestItems(manifestPath, packageRoot, documentText, pos)
	assert.NotNil(t, findCompletionItem(items, "required"))

	documentText, pos = completionDocument(t, `type: input
policy_templates:
  - name: demo
    title: Demo
    description: Demo
    input: otelcol
    vars:
      - sho|
`)
	items = completeManifestItems(manifestPath, packageRoot, documentText, pos)
	assert.NotNil(t, findCompletionItem(items, "show_user"))

	documentText, pos = completionDocument(t, `title: Demo
streams:
  - en|
`)
	items = completeManifestItems(dataStreamManifestPath, packageRoot, documentText, pos)
	assert.NotNil(t, findCompletionItem(items, "enabled"))
}

func TestCompleteManifestItemsSuggestsSchemaValues(t *testing.T) {
	packageRoot := "/tmp/pkg"
	manifestPath := filepath.Join(packageRoot, "manifest.yml")
	dataStreamManifestPath := filepath.Join(packageRoot, "data_stream", "logs", "manifest.yml")

	documentText, pos := completionDocument(t, `owner:
  type: el|
`)
	items := completeManifestItems(manifestPath, packageRoot, documentText, pos)
	assert.NotNil(t, findCompletionItem(items, "elastic"))

	documentText, pos = completionDocument(t, `type: integration
policy_templates:
  - name: demo
  - name: second
policy_templates_behavior: c|
`)
	items = completeManifestItems(manifestPath, packageRoot, documentText, pos)
	assert.NotNil(t, findCompletionItem(items, "combined_policy"))
	assert.Nil(t, findCompletionItem(items, "all"))

	documentText, pos = completionDocument(t, `type: input
vars:
  - type: bo|
`)
	items = completeManifestItems(manifestPath, packageRoot, documentText, pos)
	assert.NotNil(t, findCompletionItem(items, "bool"))

	documentText, pos = completionDocument(t, `type: input
vars:
  - required: f|
`)
	items = completeManifestItems(manifestPath, packageRoot, documentText, pos)
	assert.NotNil(t, findCompletionItem(items, "false"))
	assert.Nil(t, findCompletionItem(items, "true"))

	documentText, pos = completionDocument(t, `title: Demo
streams:
  - enabled: t|
`)
	items = completeManifestItems(dataStreamManifestPath, packageRoot, documentText, pos)
	assert.NotNil(t, findCompletionItem(items, "true"))
}

func TestCompleteFieldTypeValuesSuggestsKnownTypes(t *testing.T) {
	items := completeFieldTypeValues("type: k")

	require.NotEmpty(t, items)
	assert.NotNil(t, findCompletionItem(items, "keyword"))
	assert.Nil(t, findCompletionItem(items, "boolean"))
	assert.NotNil(t, findCompletionItem(completeFieldTypeValues(`type: "ke`), "keyword"))
	assert.Nil(t, completeFieldTypeValues("name: apache"))
}

func TestTextDocumentCompletionUsesOpenBufferContent(t *testing.T) {
	packageRoot := copyFixturePackage(t, fixturePackagePath(t, "test", "packages", "parallel", "apache"))
	server := NewServer()

	manifestPath := filepath.Join(packageRoot, "manifest.yml")
	manifestURI := protocol.DocumentUri(pathToURI(manifestPath))
	server.documents.Set(manifestURI, "type: input\nvars:\n  - req")

	result, err := server.textDocumentCompletion(nil, &protocol.CompletionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: manifestURI},
			Position:     protocol.Position{Line: 2, Character: 7},
		},
	})
	require.NoError(t, err)
	manifestItems, ok := result.([]protocol.CompletionItem)
	require.True(t, ok)
	assert.NotNil(t, findCompletionItem(manifestItems, "required"))

	fieldsPath := filepath.Join(packageRoot, "data_stream", "access", "fields", "tmp.yml")
	fieldsURI := protocol.DocumentUri(pathToURI(fieldsPath))
	server.documents.Set(fieldsURI, "type: ")

	result, err = server.textDocumentCompletion(nil, &protocol.CompletionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: fieldsURI},
			Position:     protocol.Position{Line: 0, Character: 6},
		},
	})
	require.NoError(t, err)
	typeItems, ok := result.([]protocol.CompletionItem)
	require.True(t, ok)
	assert.NotNil(t, findCompletionItem(typeItems, "keyword"))
}

func findCompletionItem(items []protocol.CompletionItem, label string) *protocol.CompletionItem {
	for i := range items {
		if items[i].Label == label {
			return &items[i]
		}
	}
	return nil
}

func completionDocument(t *testing.T, marked string) (string, protocol.Position) {
	t.Helper()

	lines := strings.Split(marked, "\n")
	for lineNum, line := range lines {
		if idx := strings.IndexRune(line, '|'); idx >= 0 {
			lines[lineNum] = strings.Replace(line, "|", "", 1)
			return strings.Join(lines, "\n"), protocol.Position{
				Line:      uint32(lineNum),
				Character: uint32(utf16Column(line[:idx])),
			}
		}
	}

	t.Fatal("missing cursor marker")
	return "", protocol.Position{}
}

func utf16Column(s string) int {
	column := 0
	for _, r := range s {
		column += utf16Width(r)
	}
	return column
}
