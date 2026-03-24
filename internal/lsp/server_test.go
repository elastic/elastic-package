// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package lsp

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tliron/glsp"
	protocol "github.com/tliron/glsp/protocol_3_16"
)

func TestServerInitializeAndLifecycle(t *testing.T) {
	server := NewServer()

	result, err := server.initialize(&glsp.Context{
		Notify: func(method string, params any) {},
	}, &protocol.InitializeParams{})
	require.NoError(t, err)

	initializeResult, ok := result.(protocol.InitializeResult)
	require.True(t, ok)
	require.NotNil(t, initializeResult.Capabilities.TextDocumentSync)

	assert.True(t, initializeResult.Capabilities.HoverProvider.(bool))
	assert.NotNil(t, initializeResult.Capabilities.CompletionProvider)
	assert.NotNil(t, server.notify)
	assert.NoError(t, server.initialized(nil, &protocol.InitializedParams{}))
	assert.NoError(t, server.setTrace(nil, &protocol.SetTraceParams{Value: protocol.TraceValueMessage}))
	assert.NoError(t, server.shutdown(nil))
}

func TestServerOpenAndSaveHandlers(t *testing.T) {
	packageRoot := copyFixturePackage(t, fixturePackagePath(t, "test", "packages", "parallel", "apache"))
	manifestPath := filepath.Join(packageRoot, "manifest.yml")
	manifestURI := protocol.DocumentUri(pathToURI(manifestPath))

	server := NewServer()
	server.notifyMu.Lock()
	server.notify = func(method string, params any) {}
	server.notifyMu.Unlock()
	t.Cleanup(server.debouncer.Shutdown)

	err := server.textDocumentDidOpen(nil, &protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI:  manifestURI,
			Text: "title",
		},
	})
	require.NoError(t, err)

	text, ok := server.documents.Text(manifestPath)
	require.True(t, ok)
	assert.Equal(t, "title", text)

	err = server.textDocumentDidSave(nil, &protocol.DidSaveTextDocumentParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: manifestURI},
	})
	require.NoError(t, err)
}
