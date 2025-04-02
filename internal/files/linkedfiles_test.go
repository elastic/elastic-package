// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package files

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/elastic/package-spec/v3/code/go/pkg/linkedfiles"
	"github.com/stretchr/testify/assert"
)

func TestAreLinkedFilesUpToDate(t *testing.T) {
	linkedFiles, err := AreLinkedFilesUpToDate("internal/files/testdata/links")
	assert.NoError(t, err)
	assert.NotEmpty(t, linkedFiles)
	assert.Len(t, linkedFiles, 1)
	assert.Equal(t, "internal/files/testdata/links/outdated.yml.link", linkedFiles[0].LinkFilePath)
	assert.Empty(t, linkedFiles[0].LinkChecksum)
	assert.Equal(t, "internal/files/testdata/links/outdated.yml", linkedFiles[0].TargetFilePath)
	assert.Equal(t, "internal/files/testdata/links/included.yml", linkedFiles[0].IncludedFilePath)
	assert.Equal(t, "d709feed45b708c9548a18ca48f3ad4f41be8d3f691f83d7417ca902a20e6c1e", linkedFiles[0].IncludedFileContentsChecksum)
	assert.False(t, linkedFiles[0].UpToDate)
}

func TestUpdateLinkedFilesChecksums(t *testing.T) {
	root, err := linkedfiles.FindRepositoryRoot()
	assert.NoError(t, err)
	updated, err := UpdateLinkedFilesChecksums("internal/files/testdata/links")
	t.Cleanup(func() {
		_ = linkedfiles.WriteFileToRoot(root, updated[0].LinkFilePath, []byte(updated[0].IncludedFilePath))
	})
	assert.NoError(t, err)
	assert.NotEmpty(t, updated)
	assert.Len(t, updated, 1)
	assert.True(t, updated[0].UpToDate)
	assert.Equal(t, "d709feed45b708c9548a18ca48f3ad4f41be8d3f691f83d7417ca902a20e6c1e", updated[0].LinkChecksum)

}

func TestLinkedFilesByPackageFrom(t *testing.T) {
	m, err := LinkedFilesByPackageFrom("internal/files/testdata/links")
	assert.NoError(t, err)
	assert.NotEmpty(t, m)
	assert.Len(t, m, 1)
	assert.NotEmpty(t, m[0])
	assert.Len(t, m[0], 1)
	assert.NotEmpty(t, m[0]["testpackage"])
	assert.Len(t, m[0]["testpackage"], 1)
	match := strings.HasSuffix(
		filepath.ToSlash(m[0]["testpackage"][0]),
		"/testdata/testpackage/included.yml.link",
	)
	assert.True(t, match)
}
