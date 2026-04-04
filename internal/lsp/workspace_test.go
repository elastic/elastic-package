// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package lsp

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestURIPathRoundTripEncodesSpaces(t *testing.T) {
	path := "/tmp/elastic package/manifest.yml"
	uri := pathToURI(path)

	assert.Equal(t, "file:///tmp/elastic%20package/manifest.yml", uri)

	roundTripped, err := uriToPath(uri)
	require.NoError(t, err)
	assert.Equal(t, path, roundTripped)
}

func TestURIToPathRejectsNonFileScheme(t *testing.T) {
	_, err := uriToPath("https://example.com/manifest.yml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported URI scheme")
}
