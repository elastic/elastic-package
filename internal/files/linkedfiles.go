// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package files

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/packages"
)

const linkExtension = ".link"

// PackageLinks represents linked files grouped by package.
type PackageLinks struct {
	PackageName string
	Links       []string
}

// CheckLinkedFiles checks if all linked files in the given directory are up-to-date.
// Returns a list of outdated links that need updating.
func CheckLinkedFiles(fromDir string) ([]Link, error) {
	repoRoot, err := FindRepositoryRootDirectory()
	if err != nil {
		return nil, fmt.Errorf("finding repository root: %w", err)
	}

	root, err := os.OpenRoot(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("opening repository root: %w", err)
	}

	return AreLinkedFilesUpToDate(root, fromDir)
}

// UpdateLinkedFiles updates the checksums of all outdated linked files in the given directory.
// Returns a list of links that were updated.
func UpdateLinkedFiles(fromDir string) ([]Link, error) {
	repoRoot, err := FindRepositoryRootDirectory()
	if err != nil {
		return nil, fmt.Errorf("finding repository root: %w", err)
	}

	root, err := os.OpenRoot(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("opening repository root: %w", err)
	}

	return UpdateLinkedFilesChecksums(root, fromDir)
}

// IncludeLinkedFilesFromPath copies all linked files from the source directory to the target directory.
// This is used during package building to include linked files in the build output.
func IncludeLinkedFilesFromPath(fromDir, toDir string) ([]Link, error) {
	repoRoot, err := FindRepositoryRootDirectory()
	if err != nil {
		return nil, fmt.Errorf("finding repository root: %w", err)
	}

	root, err := os.OpenRoot(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("opening repository root: %w", err)
	}

	return IncludeLinkedFiles(root, fromDir, toDir)
}

// ListLinkedFilesByPackage returns a mapping of packages to their linked files that reference
// files from the given directory.
func ListLinkedFilesByPackage(fromDir string) ([]PackageLinks, error) {
	repoRoot, err := FindRepositoryRootDirectory()
	if err != nil {
		return nil, fmt.Errorf("finding repository root: %w", err)
	}

	root, err := os.OpenRoot(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("opening repository root: %w", err)
	}

	return LinkedFilesByPackageFrom(root, fromDir)
}

// CreateLinksFSFromPath creates a LinksFS for the given directory within the repository.
func CreateLinksFSFromPath(workDir string) (*LinksFS, error) {
	repoRoot, err := FindRepositoryRootDirectory()
	if err != nil {
		return nil, fmt.Errorf("finding repository root: %w", err)
	}

	root, err := os.OpenRoot(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("opening repository root: %w", err)
	}

	return NewLinksFS(root, workDir), nil
}

var _ fs.FS = (*LinksFS)(nil)

// LinksFS is a filesystem that handles linked files.
// It wraps another filesystem and checks for linked files with the ".link" extension.
// If a linked file is found, it reads the link file to determine the included file
// and its checksum. If the included file is up to date, it returns the included file.
// Otherwise, it returns an error.
type LinksFS struct {
	repoRoot *os.Root // The root of the repository, used to check if paths are within the repository.
	workDir  string
	inner    fs.FS
}

// NewLinksFS creates a new LinksFS.
func NewLinksFS(repoRoot *os.Root, workDir string) *LinksFS {
	return &LinksFS{repoRoot: repoRoot, workDir: workDir, inner: os.DirFS(workDir)}
}

// Open opens a file in the filesystem.
func (lfs *LinksFS) Open(name string) (fs.File, error) {
	// For non-link files, use the inner filesystem
	if filepath.Ext(name) != linkExtension {
		return lfs.inner.Open(name)
	}

	// For link files, construct the full path
	var linkFilePath string
	if filepath.IsAbs(lfs.workDir) {
		linkFilePath = filepath.Join(lfs.workDir, name)
	} else {
		linkFilePath = filepath.Join(lfs.repoRoot.Name(), lfs.workDir, name)
	}

	l, err := NewLinkedFile(lfs.repoRoot, linkFilePath)
	if err != nil {
		return nil, err
	}
	if !l.UpToDate {
		return nil, fmt.Errorf("linked file %s is not up to date", name)
	}

	// Calculate the included file path
	var includedPath string
	if filepath.IsAbs(lfs.workDir) {
		includedPath = filepath.Join(lfs.workDir, filepath.Dir(name), l.IncludedFilePath)
	} else {
		includedPath = filepath.Join(lfs.repoRoot.Name(), lfs.workDir, filepath.Dir(name), l.IncludedFilePath)
	}

	// Convert to relative path from repository root for secure access of target file
	relativePath, err := filepath.Rel(lfs.repoRoot.Name(), includedPath)
	if err != nil {
		return nil, fmt.Errorf("could not get relative path: %w", err)
	}

	return lfs.repoRoot.Open(relativePath)
}

// ReadFile reads a file from the filesystem.
func (lfs *LinksFS) ReadFile(name string) ([]byte, error) {
	f, err := lfs.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}
	return b, nil
}

// A Link represents a linked file.
// It contains the path to the link file, the checksum of the link file,
// the path to the included file, and the checksum of the included file contents.
// It also contains a boolean indicating whether the link is up to date.
type Link struct {
	WorkDir string

	LinkFilePath    string
	LinkChecksum    string
	LinkPackageName string

	IncludedFilePath             string
	IncludedFileContentsChecksum string
	IncludedPackageName          string

	UpToDate bool
}

// NewLinkedFile creates a new Link from the given link file path.
func NewLinkedFile(root *os.Root, linkFilePath string) (Link, error) {
	var l Link
	l.WorkDir = filepath.Dir(linkFilePath)
	if linkPackageRoot, _, _ := packages.FindPackageRootFrom(l.WorkDir); linkPackageRoot != "" {
		l.LinkPackageName = filepath.Base(linkPackageRoot)
	}

	firstLine, err := readFirstLine(linkFilePath)
	if err != nil {
		return Link{}, err
	}
	l.LinkFilePath, err = filepath.Rel(l.WorkDir, linkFilePath)
	if err != nil {
		return Link{}, fmt.Errorf("could not get relative path: %w", err)
	}

	fields := strings.Fields(firstLine)
	if len(fields) == 0 {
		return Link{}, fmt.Errorf("link file %s is empty or has no valid content", linkFilePath)
	}
	if len(fields) > 2 {
		return Link{}, fmt.Errorf("link file %s has invalid format: expected 1 or 2 fields, got %d", linkFilePath, len(fields))
	}
	l.IncludedFilePath = fields[0]
	if len(fields) == 2 {
		l.LinkChecksum = fields[1]
	}

	pathName := filepath.Clean(filepath.Join(l.WorkDir, filepath.FromSlash(l.IncludedFilePath)))

	inRoot, err := pathIsInRepositoryRoot(root, pathName)
	if err != nil {
		return Link{}, fmt.Errorf("could not check if path %s is in repository root: %w", pathName, err)
	}
	if !inRoot {
		return Link{}, fmt.Errorf("path %s escapes the repository root", pathName)
	}

	// Store the original absolute path for package root detection
	originalAbsPath := pathName

	// Convert to relative path for secure access of target file
	if filepath.IsAbs(pathName) {
		pathName, err = filepath.Rel(root.Name(), pathName)
		if err != nil {
			return Link{}, fmt.Errorf("could not get relative path: %w", err)
		}
	}

	if _, err := root.Stat(pathName); err != nil {
		return Link{}, err
	}

	cs, err := getLinkedFileChecksumFromRoot(root, pathName)
	if err != nil {
		return Link{}, fmt.Errorf("could not collect file %s: %w", l.IncludedFilePath, err)
	}
	if l.LinkChecksum == cs {
		l.UpToDate = true
	}
	l.IncludedFileContentsChecksum = cs

	if includedPackageRoot, _, _ := packages.FindPackageRootFrom(filepath.Dir(originalAbsPath)); includedPackageRoot != "" {
		l.IncludedPackageName = filepath.Base(includedPackageRoot)
	}

	return l, nil
}

// UpdateChecksum function updates the checksum of the linked file.
// It returns true if the checksum was updated, false if it was already up-to-date.
func (l *Link) UpdateChecksum() (bool, error) {
	if l.UpToDate {
		return false, nil
	}
	if l.IncludedFilePath == "" {
		return false, fmt.Errorf("included file path is empty for link file %s", l.LinkFilePath)
	}
	if l.IncludedFileContentsChecksum == "" {
		return false, fmt.Errorf("checksum is empty for included file %s", l.IncludedFilePath)
	}
	newContent := fmt.Sprintf("%v %v", filepath.ToSlash(l.IncludedFilePath), l.IncludedFileContentsChecksum)
	if err := writeFile(filepath.Join(l.WorkDir, l.LinkFilePath), []byte(newContent)); err != nil {
		return false, fmt.Errorf("could not update checksum for link file %s: %w", l.LinkFilePath, err)
	}
	l.LinkChecksum = l.IncludedFileContentsChecksum
	l.UpToDate = true
	return true, nil
}

// TargetFilePath returns the path where the linked file should be written.
// If workDir is provided, it uses that as the base directory, otherwise uses the link's WorkDir.
func (l *Link) TargetFilePath(workDir ...string) string {
	targetFilePath := filepath.FromSlash(strings.TrimSuffix(l.LinkFilePath, linkExtension))
	wd := l.WorkDir
	if len(workDir) > 0 {
		wd = workDir[0]
	}
	return filepath.Join(wd, targetFilePath)
}

// IncludeLinkedFiles function includes linked files from the source
// directory to the target directory.
// It returns a slice of Link structs representing the included files.
// It also updates the checksum of the linked files.
// Both directories must be relative to the root.
func IncludeLinkedFiles(root *os.Root, fromDir, toDir string) ([]Link, error) {
	links, err := ListLinkedFiles(root, fromDir)
	if err != nil {
		return nil, fmt.Errorf("including linked files failed: %w", err)
	}
	for _, l := range links {
		if _, err := l.UpdateChecksum(); err != nil {
			return nil, fmt.Errorf("could not update checksum for file %s: %w", l.LinkFilePath, err)
		}
		targetFilePath := l.TargetFilePath(toDir)
		if err := copyFromRoot(
			root,
			filepath.Join(l.WorkDir, filepath.FromSlash(l.IncludedFilePath)),
			targetFilePath,
		); err != nil {
			return nil, fmt.Errorf("could not write file %s: %w", targetFilePath, err)
		}
	}

	return links, nil
}

// ListLinkedFiles function returns a slice of Link structs representing linked files.
func ListLinkedFiles(root *os.Root, fromDir string) ([]Link, error) {
	var linkFiles []string
	if err := filepath.Walk(
		filepath.FromSlash(fromDir),
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() && strings.HasSuffix(info.Name(), linkExtension) {
				linkFiles = append(linkFiles, path)
			}
			return nil
		}); err != nil {
		return nil, err
	}

	links := make([]Link, len(linkFiles))

	for i, f := range linkFiles {
		l, err := NewLinkedFile(root, filepath.FromSlash(f))
		if err != nil {
			return nil, fmt.Errorf("could not initialize linked file %s: %w", f, err)
		}
		links[i] = l
	}

	return links, nil
}

// createDirInRoot function creates a directory and all its parents within the root.
func createDirInRoot(root *os.Root, dir string) error {
	dir = filepath.Clean(dir)
	if dir == "." || dir == "/" {
		return nil
	}

	// Check if the directory already exists
	if _, err := root.Stat(dir); err == nil {
		return nil
	}

	// Create parent directory first
	parent := filepath.Dir(dir)
	if parent != dir { // Avoid infinite recursion
		if err := createDirInRoot(root, parent); err != nil {
			return err
		}
	}

	// Create the directory
	return root.Mkdir(dir, 0700)
}

// copyFromRoot function copies a file from to to inside the root.
func copyFromRoot(root *os.Root, from, to string) error {
	var err error
	if filepath.IsAbs(from) {
		from, err = filepath.Rel(root.Name(), filepath.FromSlash(from))
		if err != nil {
			return fmt.Errorf("could not get relative path: %w", err)
		}
	}
	source, err := root.Open(from)
	if err != nil {
		return err
	}
	defer source.Close()

	if filepath.IsAbs(to) {
		to, err = filepath.Rel(root.Name(), filepath.FromSlash(to))
		if err != nil {
			return fmt.Errorf("could not get relative path: %w", err)
		}
	}
	dir := filepath.Dir(to)
	if _, err := root.Stat(dir); os.IsNotExist(err) {
		if err := createDirInRoot(root, dir); err != nil {
			return err
		}
	}
	destination, err := root.Create(to)
	if err != nil {
		return err
	}
	defer destination.Close()

	_, err = io.Copy(destination, source)
	return err
}

// writeFile function writes a byte slice to a file inside the root.
func writeFile(to string, b []byte) error {
	to = filepath.FromSlash(to)
	if _, err := os.Stat(filepath.Dir(to)); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(to), 0700); err != nil {
			return err
		}
	}
	return os.WriteFile(to, b, 0644)
}

// AreLinkedFilesUpToDate function checks if all the linked files are up-to-date.
func AreLinkedFilesUpToDate(root *os.Root, fromDir string) ([]Link, error) {
	links, err := ListLinkedFiles(root, fromDir)
	if err != nil {
		return nil, fmt.Errorf("checking linked files failed: %w", err)
	}

	var outdated []Link
	for _, l := range links {
		logger.Debugf("Check if %s is up-to-date", l.LinkFilePath)
		if !l.UpToDate {
			outdated = append(outdated, l)
		}
	}

	return outdated, nil
}

// UpdateLinkedFilesChecksums function updates the checksums of the linked files.
// It returns a slice of updated links.
// If no links were updated, it returns an empty slice.
func UpdateLinkedFilesChecksums(root *os.Root, fromDir string) ([]Link, error) {
	links, err := ListLinkedFiles(root, fromDir)
	if err != nil {
		return nil, fmt.Errorf("updating linked files checksums failed: %w", err)
	}

	var updatedLinks []Link
	for _, l := range links {
		updated, err := l.UpdateChecksum()
		if err != nil {
			return nil, fmt.Errorf("updating linked files checksums failed: %w", err)
		}
		if updated {
			updatedLinks = append(updatedLinks, l)
		}
	}

	return updatedLinks, nil
}

// LinkedFilesByPackageFrom function returns a slice of PackageLinks containing linked files grouped by package.
// Each PackageLinks contains the package name and a slice of linked file paths.
func LinkedFilesByPackageFrom(root *os.Root, fromDir string) ([]PackageLinks, error) {
	// we list linked files from all the root directory
	// to check which ones are linked to the 'fromDir' package
	links, err := ListLinkedFiles(root, root.Name())
	if err != nil {
		return nil, fmt.Errorf("listing linked files failed: %w", err)
	}

	var packageName string
	if packageRoot, _, _ := packages.FindPackageRootFrom(fromDir); packageRoot != "" {
		packageName = filepath.Base(packageRoot)
	}
	byPackageMap := map[string][]string{}
	for _, l := range links {
		if l.LinkPackageName == l.IncludedPackageName ||
			packageName != l.IncludedPackageName {
			continue
		}
		byPackageMap[l.LinkPackageName] = append(byPackageMap[l.LinkPackageName], filepath.Join(l.WorkDir, l.LinkFilePath))
	}

	var packages []string
	for p := range byPackageMap {
		packages = append(packages, p)
	}
	slices.Sort(packages)

	var result []PackageLinks
	for _, p := range packages {
		result = append(result, PackageLinks{
			PackageName: p,
			Links:       byPackageMap[p],
		})
	}
	return result, nil
}

// getLinkedFileChecksumFromRoot calculates the SHA256 checksum of a file using root-relative access.
func getLinkedFileChecksumFromRoot(root *os.Root, relativePath string) (string, error) {
	file, err := root.Open(filepath.FromSlash(relativePath))
	if err != nil {
		return "", err
	}
	defer file.Close()

	b, err := io.ReadAll(file)
	if err != nil {
		return "", err
	}
	cs, err := checksum(b)
	if err != nil {
		return "", err
	}
	return cs, nil
}

// readFirstLine reads and returns the first line of a file.
func readFirstLine(filePath string) (string, error) {
	file, err := os.Open(filepath.FromSlash(filePath))
	if err != nil {
		return "", err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	if scanner.Scan() {
		return scanner.Text(), nil
	}

	if err := scanner.Err(); err != nil {
		return "", err
	}

	return "", fmt.Errorf("file is empty or first line is missing")
}

// checksum calculates the SHA256 checksum of a byte slice.
func checksum(b []byte) (string, error) {
	hash := sha256.Sum256(b)
	return hex.EncodeToString(hash[:]), nil
}

// pathIsInRepositoryRoot checks if a path is within the repository root and doesn't escape it.
func pathIsInRepositoryRoot(root *os.Root, path string) (bool, error) {
	path = filepath.FromSlash(path)
	var err error
	if filepath.IsAbs(path) {
		path, err = filepath.Rel(root.Name(), path)
		if err != nil {
			return false, fmt.Errorf("could not get relative path: %w", err)
		}
	}

	// Clean the path to resolve any ".." components
	cleanPath := filepath.Clean(path)

	// Check if the cleaned path tries to escape the root
	if strings.HasPrefix(cleanPath, "..") {
		return false, nil
	}

	if _, err := root.Stat(cleanPath); err != nil {
		return false, nil
	}
	return true, nil
}
