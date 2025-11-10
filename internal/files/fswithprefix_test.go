// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package files

import (
	"embed"
	"io/fs"
	"path"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//go:embed testdata/testfswithprefix
var testWithPrefixFS embed.FS

func TestFSWithPrefix(t *testing.T) {
	subFS, err := fs.Sub(testWithPrefixFS, "testdata/testfswithprefix")
	require.NoError(t, err)

	// It should work with any string that is a valid path.
	prefix := uuid.New().String()
	testFS := newFSWithPrefix(subFS, prefix)
	expectedFiles := []string{
		"onefile.txt",
		"otherfile.txt",
		"onedir/fileindir.txt",
	}
	wrongPath := path.Join(prefix, "./this/path/does/not/exist.txt")
	t.Run("open", func(t *testing.T) {
		for _, file := range expectedFiles {
			t.Run(file, func(t *testing.T) {
				f, err := testFS.Open(path.Join(prefix, file))
				require.NoError(t, err)

				fi, err := f.Stat()
				require.NoError(t, err)

				assert.Equal(t, path.Base(file), fi.Name())

			})
		}

		_, err = testFS.Open(wrongPath)
		var pathErr *fs.PathError
		if assert.ErrorAs(t, err, &pathErr) {
			assert.Equal(t, wrongPath, pathErr.Path)
		}
	})
	t.Run("stat", func(t *testing.T) {
		statFS := testFS.(fs.StatFS)
		for _, file := range expectedFiles {
			t.Run(file, func(t *testing.T) {
				fi, err := statFS.Stat(path.Join(prefix, file))
				require.NoError(t, err)

				assert.Equal(t, path.Base(file), fi.Name())
			})
		}

		_, err = statFS.Stat(wrongPath)
		var pathErr *fs.PathError
		if assert.ErrorAs(t, err, &pathErr) {
			assert.Equal(t, wrongPath, pathErr.Path)
		}
	})
	t.Run("readfile", func(t *testing.T) {
		readFileFS := testFS.(fs.ReadFileFS)

		for _, file := range expectedFiles {
			t.Run(file, func(t *testing.T) {
				expectedContent, err := fs.ReadFile(subFS, file)
				require.NoError(t, err)

				foundContent, err := readFileFS.ReadFile(path.Join(prefix, file))
				require.NoError(t, err)

				assert.Equal(t, string(expectedContent), string(foundContent))
			})
		}

		_, err = readFileFS.ReadFile(wrongPath)
		var pathErr *fs.PathError
		if assert.ErrorAs(t, err, &pathErr) {
			assert.Equal(t, wrongPath, pathErr.Path)
		}
	})
	t.Run("readdir", func(t *testing.T) {
		readDirFS := testFS.(fs.ReadDirFS)

		entries, err := readDirFS.ReadDir(".")
		require.NoError(t, err)
		assert.Len(t, entries, 1)
		assert.Equal(t, prefix, entries[0].Name())

		entries, err = readDirFS.ReadDir(prefix)
		require.NoError(t, err)
		assert.Len(t, entries, 3)

		entries, err = readDirFS.ReadDir(path.Join(prefix, "onedir"))
		require.NoError(t, err)
		assert.Len(t, entries, 1)
		assert.Equal(t, "fileindir.txt", entries[0].Name())

		_, err = readDirFS.ReadDir(wrongPath)
		var pathErr *fs.PathError
		if assert.ErrorAs(t, err, &pathErr) {
			assert.Equal(t, wrongPath, pathErr.Path)
		}
	})
}
