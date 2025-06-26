// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package files

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLinkUpdateChecksum tests the UpdateChecksum method of the Link struct.
// This test verifies that:
// 1. An outdated link file (without checksum) can be updated correctly
// 2. An up-to-date link file (with correct checksum) doesn't need updating
// 3. The checksum calculation and file writing works properly
func TestLinkUpdateChecksum(t *testing.T) {
	// Create a temporary directory and copy test data to avoid modifying originals
	tempDir := t.TempDir()
	wd, err := os.Getwd()
	require.NoError(t, err)
	testDataSrc := filepath.Join(wd, "testdata")
	require.NoError(t, copyDir(testDataSrc, filepath.Join(tempDir, "testdata")))

	// Set up paths within the temporary directory
	basePath := filepath.Join(tempDir, "testdata/links")

	// Create an os.Root for secure file operations within tempDir
	root, err := os.OpenRoot(tempDir)
	require.NoError(t, err)

	// Test Case 1: Outdated link file (missing checksum)
	// Load a link file that points to an included file but has no checksum
	outdatedFile, err := NewLinkedFile(root, filepath.Join(basePath, "outdated.yml.link"))
	require.NoError(t, err)

	// Verify initial state: file should not be up-to-date and have no checksum
	assert.False(t, outdatedFile.UpToDate)
	assert.Empty(t, outdatedFile.LinkChecksum)

	// Update the checksum and verify it was actually updated
	updated, err := outdatedFile.UpdateChecksum()
	assert.NoError(t, err)
	assert.True(t, updated) // Should return true indicating an update occurred

	// Verify the checksum was calculated correctly (this is the SHA256 of the included file)
	assert.Equal(t, "d709feed45b708c9548a18ca48f3ad4f41be8d3f691f83d7417ca902a20e6c1e", outdatedFile.LinkChecksum)
	assert.True(t, outdatedFile.UpToDate)

	// Test Case 2: Up-to-date link file (already has correct checksum)
	// Load a link file that already has the correct checksum
	uptodateFile, err := NewLinkedFile(root, filepath.Join(basePath, "uptodate.yml.link"))
	assert.NoError(t, err)

	// Verify it's already up-to-date
	assert.True(t, uptodateFile.UpToDate)

	// Attempt to update - should return false since no update is needed
	updated, err = uptodateFile.UpdateChecksum()
	assert.NoError(t, err)
	assert.False(t, updated) // Should return false indicating no update was needed
}

// TestListLinkedFiles tests the ListLinkedFiles function that discovers and parses all link files in a directory.
// This test verifies that:
// 1. All .link files in the test directory are discovered (expects 2 files)
// 2. Each link file is correctly parsed with proper paths, checksums, and status
// 3. Outdated link files (without checksums) are identified correctly
// 4. Up-to-date link files (with matching checksums) are identified correctly
func TestListLinkedFiles(t *testing.T) {
	// Get current working directory to locate test data
	wd, err := os.Getwd()
	assert.NoError(t, err)
	basePath := filepath.Join(wd, filepath.FromSlash("testdata/links"))

	// Find the repository root to create a secure os.Root context
	root, err := FindRepositoryRoot()
	require.NoError(t, err)

	// List all linked files in the test directory
	linkedFiles, err := ListLinkedFiles(root, basePath)
	require.NoError(t, err)
	require.NotEmpty(t, linkedFiles)
	require.Len(t, linkedFiles, 2) // Expect exactly 2 link files in testdata

	// Verify first file (outdated.yml.link) - should be outdated (no checksum)
	assert.Equal(t, "outdated.yml.link", linkedFiles[0].LinkFilePath)
	assert.Empty(t, linkedFiles[0].LinkChecksum) // No checksum = outdated
	assert.Equal(t, "outdated.yml", linkedFiles[0].TargetFilePath(""))
	assert.Equal(t, "./included.yml", linkedFiles[0].IncludedFilePath)
	assert.Equal(t, "d709feed45b708c9548a18ca48f3ad4f41be8d3f691f83d7417ca902a20e6c1e", linkedFiles[0].IncludedFileContentsChecksum)
	assert.False(t, linkedFiles[0].UpToDate)

	// Verify second file (uptodate.yml.link) - should be up-to-date (has matching checksum)
	assert.Equal(t, "uptodate.yml.link", linkedFiles[1].LinkFilePath)
	assert.Equal(t, "d709feed45b708c9548a18ca48f3ad4f41be8d3f691f83d7417ca902a20e6c1e", linkedFiles[1].LinkChecksum)
	assert.Equal(t, "uptodate.yml", linkedFiles[1].TargetFilePath(""))
	assert.Equal(t, "./included.yml", linkedFiles[1].IncludedFilePath)
	assert.Equal(t, "d709feed45b708c9548a18ca48f3ad4f41be8d3f691f83d7417ca902a20e6c1e", linkedFiles[1].IncludedFileContentsChecksum)
	assert.True(t, linkedFiles[1].UpToDate)
}

// TestCopyFile tests the copyFromRoot helper function that securely copies files within the repository root.
// This test verifies that:
// 1. Files can be copied correctly within the repository boundaries using os.Root
// 2. The copied file has identical contents to the original
// 3. The copy operation works with the security abstraction (os.Root)
func TestCopyFile(t *testing.T) {
	fileA := "fileA.txt"
	fileB := "fileB.txt"
	tempDir := t.TempDir()

	// Setup cleanup to remove test files after the test
	t.Cleanup(func() { _ = os.Remove(filepath.Join(tempDir, fileA)) })
	t.Cleanup(func() { _ = os.Remove(filepath.Join(tempDir, fileB)) })

	// Create a source file with test content
	createDummyFile(t, filepath.Join(tempDir, fileA), "This is the content of the file.")

	// Create an os.Root for secure file operations within tempDir
	root, err := os.OpenRoot(tempDir)
	require.NoError(t, err)

	// Copy the file using the secure copyFromRoot function
	assert.NoError(t, copyFromRoot(root, fileA, fileB))

	// Verify that the copied file has identical content to the original
	equal, err := filesEqual(filepath.Join(tempDir, fileA), filepath.Join(tempDir, fileB))
	require.NoError(t, err)
	assert.True(t, equal, "files should be equal after copying")
}

// TestAreLinkedFilesUpToDate tests the AreLinkedFilesUpToDate function that identifies outdated link files.
// This test verifies that:
// 1. The function correctly identifies which link files are outdated (missing or incorrect checksums)
// 2. Only outdated files are returned (up-to-date files are filtered out)
// 3. The returned outdated file has correct metadata and status information
func TestAreLinkedFilesUpToDate(t *testing.T) {
	// Get current working directory to locate test data
	wd, err := os.Getwd()
	assert.NoError(t, err)
	basePath := filepath.Join(wd, filepath.FromSlash("testdata/links"))

	// Find the repository root to create a secure os.Root context
	root, err := FindRepositoryRoot()
	require.NoError(t, err)

	// Get all outdated linked files from the test directory
	linkedFiles, err := AreLinkedFilesUpToDate(root, basePath)
	assert.NoError(t, err)
	assert.NotEmpty(t, linkedFiles)
	assert.Len(t, linkedFiles, 1) // Expect exactly 1 outdated file (outdated.yml.link)

	// Verify the outdated file details
	assert.Equal(t, "outdated.yml.link", linkedFiles[0].LinkFilePath)
	assert.Empty(t, linkedFiles[0].LinkChecksum) // No checksum indicates outdated
	assert.Equal(t, "outdated.yml", linkedFiles[0].TargetFilePath(""))
	assert.Equal(t, "./included.yml", linkedFiles[0].IncludedFilePath)
	assert.Equal(t, "d709feed45b708c9548a18ca48f3ad4f41be8d3f691f83d7417ca902a20e6c1e", linkedFiles[0].IncludedFileContentsChecksum)
	assert.False(t, linkedFiles[0].UpToDate)
}

// TestUpdateLinkedFilesChecksums tests the UpdateLinkedFilesChecksums function that updates outdated link files.
// This test verifies that:
// 1. The function correctly identifies and updates outdated link files with proper checksums
// 2. Only outdated files are updated (up-to-date files are left unchanged)
// 3. After updating, the previously outdated file becomes up-to-date with correct checksum
// 4. The function returns details about which files were updated
func TestUpdateLinkedFilesChecksums(t *testing.T) {
	// Create a temporary directory and copy test data to avoid modifying originals
	tempDir := t.TempDir()
	wd, err := os.Getwd()
	require.NoError(t, err)
	testDataSrc := filepath.Join(wd, "testdata")
	require.NoError(t, copyDir(testDataSrc, filepath.Join(tempDir, "testdata")))

	// Set up paths within the temporary directory
	basePath := filepath.Join(tempDir, "testdata/links")

	// Create an os.Root for secure file operations within tempDir
	root, err := os.OpenRoot(tempDir)
	require.NoError(t, err)

	// Update checksums for all outdated linked files
	updated, err := UpdateLinkedFilesChecksums(root, basePath)

	// Verify the update operation succeeded
	assert.NoError(t, err)
	assert.NotEmpty(t, updated)
	assert.Len(t, updated, 1) // Expect exactly 1 file was updated (outdated.yml.link)

	// Verify the updated file is now up-to-date with correct checksum
	assert.True(t, updated[0].UpToDate)
	assert.Equal(t, "d709feed45b708c9548a18ca48f3ad4f41be8d3f691f83d7417ca902a20e6c1e", updated[0].LinkChecksum)
}

// TestLinkedFilesByPackageFrom tests the LinkedFilesByPackageFrom function that organizes linked files by package.
// This test verifies that:
// 1. The function correctly discovers and groups linked files by their source packages
// 2. The returned structure properly maps package names to their linked files
// 3. File paths are correctly constructed and resolved relative to the package directories
// 4. The specific test package "testpackage" is found with its expected linked file
func TestLinkedFilesByPackageFrom(t *testing.T) {
	// Get current working directory to locate test data
	wd, err := os.Getwd()
	assert.NoError(t, err)
	basePath := filepath.Join(wd, filepath.FromSlash("testdata/links"))

	// Find the repository root to create a secure os.Root context
	root, err := FindRepositoryRoot()
	require.NoError(t, err)

	// Get linked files organized by package
	packageLinks, err := LinkedFilesByPackageFrom(root, basePath)
	assert.NoError(t, err)
	assert.NotEmpty(t, packageLinks)
	assert.Len(t, packageLinks, 1) // Expect 1 package group

	// Verify the package structure
	pkg := packageLinks[0]
	assert.Equal(t, "testpackage", pkg.PackageName)
	assert.NotEmpty(t, pkg.Links)
	assert.Len(t, pkg.Links, 1) // Expect 1 linked file in testpackage

	// Verify the linked file path ends with the expected relative path
	match := strings.HasSuffix(
		filepath.ToSlash(pkg.Links[0]),
		"/testdata/testpackage/included.yml.link",
	)
	assert.True(t, match)
}

// TestIncludeLinkedFiles tests the IncludeLinkedFiles function that copies linked files to a destination directory.
// This test verifies that:
// 1. Linked files are correctly discovered from a source package directory
// 2. The included files are copied to the specified destination directory
// 3. The copied files have identical content to their original included files
// 4. The target file paths are correctly constructed in the destination
// 5. The function works with a temporary directory setup to avoid affecting real files
func TestIncludeLinkedFiles(t *testing.T) {
	// Get current working directory to locate test data
	wd, err := os.Getwd()
	assert.NoError(t, err)
	testPkg := filepath.Join(wd, filepath.FromSlash("testdata"))

	// Create a temporary directory and copy test data to avoid modifying originals
	tempDir := t.TempDir()
	require.NoError(t, copyDir(testPkg, filepath.Join(tempDir, "testdata")))

	// Set up source and destination directories
	fromDir := filepath.Join(tempDir, "testdata/testpackage")
	toDir := filepath.Join(tempDir, "dest")

	// Create an os.Root for secure file operations within tempDir
	root, err := os.OpenRoot(tempDir)
	require.NoError(t, err)

	// Include (copy) all linked files from source to destination
	linkedFiles, err := IncludeLinkedFiles(root, fromDir, toDir)
	assert.NoError(t, err)
	require.Equal(t, 1, len(linkedFiles)) // Expect 1 linked file to be processed

	// Verify the target file was created in the destination directory
	assert.FileExists(t, linkedFiles[0].TargetFilePath(toDir))

	// Verify the copied file has identical content to the original included file
	equal, err := filesEqual(
		filepath.Join(linkedFiles[0].WorkDir, filepath.FromSlash(linkedFiles[0].IncludedFilePath)),
		linkedFiles[0].TargetFilePath(toDir),
	)
	assert.NoError(t, err)
	assert.True(t, equal, "files should be equal after copying")
}

// createDummyFile is a test helper that creates a file with specified content.
// This helper ensures the file is created successfully and writes the provided content to it.
func createDummyFile(t *testing.T, filename, content string) {
	file, err := os.Create(filename)
	assert.NoError(t, err)
	defer file.Close()
	_, err = file.WriteString(content)
	assert.NoError(t, err)
}

// filesEqual is a test helper that compares the contents of two files for equality.
// Returns true if both files exist and have identical content, false otherwise.
// Any error reading the files is returned to the caller.
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

// copyDir recursively copies a directory from src to dst.
// This helper function is used in tests to create isolated copies of test data
// to avoid modifying the original test files during test execution.
func copyDir(src, dst string) error {
	// Get properties of source.
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	// Create the destination directory.
	err = os.MkdirAll(dst, srcInfo.Mode())
	if err != nil {
		return err
	}

	// Read the source directory.
	dir, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	// Copy all entries.
	for _, entry := range dir {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			// If it's a directory, recurse.
			err = copyDir(srcPath, dstPath)
			if err != nil {
				return err
			}
		} else {
			// It's a file, so copy it.
			err = copyFile(srcPath, dstPath)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// copyFile copies a single file from src to dst.
// This helper function is used by copyDir to copy individual files while preserving
// their content and permissions. It efficiently copies large files using io.Copy.
func copyFile(src, dst string) error {
	// Open the source file for reading.
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	// Create the destination file, overwriting it if it already exists.
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	// Use io.Copy to efficiently copy the contents from source to destination.
	_, err = io.Copy(out, in)
	if err != nil {
		return err
	}

	// Get the file information (metadata) from the source file.
	info, err := os.Stat(src)
	if err != nil {
		return err
	}

	// Set the permissions (mode) of the destination file to match the source file.
	return os.Chmod(dst, info.Mode())
}

func TestNewLinkedFileRejectsPathTraversal(t *testing.T) {
	tempDir := t.TempDir()

	// Create a repository root
	repoDir := filepath.Join(tempDir, "repo")
	err := os.MkdirAll(repoDir, 0755)
	require.NoError(t, err)

	// Create a file outside the repository that we'll try to link to
	outsideDir := filepath.Join(tempDir, "outside")
	err = os.MkdirAll(outsideDir, 0755)
	require.NoError(t, err)
	outsideFile := filepath.Join(outsideDir, "secret.txt")
	err = os.WriteFile(outsideFile, []byte("secret content"), 0644)
	require.NoError(t, err)

	// Create a subdirectory in the repo for our link file
	linkDir := filepath.Join(repoDir, "links")
	err = os.MkdirAll(linkDir, 0755)
	require.NoError(t, err)

	// Create a valid file within the repository for testing
	validFile := filepath.Join(linkDir, "valid.txt")
	err = os.WriteFile(validFile, []byte("valid content"), 0644)
	require.NoError(t, err)

	// Test cases with different path traversal attempts
	testCases := []struct {
		name         string
		linkContent  string
		expectError  bool
		errorMessage string
	}{
		{
			name:         "simple parent directory escape",
			linkContent:  "../../../outside/secret.txt",
			expectError:  true,
			errorMessage: "escapes the repository root",
		},
		{
			name:         "absolute path escape",
			linkContent:  outsideFile,
			expectError:  true,
			errorMessage: "escapes the repository root",
		},
		{
			name:         "complex path traversal",
			linkContent:  "../../repo/../outside/secret.txt",
			expectError:  true,
			errorMessage: "escapes the repository root",
		},
		{
			name:         "valid relative path",
			linkContent:  "./valid.txt",
			expectError:  false,
			errorMessage: "",
		},
	}

	root, err := os.OpenRoot(repoDir)
	require.NoError(t, err)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create the malicious link file
			linkFile := filepath.Join(linkDir, "malicious.link")
			err := os.WriteFile(linkFile, []byte(tc.linkContent), 0644)
			require.NoError(t, err)

			// Attempt to create a NewLinkedFile
			_, err = NewLinkedFile(root, linkFile)

			if tc.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.errorMessage)
			} else {
				assert.NoError(t, err)
			}

			// Clean up the link file for next iteration
			os.Remove(linkFile)
		})
	}
}

func TestLinksFSSecurityIsolation(t *testing.T) {
	tempDir := t.TempDir()

	// Create a repository root
	repoDir := filepath.Join(tempDir, "repo")
	err := os.MkdirAll(repoDir, 0755)
	require.NoError(t, err)

	// Create a working directory inside repo
	workDir := filepath.Join(repoDir, "work")
	err = os.MkdirAll(workDir, 0755)
	require.NoError(t, err)

	// Create a valid included file in the repo
	includedFile := filepath.Join(workDir, "included.txt")
	err = os.WriteFile(includedFile, []byte("included content"), 0644)
	require.NoError(t, err)

	// Create a link file that points to the included file with proper checksum
	linkFile := filepath.Join(workDir, "test.txt.link")
	// Calculate the checksum of the included file
	hash := sha256.Sum256([]byte("included content"))
	checksum := hex.EncodeToString(hash[:])
	linkContent := fmt.Sprintf("./included.txt %s", checksum)
	err = os.WriteFile(linkFile, []byte(linkContent), 0644)
	require.NoError(t, err)

	// Create LinksFS
	root, err := os.OpenRoot(repoDir)
	require.NoError(t, err)

	// Get the relative path from repo root to work directory
	relWorkDir, err := filepath.Rel(repoDir, workDir)
	require.NoError(t, err)

	lfs := NewLinksFS(root, relWorkDir)

	// Test opening the linked file - this should work and use the repository root
	file, err := lfs.Open("test.txt.link")
	require.NoError(t, err)
	defer file.Close()

	// Read the content to ensure it's correct
	content, err := io.ReadAll(file)
	require.NoError(t, err)
	assert.Equal(t, "included content", string(content))
}
