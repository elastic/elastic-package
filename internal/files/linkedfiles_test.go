// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package files

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLinkUpdateChecksum(t *testing.T) {
	root, err := FindRepositoryRoot()
	assert.NoError(t, err)

	outdatedFile, err := newLinkedFile(root, "testdata/links/outdated.yml.link")
	t.Cleanup(func() {
		_ = writeFile(outdatedFile.LinkFilePath, []byte(outdatedFile.IncludedFilePath))
	})
	assert.NoError(t, err)
	assert.False(t, outdatedFile.UpToDate)
	assert.Empty(t, outdatedFile.LinkChecksum)
	updated, err := outdatedFile.UpdateChecksum()
	assert.NoError(t, err)
	assert.True(t, updated)
	assert.Equal(t, "d709feed45b708c9548a18ca48f3ad4f41be8d3f691f83d7417ca902a20e6c1e", outdatedFile.LinkChecksum)
	assert.True(t, outdatedFile.UpToDate)

	uptodateFile, err := newLinkedFile(root, "testdata/links/uptodate.yml.link")
	assert.NoError(t, err)
	assert.True(t, uptodateFile.UpToDate)
	updated, err = uptodateFile.UpdateChecksum()
	assert.NoError(t, err)
	assert.False(t, updated)
}

func TestLinkReplaceTargetFilePathDirectory(t *testing.T) {
	root, err := FindRepositoryRoot()
	assert.NoError(t, err)

	linkedFile, err := newLinkedFile(root, "testdata/links/uptodate.yml.link")
	assert.NoError(t, err)
	assert.Equal(t, "testdata/links/uptodate.yml", linkedFile.TargetFilePath)

	linkedFile.ReplaceTargetFilePathDirectory("testdata/links", "build/testdata/links")
	assert.Equal(t, "build/testdata/links/uptodate.yml", linkedFile.TargetFilePath)
}

func TestAreLinkedFilesUpToDate(t *testing.T) {
	linkedFiles, err := AreLinkedFilesUpToDate("testdata/links")
	assert.NoError(t, err)
	assert.NotEmpty(t, linkedFiles)
	assert.Len(t, linkedFiles, 1)
	assert.Equal(t, "testdata/links/outdated.yml.link", linkedFiles[0].LinkFilePath)
	assert.Empty(t, linkedFiles[0].LinkChecksum)
	assert.Equal(t, "testdata/links/outdated.yml", linkedFiles[0].TargetFilePath)
	assert.Equal(t, "internal/files/testdata/links/included.yml", linkedFiles[0].IncludedFilePath)
	assert.Equal(t, "d709feed45b708c9548a18ca48f3ad4f41be8d3f691f83d7417ca902a20e6c1e", linkedFiles[0].IncludedFileContentsChecksum)
	assert.False(t, linkedFiles[0].UpToDate)
}

func TestUpdateLinkedFilesChecksums(t *testing.T) {
	updated, err := UpdateLinkedFilesChecksums("testdata/links")
	t.Cleanup(func() {
		_ = writeFile(updated[0].LinkFilePath, []byte(updated[0].IncludedFilePath))
	})
	assert.NoError(t, err)
	assert.NotEmpty(t, updated)
	assert.Len(t, updated, 1)
	assert.True(t, updated[0].UpToDate)
	assert.Equal(t, "d709feed45b708c9548a18ca48f3ad4f41be8d3f691f83d7417ca902a20e6c1e", updated[0].LinkChecksum)

}

func TestLinkedFilesByPackageFrom(t *testing.T) {
	m, err := LinkedFilesByPackageFrom("testdata/links")
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

func TestListLinkedFiles(t *testing.T) {
	linkedFiles, err := ListLinkedFiles("testdata/links")
	assert.NoError(t, err)
	assert.NotEmpty(t, linkedFiles)
	assert.Len(t, linkedFiles, 2)
	assert.Equal(t, "testdata/links/outdated.yml.link", linkedFiles[0].LinkFilePath)
	assert.Empty(t, linkedFiles[0].LinkChecksum)
	assert.Equal(t, "testdata/links/outdated.yml", linkedFiles[0].TargetFilePath)
	assert.Equal(t, "internal/files/testdata/links/included.yml", linkedFiles[0].IncludedFilePath)
	assert.Equal(t, "d709feed45b708c9548a18ca48f3ad4f41be8d3f691f83d7417ca902a20e6c1e", linkedFiles[0].IncludedFileContentsChecksum)
	assert.False(t, linkedFiles[0].UpToDate)
	assert.Equal(t, "testdata/links/uptodate.yml.link", linkedFiles[1].LinkFilePath)
	assert.Equal(t, "d709feed45b708c9548a18ca48f3ad4f41be8d3f691f83d7417ca902a20e6c1e", linkedFiles[1].LinkChecksum)
	assert.Equal(t, "testdata/links/uptodate.yml", linkedFiles[1].TargetFilePath)
	assert.Equal(t, "internal/files/testdata/links/included.yml", linkedFiles[1].IncludedFilePath)
	assert.Equal(t, "d709feed45b708c9548a18ca48f3ad4f41be8d3f691f83d7417ca902a20e6c1e", linkedFiles[1].IncludedFileContentsChecksum)
	assert.True(t, linkedFiles[1].UpToDate)
}

func TestCopyFile(t *testing.T) {
	fileA := filepath.Join(t.TempDir(), "fileA.txt")
	fileB := filepath.Join(t.TempDir(), "fileB.txt")
	t.Cleanup(func() { _ = os.Remove(fileA) })
	t.Cleanup(func() { _ = os.Remove(fileB) })

	createDummyFile(t, fileA, "This is the content of the file.")

	assert.NoError(t, CopyFile(fileA, fileB))

	equal, err := filesEqual(fileA, fileB)
	assert.NoError(t, err)
	assert.True(t, equal, "files should be equal after copying")
}

func createDummyFile(t *testing.T, filename, content string) {
	file, err := os.Create(filename)
	assert.NoError(t, err)
	defer file.Close()
	_, err = file.WriteString(content)
	assert.NoError(t, err)
}

func filesEqual(file1, file2 string) (bool, error) {
	f1, err := os.ReadFile(file1)
	if err != nil {
		return false, err
	}

	f2, err := os.ReadFile(file2)
	if err != nil {
		return false, err
	}

	return bytes.Equal(f1, f2), nil
}
