// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package lsp

import (
	"sync"

	"github.com/tliron/commonlog"
	"github.com/tliron/glsp"
	protocol "github.com/tliron/glsp/protocol_3_16"
	"github.com/tliron/glsp/server"
)

const serverName = "elastic-package-lsp"

var version = "0.1.0"

// notifyFunc is a function that sends a notification to the client.
type notifyFunc = glsp.NotifyFunc

// Server is the elastic-package LSP server.
type Server struct {
	handler   protocol.Handler
	server    *server.Server
	debouncer *Debouncer
	documents *documentStore
	logger    commonlog.Logger

	// notify is captured during initialize and used for async notifications.
	notifyMu sync.Mutex
	notify   notifyFunc

	// prevDiagFiles tracks which files had diagnostics published in the last
	// validation run per package root, so we can clear them when errors are fixed.
	prevDiagFilesMu sync.Mutex
	prevDiagFiles   map[string]map[string]struct{} // packageRoot -> set of filePaths
}

// NewServer creates a new LSP server.
func NewServer() *Server {
	s := &Server{
		debouncer:     NewDebouncer(),
		documents:     newDocumentStore(),
		logger:        commonlog.GetLogger(serverName),
		prevDiagFiles: make(map[string]map[string]struct{}),
	}

	s.handler = protocol.Handler{
		Initialize:             s.initialize,
		Initialized:            s.initialized,
		Shutdown:               s.shutdown,
		SetTrace:               s.setTrace,
		TextDocumentDidOpen:    s.textDocumentDidOpen,
		TextDocumentDidChange:  s.textDocumentDidChange,
		TextDocumentDidSave:    s.textDocumentDidSave,
		TextDocumentDidClose:   s.textDocumentDidClose,
		TextDocumentCompletion: s.textDocumentCompletion,
		TextDocumentHover:      s.textDocumentHover,
	}

	s.server = server.NewServer(&s.handler, serverName, false)

	return s
}

// RunStdio starts the server on stdin/stdout.
func (s *Server) RunStdio() error {
	return s.server.RunStdio()
}

func (s *Server) initialize(ctx *glsp.Context, params *protocol.InitializeParams) (any, error) {
	// Capture the notify function for use in async handlers (debounced validation).
	s.notifyMu.Lock()
	s.notify = ctx.Notify
	s.notifyMu.Unlock()

	s.logger.Infof("initialize request received")

	capabilities := s.handler.CreateServerCapabilities()

	sync := protocol.TextDocumentSyncKindFull
	capabilities.TextDocumentSync = &protocol.TextDocumentSyncOptions{
		OpenClose: boolPtr(true),
		Change:    &sync,
		Save: &protocol.SaveOptions{
			IncludeText: boolPtr(false),
		},
	}
	capabilities.CompletionProvider = &protocol.CompletionOptions{}
	capabilities.HoverProvider = true

	return protocol.InitializeResult{
		Capabilities: capabilities,
		ServerInfo: &protocol.InitializeResultServerInfo{
			Name:    serverName,
			Version: &version,
		},
	}, nil
}

func (s *Server) initialized(ctx *glsp.Context, params *protocol.InitializedParams) error {
	return nil
}

func (s *Server) shutdown(ctx *glsp.Context) error {
	protocol.SetTraceValue(protocol.TraceValueOff)
	s.debouncer.Shutdown()
	return nil
}

func (s *Server) setTrace(ctx *glsp.Context, params *protocol.SetTraceParams) error {
	protocol.SetTraceValue(params.Value)
	return nil
}

func (s *Server) textDocumentDidOpen(ctx *glsp.Context, params *protocol.DidOpenTextDocumentParams) error {
	s.documents.Set(params.TextDocument.URI, params.TextDocument.Text)
	s.triggerValidation(ctx, params.TextDocument.URI)
	return nil
}

func (s *Server) textDocumentDidChange(ctx *glsp.Context, params *protocol.DidChangeTextDocumentParams) error {
	s.documents.Update(params.TextDocument.URI, params.ContentChanges)
	return nil
}

func (s *Server) textDocumentDidSave(ctx *glsp.Context, params *protocol.DidSaveTextDocumentParams) error {
	s.triggerValidation(ctx, params.TextDocument.URI)
	return nil
}

func (s *Server) textDocumentDidClose(ctx *glsp.Context, params *protocol.DidCloseTextDocumentParams) error {
	s.documents.Delete(params.TextDocument.URI)
	return nil
}

func (s *Server) triggerValidation(ctx *glsp.Context, uri protocol.DocumentUri) {
	filePath, err := uriToPath(uri)
	if err != nil {
		s.logger.Errorf("failed to parse URI %s: %v", uri, err)
		return
	}

	s.logger.Infof("triggerValidation for %s", filePath)

	packageRoot, err := findPackageRoot(filePath)
	if err != nil {
		s.logger.Infof("file not inside a package: %s", filePath)
		return
	}

	s.logger.Infof("found package root: %s", packageRoot)

	s.debouncer.Trigger(packageRoot, func() {
		s.logger.Infof("debounce fired, validating %s", packageRoot)
		diags := validatePackage(packageRoot)
		s.logger.Infof("validation returned %d file(s) with diagnostics", len(diags))
		s.publishAllDiagnostics(packageRoot, diags)
	})
}

func (s *Server) publishAllDiagnostics(packageRoot string, diagsByFile map[string][]protocol.Diagnostic) {
	s.notifyMu.Lock()
	notify := s.notify
	s.notifyMu.Unlock()

	if notify == nil {
		s.logger.Errorf("cannot publish diagnostics: no notify function (initialize not called?)")
		return
	}

	s.prevDiagFilesMu.Lock()
	defer s.prevDiagFilesMu.Unlock()

	// Clear diagnostics for files that had errors before but don't anymore.
	if prev, ok := s.prevDiagFiles[packageRoot]; ok {
		for filePath := range prev {
			if _, stillHasErrors := diagsByFile[filePath]; !stillHasErrors {
				uri := pathToURI(filePath)
				notify(protocol.ServerTextDocumentPublishDiagnostics, protocol.PublishDiagnosticsParams{
					URI:         uri,
					Diagnostics: []protocol.Diagnostic{},
				})
			}
		}
	}

	// Publish current diagnostics and track the files.
	currentFiles := make(map[string]struct{}, len(diagsByFile))
	for filePath, diags := range diagsByFile {
		uri := pathToURI(filePath)
		s.logger.Infof("publishing %d diagnostic(s) for %s", len(diags), filePath)
		notify(protocol.ServerTextDocumentPublishDiagnostics, protocol.PublishDiagnosticsParams{
			URI:         uri,
			Diagnostics: diags,
		})
		if len(diags) > 0 {
			currentFiles[filePath] = struct{}{}
		}
	}
	s.prevDiagFiles[packageRoot] = currentFiles
}

func boolPtr(b bool) *bool {
	return &b
}
