// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package lsp

import (
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
