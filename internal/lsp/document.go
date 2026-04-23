// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package lsp

import (
	"os"
	"strings"
	"sync"
	"unicode/utf16"

	protocol "github.com/tliron/glsp/protocol_3_16"
)

// documentStore keeps the latest text for open documents so editor features can
// use unsaved content instead of falling back to the on-disk file.
type documentStore struct {
	mu   sync.RWMutex
	text map[string]string
}

func newDocumentStore() *documentStore {
	return &documentStore{
		text: make(map[string]string),
	}
}

func (d *documentStore) Set(uri protocol.DocumentUri, text string) {
	filePath, err := uriToPath(uri)
	if err != nil {
		return
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	d.text[filePath] = text
}

func (d *documentStore) Update(uri protocol.DocumentUri, changes []any) {
	if len(changes) == 0 {
		return
	}

	filePath, err := uriToPath(uri)
	if err != nil {
		return
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	text := d.text[filePath]
	for _, change := range changes {
		switch value := change.(type) {
		case protocol.TextDocumentContentChangeEventWhole:
			text = value.Text
		case protocol.TextDocumentContentChangeEvent:
			if value.Range == nil {
				text = value.Text
				continue
			}
			text = applyTextChange(text, *value.Range, value.Text)
		}
	}
	d.text[filePath] = text
}

func (d *documentStore) Delete(uri protocol.DocumentUri) {
	filePath, err := uriToPath(uri)
	if err != nil {
		return
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.text, filePath)
}

func (d *documentStore) Text(filePath string) (string, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	text, ok := d.text[filePath]
	return text, ok
}

func (d *documentStore) Snapshot(packageRoot string) map[string]string {
	d.mu.RLock()
	defer d.mu.RUnlock()

	snapshot := make(map[string]string)
	for filePath, text := range d.text {
		relPath, ok := relativeFSPath(packageRoot, filePath)
		if !ok {
			continue
		}
		snapshot[relPath] = text
	}
	return snapshot
}

func (s *Server) documentText(filePath string) string {
	if text, ok := s.documents.Text(filePath); ok {
		return text
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return ""
	}
	return string(data)
}

func getLineAtText(text string, lineNum int) string {
	lines := splitLines(text)
	if lineNum < 0 || lineNum >= len(lines) {
		return ""
	}
	return lines[lineNum]
}

func splitLines(text string) []string {
	return strings.Split(text, "\n")
}

func applyTextChange(text string, rng protocol.Range, replacement string) string {
	runes := []rune(text)
	start := positionOffset(text, rng.Start)
	end := positionOffset(text, rng.End)
	if start < 0 || end < start || start > len(runes) || end > len(runes) {
		return text
	}

	return string(runes[:start]) + replacement + string(runes[end:])
}

func positionOffset(text string, pos protocol.Position) int {
	lines := splitLines(text)
	targetLine := int(pos.Line)
	if targetLine < 0 {
		return 0
	}
	if targetLine >= len(lines) {
		return len([]rune(text))
	}

	offset := 0
	for i := 0; i < targetLine; i++ {
		offset += len([]rune(lines[i])) + 1
	}

	return offset + utf16ColumnToRuneOffset(lines[targetLine], int(pos.Character))
}

func utf16ColumnToRuneOffset(line string, target int) int {
	if target <= 0 {
		return 0
	}

	offset := 0
	column := 0
	for _, r := range line {
		if column >= target {
			return offset
		}
		column += utf16Width(r)
		offset++
	}
	return offset
}

func utf16Width(r rune) int {
	width := utf16.RuneLen(r)
	if width < 1 {
		return 1
	}
	return width
}
