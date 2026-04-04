// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package lsp

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildFieldIndexIncludesNestedFieldsAcrossDataStreams(t *testing.T) {
	packageRoot := copyFixturePackage(t, fixturePackagePath(t, "test", "packages", "parallel", "apache"))

	idx := BuildFieldIndex(packageRoot)

	assert.Contains(t, idx, "apache.access.ssl.protocol")
	assert.Contains(t, idx, "apache.error.module")
	assert.Contains(t, idx, "apache.status.total_bytes")
	assert.Equal(t, "keyword", idx["apache.access.ssl.protocol"].Type)
	assert.Equal(t, "byte", idx["apache.status.total_bytes"].Unit)
}

func TestBuildFieldIndexForDataStreamScopesResults(t *testing.T) {
	packageRoot := copyFixturePackage(t, fixturePackagePath(t, "test", "packages", "parallel", "apache"))

	idx := BuildFieldIndexForDataStream(packageRoot, "status")

	assert.Contains(t, idx, "apache.status.total_bytes")
	assert.NotContains(t, idx, "apache.access.ssl.protocol")
}
