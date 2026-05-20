// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package builder

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFindRepositoryLicense(t *testing.T) {
	t.Run("FileExists", func(t *testing.T) {
		repositoryRoot, err := os.OpenRoot(t.TempDir())
		require.NoError(t, err)
		defer repositoryRoot.Close()

		filename := "LICENSE.txt"
		// Create a LICENSE.txt file in the temp directory
		err = repositoryRoot.WriteFile(filename, []byte("license content"), 0644)
		require.NoError(t, err)

		expectedPath := filepath.Join(repositoryRoot.Name(), filename)

		path, err := findRepositoryLicensePath(repositoryRoot, filename)
		require.NoError(t, err)
		assert.Equal(t, expectedPath, path)
	})

	t.Run("FileDoesNotExist", func(t *testing.T) {
		repositoryRoot, err := os.OpenRoot(t.TempDir())
		require.NoError(t, err)
		defer repositoryRoot.Close()

		path, err := findRepositoryLicensePath(repositoryRoot, "NON_EXISTENT_LICENSE.txt")
		require.Error(t, err)
		assert.Empty(t, path)
		assert.ErrorIs(t, err, os.ErrNotExist)
	})

	t.Run("FileOutsideRoot", func(t *testing.T) {
		repositoryRoot, err := os.OpenRoot(t.TempDir())
		require.NoError(t, err)
		defer repositoryRoot.Close()

		path, err := findRepositoryLicensePath(repositoryRoot, filepath.Join("..", "..", "out.txt"))
		require.Error(t, err)
		assert.Empty(t, path)
		assert.ErrorContains(t, err, "path escapes from parent")
	})

}

func TestCopyLicenseTextFile_UsesExistingLicenseFile(t *testing.T) {

	t.Run("targetLicensePath is relative", func(t *testing.T) {
		repositoryRoot, err := os.OpenRoot(t.TempDir())
		require.NoError(t, err)
		defer repositoryRoot.Close()

		licensePathRel := filepath.Join("LICENSE.txt")

		// Should not attempt to copy, just return nil
		err = copyLicenseTextFile(repositoryRoot, licensePathRel)
		assert.Error(t, err)

	})

	t.Run("targetLicensePath is absolute", func(t *testing.T) {
		repositoryRoot, err := os.OpenRoot(t.TempDir())
		require.NoError(t, err)
		defer repositoryRoot.Close()

		filename := "LICENSE.txt"

		err = repositoryRoot.WriteFile(filename, []byte("existing license"), 0644)
		require.NoError(t, err)

		targetLicensePath := filepath.Join(repositoryRoot.Name(), filename)
		// Should not attempt to copy, just return nil
		err = copyLicenseTextFile(repositoryRoot, targetLicensePath)
		assert.NoError(t, err)

		// License file should remain unchanged
		content, err := repositoryRoot.ReadFile(filename)
		require.NoError(t, err)
		assert.Equal(t, "existing license", string(content))

	})

	t.Run("ExistingDirectory", func(t *testing.T) {
		repositoryRoot, err := os.OpenRoot(t.TempDir())
		require.NoError(t, err)
		defer repositoryRoot.Close()

		targetLicensePath := filepath.Join(t.TempDir())

		err = copyLicenseTextFile(repositoryRoot, targetLicensePath)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "is a directory")
	})

	t.Run("RepoLicenseDefaultFileName", func(t *testing.T) {
		repositoryRoot, err := os.OpenRoot(t.TempDir())
		require.NoError(t, err)
		defer repositoryRoot.Close()

		targetLicensePath := filepath.Join(t.TempDir(), "REPO_LICENSE.txt")
		err = os.WriteFile(filepath.Join(repositoryRoot.Name(), licenseTextFileName), []byte("repo license"), 0644)
		require.NoError(t, err)

		err = copyLicenseTextFile(repositoryRoot, targetLicensePath)
		assert.NoError(t, err)

		content, err := os.ReadFile(targetLicensePath)
		require.NoError(t, err)
		assert.Equal(t, "repo license", string(content))
	})

	t.Run("RepoLicenseCustomFileName", func(t *testing.T) {
		repositoryRoot, err := os.OpenRoot(t.TempDir())
		require.NoError(t, err)
		defer repositoryRoot.Close()

		filename := "CUSTOM_LICENSE.txt"
		t.Setenv(repositoryLicenseEnv, filename)

		// target license file path outside the repository root
		targetLicensePath := filepath.Join(t.TempDir(), "REPO_LICENSE.txt")

		// original license file path
		err = repositoryRoot.WriteFile(filename, []byte("repo license"), 0644)
		require.NoError(t, err)

		err = copyLicenseTextFile(repositoryRoot, targetLicensePath)
		assert.NoError(t, err)

		// read outside the repository root
		content, err := os.ReadFile(targetLicensePath)
		require.NoError(t, err)
		assert.Equal(t, "repo license", string(content))
	})

	t.Run("RepoLicenseFileDoesNotExist", func(t *testing.T) {
		repositoryRoot, err := os.OpenRoot(t.TempDir())
		require.NoError(t, err)
		defer repositoryRoot.Close()

		targetLicensePath := filepath.Join(t.TempDir(), "REPO_LICENSE.txt")
		err = copyLicenseTextFile(repositoryRoot, targetLicensePath)
		assert.NoError(t, err)

		_, err = repositoryRoot.Stat("LICENSE.txt")
		assert.ErrorIs(t, err, os.ErrNotExist)

		_, err = repositoryRoot.Stat("REPO_LICENSE.txt")
		assert.ErrorIs(t, err, os.ErrNotExist)
	})

}

const builtPackageManifestTestPkgName = "testpkg"

func writeBuiltPackageTestManifest(t *testing.T, dir, version string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "manifest.yml"),
		[]byte(fmt.Sprintf("name: %s\ntitle: Test Package\nversion: %s\ntype: integration\nformat_version: 3.0.0\n", builtPackageManifestTestPkgName, version)),
		0o644,
	))
}

// TestReadBuiltPackageManifest pins the strict behavior of ReadBuiltPackageManifest:
// it must locate the built tree at build/packages/<name>/<version>/ matching the
// version in the source manifest, and fail otherwise. The script tester relies on
// this strictness — when -version is set, it deliberately bypasses
// ReadBuiltPackageManifest because it would resolve to a non-existent path
// (the EPR version is not built locally).
func TestReadBuiltPackageManifest(t *testing.T) {
	t.Run("ReturnsBuiltTree", func(t *testing.T) {
		root := t.TempDir()
		t.Chdir(root)

		srcVersion := "1.2.3"
		packageRoot := filepath.Join(root, "packages", builtPackageManifestTestPkgName)
		writeBuiltPackageTestManifest(t, packageRoot, srcVersion)

		builtRoot := filepath.Join(root, "build", "packages", builtPackageManifestTestPkgName, srcVersion)
		writeBuiltPackageTestManifest(t, builtRoot, srcVersion)

		gotRoot, gotPkg, err := ReadBuiltPackageManifest(packageRoot)
		require.NoError(t, err)
		assert.Equal(t, builtRoot, gotRoot)
		assert.Equal(t, builtPackageManifestTestPkgName, gotPkg.Name)
		assert.Equal(t, srcVersion, gotPkg.Version)
	})

	t.Run("ErrorWhenBuiltTreeMissing", func(t *testing.T) {
		// No build/ directory and no .git — strict mode must fail.
		root := t.TempDir()
		t.Chdir(root)

		packageRoot := filepath.Join(root, "epr", builtPackageManifestTestPkgName, "1.0.0")
		writeBuiltPackageTestManifest(t, packageRoot, "1.0.0")

		_, _, err := ReadBuiltPackageManifest(packageRoot)
		require.Error(t, err)
	})

	t.Run("ErrorWhenBuiltVersionDiffers", func(t *testing.T) {
		// Built tree exists but for a different version — strict mode must fail.
		// This is the exact on-disk shape that triggered #3552: the EPR-extracted
		// package is at one version and the local build/ is at another.
		root := t.TempDir()
		t.Chdir(root)

		packageRoot := filepath.Join(root, "epr", builtPackageManifestTestPkgName, "1.1.0")
		writeBuiltPackageTestManifest(t, packageRoot, "1.1.0")

		builtDev := filepath.Join(root, "build", "packages", builtPackageManifestTestPkgName, "1.2.3")
		writeBuiltPackageTestManifest(t, builtDev, "1.2.3")

		_, _, err := ReadBuiltPackageManifest(packageRoot)
		require.Error(t, err)
	})
}
