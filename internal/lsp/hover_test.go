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

func TestHoverFieldReferenceAndFormatting(t *testing.T) {
	packageRoot := copyFixturePackage(t, fixturePackagePath(t, "test", "packages", "parallel", "apache"))
	filePath := filepath.Join(packageRoot, "data_stream", "access", "manifest.yml")
	line := `field: "apache.access.ssl.protocol"`

	md := hoverFieldReference(line, protocol.Position{Line: 0, Character: 12}, packageRoot, filePath)
	require.NotEmpty(t, md)
	assert.Contains(t, md, "apache.access.ssl.protocol")
	assert.Contains(t, md, "SSL protocol version")
	assert.Empty(t, hoverFieldReference(line, protocol.Position{Line: 0, Character: 2}, packageRoot, filePath))
	assert.Equal(t, "apache.access.ssl.protocol", extractFieldValueAtCursor(line, protocol.Position{Line: 0, Character: 12}))
	assert.Empty(t, extractFieldValueAtCursor(line, protocol.Position{Line: 0, Character: 2}))
}

func TestHoverManifestKeyAndFieldDefinitions(t *testing.T) {
	packageRoot := copyFixturePackage(t, fixturePackagePath(t, "test", "packages", "parallel", "apache"))
	manifestPath := filepath.Join(packageRoot, "manifest.yml")
	manifestText := readTestFile(t, manifestPath)
	line := getLineAtText(manifestText, 2)

	md := hoverManifestKey(line, protocol.Position{Line: 2, Character: 1}, manifestPath, packageRoot, manifestText)
	require.NotEmpty(t, md)
	assert.Contains(t, md, "**title**")
	assert.Empty(t, hoverManifestKey(line, protocol.Position{Line: 2, Character: 7}, manifestPath, packageRoot, manifestText))

	fieldsPath := filepath.Join(packageRoot, "data_stream", "status", "fields", "fields.yml")
	assert.Contains(t, hoverFieldDefinition("type: keyword", protocol.Position{Line: 0, Character: 8}, fieldsPath), "Exact-value string")
	assert.Contains(t, hoverFieldDefinition("unit: byte", protocol.Position{Line: 0, Character: 7}, fieldsPath), "Data size in bytes")
	assert.Contains(t, hoverFieldDefinition("metric_type: counter", protocol.Position{Line: 0, Character: 15}, fieldsPath), "cumulative metric")
	assert.Empty(t, hoverFieldDefinition("type: keyword", protocol.Position{Line: 0, Character: 2}, fieldsPath))
	assert.Equal(t, "type", extractYAMLKey("  type: text"))
}

func TestTextDocumentHoverUsesOpenBufferContent(t *testing.T) {
	packageRoot := copyFixturePackage(t, fixturePackagePath(t, "test", "packages", "parallel", "apache"))
	server := NewServer()

	manifestPath := filepath.Join(packageRoot, "manifest.yml")
	manifestText := readTestFile(t, manifestPath)
	manifestURI := protocol.DocumentUri(pathToURI(manifestPath))
	server.documents.Set(manifestURI, manifestText)

	hover, err := server.textDocumentHover(nil, &protocol.HoverParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: manifestURI},
			Position:     protocol.Position{Line: 2, Character: 1},
		},
	})
	require.NoError(t, err)
	require.NotNil(t, hover)

	content, ok := hover.Contents.(protocol.MarkupContent)
	require.True(t, ok)
	assert.Contains(t, content.Value, "**title**")
}
