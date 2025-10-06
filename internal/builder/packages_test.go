// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package builder

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFindRepositoryLicense(t *testing.T) {

	t.Run("FileExists", func(t *testing.T) {
		root, err := os.OpenRoot(t.TempDir())
		require.NoError(t, err)
		defer root.Close()

		// Create a LICENSE.txt file in the temp directory
		expectedPath := filepath.Join(root.Name(), "LICENSE.txt")
		err = os.WriteFile(expectedPath, []byte("license content"), 0644)
		require.NoError(t, err)

		path, err := findRepositoryLicensePath(root, "LICENSE.txt")
		require.NoError(t, err)
		assert.Equal(t, expectedPath, path)
	})

	t.Run("FileDoesNotExist", func(t *testing.T) {
		root, err := os.OpenRoot(t.TempDir())
		require.NoError(t, err)
		defer root.Close()

		path, err := findRepositoryLicensePath(root, "NON_EXISTENT_LICENSE.txt")
		require.Error(t, err)
		assert.Empty(t, path)
	})

}

func TestCopyLicenseTextFile_UsesExistingLicenseFile(t *testing.T) {

	t.Run("ExistingFile", func(t *testing.T) {
		dir := t.TempDir()
		licensePath := filepath.Join(dir, "LICENSE.txt")
		err := os.WriteFile(licensePath, []byte("existing license"), 0644)
		require.NoError(t, err)

		repoRoot, err := os.OpenRoot(t.TempDir())
		require.NoError(t, err)
		defer repoRoot.Close()

		// Should not attempt to copy, just return nil
		err = copyLicenseTextFile(repoRoot, licensePath)
		assert.NoError(t, err)

		// License file should remain unchanged
		content, err := os.ReadFile(licensePath)
		require.NoError(t, err)
		assert.Equal(t, "existing license", string(content))

	})

	t.Run("ExistingDirectory", func(t *testing.T) {
		dir := t.TempDir()

		repoRoot, err := os.OpenRoot(t.TempDir())
		require.NoError(t, err)
		defer repoRoot.Close()

		err = copyLicenseTextFile(repoRoot, dir)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "is a directory")
	})

	t.Run("StatError", func(t *testing.T) {
		// Using a path that is likely invalid to trigger a stat error
		invalidPath := string([]byte{0})

		repoRoot, err := os.OpenRoot(t.TempDir())
		require.NoError(t, err)
		defer repoRoot.Close()

		err = copyLicenseTextFile(repoRoot, invalidPath)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "can't check license path")
	})

	t.Run("RepoLicenseDefaultFileName", func(t *testing.T) {
		repoRoot, err := os.OpenRoot(t.TempDir())
		require.NoError(t, err)
		defer repoRoot.Close()

		// original license file path
		originalFile := filepath.Join(repoRoot.Name(), licenseTextFileName)
		err = os.WriteFile(originalFile, []byte("repo license"), 0644)
		require.NoError(t, err)

		expectedPath := filepath.Join(repoRoot.Name(), "REPO_LICENSE.txt")

		err = copyLicenseTextFile(repoRoot, expectedPath)
		assert.NoError(t, err)

		content, err := os.ReadFile(expectedPath)
		require.NoError(t, err)
		assert.Equal(t, "repo license", string(content))
	})

	t.Run("RepoLicenseCustomFileName", func(t *testing.T) {
		repoRoot, err := os.OpenRoot(t.TempDir())
		require.NoError(t, err)
		defer repoRoot.Close()

		// original license file path
		originalFile := filepath.Join(repoRoot.Name(), "CUSTOM_LICENSE.txt")
		err = os.WriteFile(originalFile, []byte("repo license"), 0644)
		require.NoError(t, err)

		t.Setenv(repositoryLicenseEnv, "CUSTOM_LICENSE.txt")

		expectedPath := filepath.Join(repoRoot.Name(), "REPO_LICENSE.txt")

		err = copyLicenseTextFile(repoRoot, expectedPath)
		assert.NoError(t, err)

		content, err := os.ReadFile(expectedPath)
		require.NoError(t, err)
		assert.Equal(t, "repo license", string(content))
	})

	t.Run("RepoLicenseFileDoesNotExist", func(t *testing.T) {
		repoRoot, err := os.OpenRoot(t.TempDir())
		require.NoError(t, err)
		defer repoRoot.Close()

		expectedPath := filepath.Join(repoRoot.Name(), "REPO_LICENSE.txt")

		err = copyLicenseTextFile(repoRoot, expectedPath)
		assert.NoError(t, err)

		_, err = os.Stat(filepath.Join(repoRoot.Name(), "LICENSE.txt"))
		assert.ErrorIs(t, err, os.ErrNotExist)

		_, err = os.Stat(expectedPath)
		assert.ErrorIs(t, err, os.ErrNotExist)
	})

	t.Run("RepoLicensePathOutsideRepoRoot", func(t *testing.T) {
		repoRoot, err := os.OpenRoot(t.TempDir())
		require.NoError(t, err)
		defer repoRoot.Close()

		// Create a LICENSE.txt file in a different temp directory
		outsideDir := t.TempDir()
		outsideLicensePath := filepath.Join(outsideDir, "LICENSE.txt")

		err = copyLicenseTextFile(repoRoot, outsideLicensePath)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "is outside of the repoRoot")
	})

}
