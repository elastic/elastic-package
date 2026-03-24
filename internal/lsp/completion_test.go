// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package lsp

import (
	"path/filepath"
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
}

func TestCompleteFieldNamesAndManifestKeys(t *testing.T) {
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
	manifestItems := completeManifestKeys(manifestPath, packageRoot, "pol", readTestFile(t, manifestPath))
	require.NotEmpty(t, manifestItems)
	item := findCompletionItem(manifestItems, "policy_templates")
	require.NotNil(t, item)
	require.NotNil(t, item.InsertText)
	assert.Equal(t, "policy_templates: ", *item.InsertText)
	assert.Nil(t, findCompletionItem(manifestItems, "title"))

	assert.Nil(t, completeManifestKeys(manifestPath, packageRoot, "title: Apache", readTestFile(t, manifestPath)))
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
	server.documents.Set(manifestURI, "title")

	result, err := server.textDocumentCompletion(nil, &protocol.CompletionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: manifestURI},
			Position:     protocol.Position{Line: 0, Character: 3},
		},
	})
	require.NoError(t, err)
	manifestItems, ok := result.([]protocol.CompletionItem)
	require.True(t, ok)
	assert.NotNil(t, findCompletionItem(manifestItems, "title"))

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
