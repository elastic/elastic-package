// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package lsp

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseError_FilePattern(t *testing.T) {
	packageRoot := "/home/user/packages/apache"

	filePath, message, code := parseError(
		`file "/home/user/packages/apache/data_stream/access/fields/fields.yml" is invalid: field vars.1: Must not be present`,
		packageRoot,
	)

	assert.Equal(t, "/home/user/packages/apache/data_stream/access/fields/fields.yml", filePath)
	assert.Equal(t, "field vars.1: Must not be present", message)
	assert.Equal(t, "", code)
}

func TestParseError_RelativePath(t *testing.T) {
	packageRoot := "/home/user/packages/apache"

	filePath, message, code := parseError(
		`file "data_stream/access/manifest.yml" is invalid: field title: String length must be greater than or equal to 1`,
		packageRoot,
	)

	assert.Equal(t, "/home/user/packages/apache/data_stream/access/manifest.yml", filePath)
	assert.Equal(t, "field title: String length must be greater than or equal to 1", message)
	assert.Equal(t, "", code)
}

func TestParseError_WithCode(t *testing.T) {
	packageRoot := "/home/user/packages/apache"

	filePath, message, code := parseError(
		`changelog entry found for version 1.0.0 but package version is 2.0.0 (SVR00003)`,
		packageRoot,
	)

	// No file pattern match → falls back to manifest.yml
	assert.Equal(t, "/home/user/packages/apache/manifest.yml", filePath)
	assert.Equal(t, "changelog entry found for version 1.0.0 but package version is 2.0.0 (SVR00003)", message)
	assert.Equal(t, "SVR00003", code)
}

func TestParseError_NoFilePattern(t *testing.T) {
	packageRoot := "/home/user/packages/apache"

	filePath, message, code := parseError(
		`some generic validation error`,
		packageRoot,
	)

	assert.Equal(t, "/home/user/packages/apache/manifest.yml", filePath)
	assert.Equal(t, "some generic validation error", message)
	assert.Equal(t, "", code)
}

func TestStripNumbering(t *testing.T) {
	assert.Equal(t, "hello world", stripNumbering("   1. hello world"))
	assert.Equal(t, "error msg", stripNumbering("  12. error msg"))
	assert.Equal(t, "no number", stripNumbering("no number"))
}

func TestURIConversion(t *testing.T) {
	path := "/home/user/packages/apache/manifest.yml"
	uri := pathToURI(path)
	assert.Equal(t, "file:///home/user/packages/apache/manifest.yml", uri)

	roundTripped, err := uriToPath(uri)
	assert.NoError(t, err)
	assert.Equal(t, path, roundTripped)
}
