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
		assert.ErrorIs(t, err, os.ErrNotExist)
	})

	t.Run("FileOutsideRoot", func(t *testing.T) {
		root, err := os.OpenRoot(t.TempDir())
		require.NoError(t, err)
		defer root.Close()

		path, err := findRepositoryLicensePath(root, "../../out.txt")
		require.Error(t, err)
		assert.Empty(t, path)
		assert.ErrorContains(t, err, "path escapes from parent")
	})

}

func TestCopyLicenseTextFile_UsesExistingLicenseFile(t *testing.T) {

	t.Run("ExistingFile_RelPath", func(t *testing.T) {

		repositoryRoot, err := os.OpenRoot(t.TempDir())
		require.NoError(t, err)
		defer repositoryRoot.Close()

		licensePathRel := filepath.Join("LICENSE.txt")
		err = os.WriteFile(filepath.Join(repositoryRoot.Name(), licensePathRel), []byte("existing license"), 0644)
		require.NoError(t, err)

		// Should not attempt to copy, just return nil
		err = copyLicenseTextFile(repositoryRoot, licensePathRel)
		assert.NoError(t, err)

		// License file should remain unchanged
		content, err := os.ReadFile(filepath.Join(repositoryRoot.Name(), licensePathRel))
		require.NoError(t, err)
		assert.Equal(t, "existing license", string(content))

	})

	t.Run("ExistingFile_AbsPath", func(t *testing.T) {

		repositoryRoot, err := os.OpenRoot(t.TempDir())
		require.NoError(t, err)
		defer repositoryRoot.Close()

		licensePath := filepath.Join(repositoryRoot.Name(), "LICENSE.txt")
		err = os.WriteFile(licensePath, []byte("existing license"), 0644)
		require.NoError(t, err)

		// Should not attempt to copy, just return nil
		err = copyLicenseTextFile(repositoryRoot, licensePath)
		assert.NoError(t, err)

		// License file should remain unchanged
		content, err := os.ReadFile(licensePath)
		require.NoError(t, err)
		assert.Equal(t, "existing license", string(content))

	})

	t.Run("ExistingDirectory", func(t *testing.T) {
		repositoryRoot, err := os.OpenRoot(t.TempDir())
		require.NoError(t, err)
		defer repositoryRoot.Close()

		err = copyLicenseTextFile(repositoryRoot, ".")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "is a directory")
	})

	t.Run("StatError", func(t *testing.T) {
		// Using a path that is likely invalid to trigger a stat error
		invalidPath := string([]byte{0})

		repositoryRoot, err := os.OpenRoot(t.TempDir())
		require.NoError(t, err)
		defer repositoryRoot.Close()

		err = copyLicenseTextFile(repositoryRoot, invalidPath)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "can't check license path")
	})

	t.Run("RepoLicenseDefaultFileName", func(t *testing.T) {
		repositoryRoot, err := os.OpenRoot(t.TempDir())
		require.NoError(t, err)
		defer repositoryRoot.Close()

		// original license file path
		err = os.WriteFile(filepath.Join(repositoryRoot.Name(), licenseTextFileName), []byte("repo license"), 0644)
		require.NoError(t, err)

		err = copyLicenseTextFile(repositoryRoot, "REPO_LICENSE.txt")
		assert.NoError(t, err)

		content, err := os.ReadFile(filepath.Join(repositoryRoot.Name(), "REPO_LICENSE.txt"))
		require.NoError(t, err)
		assert.Equal(t, "repo license", string(content))
	})

	t.Run("RepoLicenseCustomFileName", func(t *testing.T) {
		repositoryRoot, err := os.OpenRoot(t.TempDir())
		require.NoError(t, err)
		defer repositoryRoot.Close()

		// original license file path
		err = os.WriteFile(filepath.Join(repositoryRoot.Name(), "CUSTOM_LICENSE.txt"), []byte("repo license"), 0644)
		require.NoError(t, err)

		t.Setenv(repositoryLicenseEnv, "CUSTOM_LICENSE.txt")

		err = copyLicenseTextFile(repositoryRoot, "REPO_LICENSE.txt")
		assert.NoError(t, err)

		content, err := os.ReadFile(filepath.Join(repositoryRoot.Name(), "REPO_LICENSE.txt"))
		require.NoError(t, err)
		assert.Equal(t, "repo license", string(content))
	})

	t.Run("RepoLicenseFileDoesNotExist", func(t *testing.T) {
		repositoryRoot, err := os.OpenRoot(t.TempDir())
		require.NoError(t, err)
		defer repositoryRoot.Close()

		err = copyLicenseTextFile(repositoryRoot, "REPO_LICENSE.txt")
		assert.NoError(t, err)

		_, err = repositoryRoot.Stat("LICENSE.txt")
		assert.ErrorIs(t, err, os.ErrNotExist)

		_, err = repositoryRoot.Stat("REPO_LICENSE.txt")
		assert.ErrorIs(t, err, os.ErrNotExist)
	})

	t.Run("RepoLicensePathOutsideRepositoryRoot", func(t *testing.T) {
		repositoryRoot, err := os.OpenRoot(t.TempDir())
		require.NoError(t, err)
		defer repositoryRoot.Close()

		// Create a LICENSE.txt file in a different temp directory
		outsideDir := t.TempDir()
		outsideLicensePath := filepath.Join(outsideDir, "LICENSE.txt")

		err = copyLicenseTextFile(repositoryRoot, outsideLicensePath)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "path escapes from parent")
	})

}
