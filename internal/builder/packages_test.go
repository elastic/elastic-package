package builder

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFindRepositoryLicense_FileExists(t *testing.T) {
	dir := t.TempDir()
	licensePath := filepath.Join(dir, "LICENSE.txt")
	err := os.WriteFile(licensePath, []byte("license content"), 0644)
	require.NoError(t, err)

	path, err := findRepositoryLicense(licensePath)
	require.NoError(t, err)
	assert.Equal(t, licensePath, path)
}

func TestFindRepositoryLicense_FileDoesNotExist(t *testing.T) {
	dir := t.TempDir()
	licensePath := filepath.Join(dir, "NON_EXISTENT_LICENSE.txt")

	path, err := findRepositoryLicense(licensePath)
	require.Error(t, err)
	assert.Empty(t, path)
}
func TestCopyLicenseTextFile_UsesExistingLicenseFile(t *testing.T) {
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
}

func TestCopyLicenseTextFile_CopiesFromRepo(t *testing.T) {
	repoRoot, err := os.OpenRoot(t.TempDir())
	require.NoError(t, err)
	defer repoRoot.Close()

	licensePath := filepath.Join(repoRoot.Name(), "LICENSE.txt")
	err = os.WriteFile(licensePath, []byte("repo license"), 0644)
	require.NoError(t, err)

	destDir := t.TempDir()
	destLicensePath := filepath.Join(destDir, "LICENSE.txt")

	err = copyLicenseTextFile(repoRoot, destLicensePath)
	assert.NoError(t, err)

	content, err := os.ReadFile(destLicensePath)
	require.NoError(t, err)
	assert.Equal(t, "repo license", string(content))
}

func TestCopyLicenseTextFile_NoRepoLicense_ReturnsNil(t *testing.T) {
	repoRoot, err := os.OpenRoot(t.TempDir())
	require.NoError(t, err)
	defer repoRoot.Close()

	destDir := t.TempDir()
	destLicensePath := filepath.Join(destDir, "LICENSE.txt")

	err = copyLicenseTextFile(repoRoot, destLicensePath)
	assert.NoError(t, err)

	_, err = os.Stat(destLicensePath)
	assert.True(t, os.IsNotExist(err))
}

func TestCopyLicenseTextFile_EnvOverridesLicenseName(t *testing.T) {
	repoRoot, err := os.OpenRoot(t.TempDir())
	require.NoError(t, err)
	defer repoRoot.Close()

	customLicenseName := "CUSTOM_LICENSE.txt"
	customLicensePath := filepath.Join(repoRoot.Name(), customLicenseName)
	err = os.WriteFile(customLicensePath, []byte("custom license"), 0644)
	require.NoError(t, err)

	destDir := t.TempDir()
	destLicensePath := filepath.Join(destDir, "LICENSE.txt")

	t.Setenv(repositoryLicenseEnv, customLicenseName)
	err = copyLicenseTextFile(repoRoot, destLicensePath)
	assert.NoError(t, err)

	content, err := os.ReadFile(destLicensePath)
	require.NoError(t, err)
	assert.Equal(t, "custom license", string(content))
}

func TestCopyLicenseTextFile_ErrorCopyingFile(t *testing.T) {
	repoRoot, err := os.OpenRoot(t.TempDir())
	require.NoError(t, err)
	defer repoRoot.Close()

	licensePath := filepath.Join(repoRoot.Name(), "LICENSE.txt")
	err = os.WriteFile(licensePath, []byte("repo license"), 0644)
	require.NoError(t, err)

	// Use a destination path that is a directory, so sh.Copy should fail
	destDir := t.TempDir()

	err = copyLicenseTextFile(repoRoot, destDir)
	assert.Error(t, err)
}
