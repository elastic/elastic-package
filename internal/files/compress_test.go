// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package files

import (
	"archive/zip"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestZip(t *testing.T) {
	sourcePath := "testdata/testzip"
	destinationName := "packagename-1.0.0"
	destinationFile := filepath.Join(t.TempDir(), destinationName+".zip")

	err := Zip(sourcePath, destinationFile)
	require.NoError(t, err)

	reader, err := zip.OpenReader(destinationFile)
	require.NoError(t, err)
	defer reader.Close()

	// Check that all files are the same in the zip file as in the source directory.
	err = filepath.WalkDir(sourcePath, func(filePath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(sourcePath, filePath)
		require.NoError(t, err)

		t.Run(filePath, func(t *testing.T) {
			destinationPath := path.Join(destinationName, filepath.ToSlash(relPath))
			if d.IsDir() {
				stat, err := fs.Stat(reader, destinationPath)
				require.NoError(t, err)
				assert.True(t, stat.IsDir())
				return
			}

			expectedContent, err := os.ReadFile(filePath)
			require.NoError(t, err)

			foundContent, err := fs.ReadFile(reader, destinationPath)
			require.NoError(t, err)

			assert.Equal(t, string(expectedContent), string(foundContent))
		})
		return nil
	})
	require.NoError(t, err)
}
