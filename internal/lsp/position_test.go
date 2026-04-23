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

func TestFindPosition_RootAdditionalProperty(t *testing.T) {
	f := writePositionTestFile(t, `format_version: 3.3.0
license: basic
`)

	r := findPosition(f, "field (root): Additional property license is not allowed")
	assert.Equal(t, uint32(1), r.Start.Line, "expected line 1 (0-based) for license")
}

func TestFindPosition_DottedAdditionalProperty(t *testing.T) {
	f := writePositionTestFile(t, `conditions:
  kibana.version: "^8.0.0"
  elastic.subscription: basic
`)

	r := findPosition(f, "field conditions: Additional property kibana.version is not allowed")
	assert.Equal(t, uint32(1), r.Start.Line, "expected line 1 (0-based) for kibana.version")

	r = findPosition(f, "field conditions: Additional property elastic.subscription is not allowed")
	assert.Equal(t, uint32(2), r.Start.Line, "expected line 2 (0-based) for elastic.subscription")
}

func TestFindPosition_HyphenatedSegment(t *testing.T) {
	f := writePositionTestFile(t, `services:
  docker-custom-agent:
    image: foo
`)

	r := findPosition(f, "field services.docker-custom-agent: Must not be present")
	assert.Equal(t, uint32(1), r.Start.Line, "expected line 1 (0-based) for docker-custom-agent")
}

func TestFindPosition_LineNumberMessage(t *testing.T) {
	f := writePositionTestFile(t, `processors:
  - set:
      field: key1
      value: value1
`)

	r := findPosition(f, `set processor at line 2 missing required tag (SVR00006)`)
	assert.Equal(t, uint32(1), r.Start.Line, "expected line 1 (0-based) for line-based diagnostic")
}

func TestFindPosition_JSONDanglingReference(t *testing.T) {
	f := writeJSONPositionTestFile(t, `{
  "references": [
    {
      "id": "missing-ref",
      "type": "search"
    }
  ]
}
`)

	r := findPosition(f, `dangling reference found: missing-ref (search)`)
	assert.Equal(t, uint32(3), r.Start.Line, "expected line 3 (0-based) for reference id")
}

func TestFindPosition_JSONDashboardReference(t *testing.T) {
	f := writeJSONPositionTestFile(t, `{
  "references": [
    {
      "id": "by-reference-vis",
      "type": "visualization"
    }
  ]
}
`)

	r := findPosition(f, `reference found in dashboard: by-reference-vis (visualization)`)
	assert.Equal(t, uint32(3), r.Start.Line, "expected line 3 (0-based) for dashboard reference id")
}

func TestFindPosition_JSONLegacyVisualization(t *testing.T) {
	f := writeJSONPositionTestFile(t, `{
  "attributes": {
    "panelsJSON": [
      {
        "title": "TSVB time series",
        "type": "visualization"
      }
    ],
    "title": "Dashboard with mixed by-value visualizations"
  }
}
`)

	r := findPosition(f, `"Dashboard with mixed by-value visualizations" contains legacy visualization: "TSVB time series" (timeseries, TSVB)`)
	assert.Equal(t, uint32(4), r.Start.Line, "expected line 4 (0-based) for visualization title")
}

func TestFindPosition_NoMatch(t *testing.T) {
	f := writePositionTestFile(t, `name: test
`)

	r := findPosition(f, "some error without field path")
	assert.Equal(t, uint32(0), r.Start.Line)
}

func writePositionTestFile(t *testing.T, contents string) string {
	t.Helper()

	dir := t.TempDir()
	f := filepath.Join(dir, "manifest.yml")
	require.NoError(t, os.WriteFile(f, []byte(contents), 0644))
	return f
}

func writeJSONPositionTestFile(t *testing.T, contents string) string {
	t.Helper()

	dir := t.TempDir()
	f := filepath.Join(dir, "asset.json")
	require.NoError(t, os.WriteFile(f, []byte(contents), 0644))
	return f
}
