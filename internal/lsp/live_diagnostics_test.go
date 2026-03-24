// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package lsp

import (
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	protocol "github.com/tliron/glsp/protocol_3_16"
)

func TestValidatePackageFSUsesUnsavedBufferText(t *testing.T) {
	packageRoot := copyFixturePackage(t, fixturePackagePath(t, "test", "packages", "parallel", "apache"))
	manifestPath := filepath.Join(packageRoot, "manifest.yml")

	manifestText := readTestFile(t, manifestPath)
	updatedText := strings.Replace(manifestText, "categories:\n  - web", "# unsaved change\ncategories:\n  - madeup", 1)

	diagsByFile := validatePackageFS(packageRoot, newOverlayFS(packageRoot, map[string]string{
		"manifest.yml": updatedText,
	}))

	categoryDiag := findDiagnostic(diagsByFile[manifestPath], "field categories.0:")
	require.NotNil(t, categoryDiag)
	assert.Equal(t, uint32(11), categoryDiag.Range.Start.Line)
}

func TestTextDocumentDidChangePublishesAndDidCloseClearsUnsavedDiagnostics(t *testing.T) {
	packageRoot := copyFixturePackage(t, fixturePackagePath(t, "test", "packages", "parallel", "apache"))
	manifestPath := filepath.Join(packageRoot, "manifest.yml")
	manifestURI := protocol.DocumentUri(pathToURI(manifestPath))

	manifestText := readTestFile(t, manifestPath)
	updatedText := strings.Replace(manifestText, "categories:\n  - web", "# unsaved change\ncategories:\n  - madeup", 1)

	server := NewServer()
	t.Cleanup(server.debouncer.Shutdown)

	var mu sync.Mutex
	var published []protocol.PublishDiagnosticsParams

	server.notifyMu.Lock()
	server.notify = func(method string, params any) {
		if method != protocol.ServerTextDocumentPublishDiagnostics {
			return
		}

		publish, ok := params.(protocol.PublishDiagnosticsParams)
		if !ok {
			return
		}

		mu.Lock()
		defer mu.Unlock()
		published = append(published, publish)
	}
	server.notifyMu.Unlock()

	err := server.textDocumentDidChange(nil, &protocol.DidChangeTextDocumentParams{
		TextDocument: protocol.VersionedTextDocumentIdentifier{
			TextDocumentIdentifier: protocol.TextDocumentIdentifier{URI: manifestURI},
			Version:                1,
		},
		ContentChanges: []any{
			protocol.TextDocumentContentChangeEventWhole{Text: updatedText},
		},
	})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		publish, ok := latestDiagnosticsForURI(&mu, &published, manifestURI)
		if !ok {
			return false
		}

		categoryDiag := findDiagnostic(publish.Diagnostics, "field categories.0:")
		return categoryDiag != nil && categoryDiag.Range.Start.Line == 11
	}, 3*time.Second, 50*time.Millisecond)

	err = server.textDocumentDidClose(nil, &protocol.DidCloseTextDocumentParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: manifestURI},
	})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		publish, ok := latestDiagnosticsForURI(&mu, &published, manifestURI)
		return ok && findDiagnostic(publish.Diagnostics, "field categories.0:") == nil
	}, 3*time.Second, 50*time.Millisecond)
}

func latestDiagnosticsForURI(mu *sync.Mutex, published *[]protocol.PublishDiagnosticsParams, uri protocol.DocumentUri) (protocol.PublishDiagnosticsParams, bool) {
	mu.Lock()
	defer mu.Unlock()

	for i := len(*published) - 1; i >= 0; i-- {
		if (*published)[i].URI == uri {
			return (*published)[i], true
		}
	}

	return protocol.PublishDiagnosticsParams{}, false
}

func findDiagnostic(diags []protocol.Diagnostic, substring string) *protocol.Diagnostic {
	for i := range diags {
		if strings.Contains(diags[i].Message, substring) {
			return &diags[i]
		}
	}
	return nil
}

func fixturePackagePath(t *testing.T, elems ...string) string {
	t.Helper()

	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)

	repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..")
	parts := append([]string{repoRoot}, elems...)
	return filepath.Join(parts...)
}

func copyFixturePackage(t *testing.T, src string) string {
	t.Helper()

	dst := filepath.Join(t.TempDir(), filepath.Base(src))
	require.NoError(t, copyDir(src, dst))
	return dst
}

func copyDir(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dst, info.Mode()); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
			continue
		}

		if err := copyFile(srcPath, dstPath); err != nil {
			return err
		}
	}

	return nil
}

func copyFile(src, dst string) error {
	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()

	info, err := source.Stat()
	if err != nil {
		return err
	}

	target, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode())
	if err != nil {
		return err
	}
	defer target.Close()

	_, err = io.Copy(target, source)
	return err
}

func readTestFile(t *testing.T, path string) string {
	t.Helper()

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(data)
}
