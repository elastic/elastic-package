// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package lsp

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	protocol "github.com/tliron/glsp/protocol_3_16"
)

func TestDocumentStoreTracksLatestOpenBufferText(t *testing.T) {
	store := newDocumentStore()
	uri := protocol.DocumentUri("file:///tmp/manifest.yml")

	store.Set(uri, "name: old")
	store.Update(uri, []any{
		protocol.TextDocumentContentChangeEventWhole{Text: "name: new"},
	})

	text, ok := store.Text("/tmp/manifest.yml")
	require.True(t, ok)
	assert.Equal(t, "name: new", text)

	store.Delete(uri)
	_, ok = store.Text("/tmp/manifest.yml")
	assert.False(t, ok)
}

func TestDocumentStoreAppliesIncrementalChanges(t *testing.T) {
	store := newDocumentStore()
	uri := protocol.DocumentUri("file:///tmp/manifest.yml")

	store.Set(uri, "title: old")
	store.Update(uri, []any{
		protocol.TextDocumentContentChangeEvent{
			Range: &protocol.Range{
				Start: protocol.Position{Line: 0, Character: 7},
				End:   protocol.Position{Line: 0, Character: 10},
			},
			Text: "new",
		},
	})

	text, ok := store.Text("/tmp/manifest.yml")
	require.True(t, ok)
	assert.Equal(t, "title: new", text)
}

func TestDocumentTextPrefersOpenBufferAndFallsBackToDisk(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "manifest.yml")
	require.NoError(t, os.WriteFile(filePath, []byte("title: disk"), 0o644))

	server := NewServer()
	uri := protocol.DocumentUri(pathToURI(filePath))

	assert.Equal(t, "title: disk", server.documentText(filePath))

	server.documents.Set(uri, "title: buffer")
	assert.Equal(t, "title: buffer", server.documentText(filePath))
}

func TestTextOffsetHelpersHandleUTF16Columns(t *testing.T) {
	assert.Equal(t, 2, utf16ColumnToRuneOffset("a😀b", 3))
	assert.Equal(t, 4, positionOffset("x\na😀b", protocol.Position{Line: 1, Character: 3}))
	assert.Equal(t, "aXb", applyTextChange("a😀b", protocol.Range{
		Start: protocol.Position{Line: 0, Character: 1},
		End:   protocol.Position{Line: 0, Character: 3},
	}, "X"))
	assert.Equal(t, 2, utf16Width('😀'))
}
