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
)

func TestFindPosition_FieldPath(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "default.yml")
	require.NoError(t, os.WriteFile(f, []byte(`---
description: "Pipeline for parsing logs"
processors:
  - set:
      field: event.kind
      value: event
  - grok:
      field: message
      patterns:
        - "%{COMBINEDAPACHELOG}"
  - remove:
      field: message
  - grokk:
      field: bad
`), 0644))

	// "field processors.3:" should land on the 4th element (0-indexed: 3)
	// which is the "grokk" mapping node
	r := findPosition(f, "field processors.3: Additional property grokk is not allowed")
	// grokk is a key inside the mapping at index 3 of the sequence
	assert.Equal(t, uint32(12), r.Start.Line, "expected line 12 (0-based) for grokk key")
}

func TestFindPosition_TopLevelField(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "manifest.yml")
	require.NoError(t, os.WriteFile(f, []byte(`format_version: 3.3.0
name: my_package
title: ""
version: 0.0.1
`), 0644))

	r := findPosition(f, "field title: String length must be greater than or equal to 1")
	assert.Equal(t, uint32(2), r.Start.Line, "expected line 2 (0-based) for title")
}

func TestFindPosition_NoMatch(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "manifest.yml")
	require.NoError(t, os.WriteFile(f, []byte(`name: test
`), 0644))

	r := findPosition(f, "some error without field path")
	assert.Equal(t, uint32(0), r.Start.Line)
}
