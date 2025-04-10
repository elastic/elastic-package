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
	"github.com/stretchr/testify/require"
)

func TestLinkUpdateChecksum(t *testing.T) {
	wd, err := os.Getwd()
	assert.NoError(t, err)
	basePath := filepath.Join(wd, filepath.FromSlash("testdata/links"))
	outdatedFile, err := NewLinkedFile(filepath.Join(basePath, "outdated.yml.link"))
	t.Cleanup(func() {
		_ = WriteFile(filepath.Join(outdatedFile.WorkDir, outdatedFile.LinkFilePath), []byte(outdatedFile.IncludedFilePath))
	})
	assert.NoError(t, err)
	assert.False(t, outdatedFile.UpToDate)
	assert.Empty(t, outdatedFile.LinkChecksum)
	updated, err := outdatedFile.UpdateChecksum()
	assert.NoError(t, err)
	assert.True(t, updated)
	assert.Equal(t, "d709feed45b708c9548a18ca48f3ad4f41be8d3f691f83d7417ca902a20e6c1e", outdatedFile.LinkChecksum)
	assert.True(t, outdatedFile.UpToDate)

	uptodateFile, err := NewLinkedFile(filepath.Join(basePath, "uptodate.yml.link"))
	assert.NoError(t, err)
	assert.True(t, uptodateFile.UpToDate)
	updated, err = uptodateFile.UpdateChecksum()
	assert.NoError(t, err)
	assert.False(t, updated)
}

func TestListLinkedFiles(t *testing.T) {
	wd, err := os.Getwd()
	assert.NoError(t, err)
	basePath := filepath.Join(wd, filepath.FromSlash("testdata/links"))
	linkedFiles, err := ListLinkedFiles(basePath)
	require.NoError(t, err)
	require.NotEmpty(t, linkedFiles)
	require.Len(t, linkedFiles, 2)
	assert.Equal(t, "outdated.yml.link", linkedFiles[0].LinkFilePath)
	assert.Empty(t, linkedFiles[0].LinkChecksum)
	assert.Equal(t, "outdated.yml", linkedFiles[0].TargetFilePath(""))
	assert.Equal(t, "./included.yml", linkedFiles[0].IncludedFilePath)
	assert.Equal(t, "d709feed45b708c9548a18ca48f3ad4f41be8d3f691f83d7417ca902a20e6c1e", linkedFiles[0].IncludedFileContentsChecksum)
	assert.False(t, linkedFiles[0].UpToDate)
	assert.Equal(t, "uptodate.yml.link", linkedFiles[1].LinkFilePath)
	assert.Equal(t, "d709feed45b708c9548a18ca48f3ad4f41be8d3f691f83d7417ca902a20e6c1e", linkedFiles[1].LinkChecksum)
	assert.Equal(t, "uptodate.yml", linkedFiles[1].TargetFilePath(""))
	assert.Equal(t, "./included.yml", linkedFiles[1].IncludedFilePath)
	assert.Equal(t, "d709feed45b708c9548a18ca48f3ad4f41be8d3f691f83d7417ca902a20e6c1e", linkedFiles[1].IncludedFileContentsChecksum)
	assert.True(t, linkedFiles[1].UpToDate)
}

func TestCopyFile(t *testing.T) {
	fileA := "fileA.txt"
	fileB := "fileB.txt"
	t.Cleanup(func() { _ = os.Remove(fileA) })
	t.Cleanup(func() { _ = os.Remove(fileB) })

	createDummyFile(t, fileA, "This is the content of the file.")

	assert.NoError(t, CopyFile(fileA, fileB))

	equal, err := filesEqual(fileA, fileB)
	assert.NoError(t, err)
	assert.True(t, equal, "files should be equal after copying")
}

func TestAreLinkedFilesUpToDate(t *testing.T) {
	wd, err := os.Getwd()
	assert.NoError(t, err)
	basePath := filepath.Join(wd, filepath.FromSlash("testdata/links"))
	linkedFiles, err := AreLinkedFilesUpToDate(basePath)
	assert.NoError(t, err)
	assert.NotEmpty(t, linkedFiles)
	assert.Len(t, linkedFiles, 1)
	assert.Equal(t, "outdated.yml.link", linkedFiles[0].LinkFilePath)
	assert.Empty(t, linkedFiles[0].LinkChecksum)
	assert.Equal(t, "outdated.yml", linkedFiles[0].TargetFilePath(""))
	assert.Equal(t, "./included.yml", linkedFiles[0].IncludedFilePath)
	assert.Equal(t, "d709feed45b708c9548a18ca48f3ad4f41be8d3f691f83d7417ca902a20e6c1e", linkedFiles[0].IncludedFileContentsChecksum)
	assert.False(t, linkedFiles[0].UpToDate)
}

func TestUpdateLinkedFilesChecksums(t *testing.T) {
	wd, err := os.Getwd()
	assert.NoError(t, err)
	basePath := filepath.Join(wd, filepath.FromSlash("testdata/links"))
	updated, err := UpdateLinkedFilesChecksums(basePath)
	t.Cleanup(func() {
		_ = WriteFile(filepath.Join(updated[0].WorkDir, updated[0].LinkFilePath), []byte(updated[0].IncludedFilePath))
	})
	assert.NoError(t, err)
	assert.NotEmpty(t, updated)
	assert.Len(t, updated, 1)
	assert.True(t, updated[0].UpToDate)
	assert.Equal(t, "d709feed45b708c9548a18ca48f3ad4f41be8d3f691f83d7417ca902a20e6c1e", updated[0].LinkChecksum)

}

func TestLinkedFilesByPackageFrom(t *testing.T) {
	wd, err := os.Getwd()
	assert.NoError(t, err)
	basePath := filepath.Join(wd, filepath.FromSlash("testdata/links"))
	m, err := LinkedFilesByPackageFrom(basePath)
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

func TestIncludeLinkedFiles(t *testing.T) {
	wd, err := os.Getwd()
	assert.NoError(t, err)
	fromDir := filepath.Join(wd, filepath.FromSlash("testdata/testpackage"))
	toDir := t.TempDir()
	linkedFiles, err := IncludeLinkedFiles(fromDir, toDir)
	assert.NoError(t, err)
	require.Equal(t, 1, len(linkedFiles))
	assert.FileExists(t, linkedFiles[0].TargetFilePath(toDir))
	equal, err := filesEqual(
		filepath.Join(linkedFiles[0].WorkDir, filepath.FromSlash(linkedFiles[0].IncludedFilePath)),
		linkedFiles[0].TargetFilePath(toDir),
	)
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
