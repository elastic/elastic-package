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

var (
	errEmptyWorkDir         = fmt.Errorf("working directory is empty")
	errInvalidWorkDir       = fmt.Errorf("working directory must be an absolute path or a path relative to the repository root")
	errInvalidWorkDirNotDir = fmt.Errorf("working directory is not a directory")
	errFileNotUpToDate      = fmt.Errorf("linked file is not up to date")
)

const linkExtension = ".link"

// PackageLinks represents linked files grouped by package.
type PackageLinks struct {
	PackageName string
	Links       []string
}

// CreateLinksFSFromPath creates a LinksFS for the given directory within the repository.
//
// - workDir can be an absolute path or a path relative to the repository root.
// in both cases, it must point to a directory within the repository.
func CreateLinksFSFromPath(repoRoot *os.Root, workDir string) (*LinksFS, error) {
	if workDir == "" {
		return nil, errEmptyWorkDir
	}

	var relWorkDir string
	if filepath.IsAbs(workDir) {
		var err error
		relWorkDir, err = filepath.Rel(repoRoot.Name(), workDir)
		if err != nil {
			return nil, fmt.Errorf("unable to find rel path for %s: %w: %w", workDir, errInvalidWorkDir, err)
		}
	} else {
		relWorkDir = workDir
	}

	info, err := repoRoot.Stat(relWorkDir)
	if err != nil {
		return nil, fmt.Errorf("unable to stat %s: %w: %w", relWorkDir, errInvalidWorkDir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("working directory %s is not a directory: %w", relWorkDir, errInvalidWorkDirNotDir)
	}

	return &LinksFS{repoRoot: repoRoot, workDir: relWorkDir}, nil
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
}

// Open opens a file in the filesystem.
//
// - name can be an absolute path or a path relative to the workDir.
// If name is absolute, it must be within the workDir.
func (lfs *LinksFS) Open(name string) (fs.File, error) {
	// innerRoot is the filesystem rooted at the workDir
	// we use it to check if the file exists in the workDir
	// and to open non-link files directly
	innerRoot, err := lfs.repoRoot.OpenRoot(lfs.workDir)
	if err != nil {
		return nil, fmt.Errorf("could not open workDir in root: %w", err)
	}
	defer innerRoot.Close()

	relName := name
	if filepath.IsAbs(name) {
		var err error
		relName, err = filepath.Rel(filepath.Join(lfs.repoRoot.Name(), lfs.workDir), name)
		if err != nil {
			return nil, fmt.Errorf("could not make name relative to workDir: %w", err)
		}
	}
	_, err = innerRoot.Stat(relName)
	if err != nil {
		return nil, fmt.Errorf("file %s not found in workDir %s: %w", relName, lfs.workDir, err)
	}

	// For non-link files, use the inner filesystem
	if filepath.Ext(relName) != linkExtension {
		return innerRoot.Open(relName)
	}

	linkFilePath := filepath.Join(lfs.repoRoot.Name(), lfs.workDir, relName)
	l, err := newLinkedFile(lfs.repoRoot, linkFilePath)
	if err != nil {
		return nil, err
	}
	if !l.UpToDate {
		return nil, fmt.Errorf("%w: file %s", errFileNotUpToDate, relName)
	}

	// includedPath is the absolute path to the included file referenced at the link file
	// inside a link file, the path to the included file is relative to the link file location
	// so we need to join the directory of the link file with the included file path
	includedPath := filepath.Join(filepath.Dir(linkFilePath), filepath.FromSlash(l.IncludedFilePath))

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
	return io.ReadAll(f)
}

// CheckLinkedFiles checks if all linked files in the directory are up-to-date.
// Returns a list of outdated links that need updating.
func (lfs *LinksFS) CheckLinkedFiles() ([]Link, error) {
	return areLinkedFilesUpToDate(lfs.repoRoot, lfs.workDir)
}

// UpdateLinkedFiles updates the checksums of all outdated linked files in the directory.
// Returns a list of links that were updated.
func (lfs *LinksFS) UpdateLinkedFiles() ([]Link, error) {
	return updateLinkedFilesChecksums(lfs.repoRoot, lfs.workDir)
}

// IncludeLinkedFiles copies all linked files from the source directory to the target directory.
// This is used during package building to include linked files in the build output.
func (lfs *LinksFS) IncludeLinkedFiles(toDir string) ([]Link, error) {
	return includeLinkedFiles(lfs.repoRoot, lfs.workDir, toDir)
}

// ListLinkedFilesByPackage returns a mapping of packages to their linked files that reference
// files from the given directory.
func (lfs *LinksFS) ListLinkedFilesByPackage() ([]PackageLinks, error) {
	return linkedFilesByPackageFrom(lfs.repoRoot, lfs.workDir)
}

// A Link represents a linked file.
// A linked file is a file with the ".link" extension that contains a reference to another file in an other package.
// The link file contains the relative path to the included file and an optional checksum of the included file contents.
type Link struct {
	WorkDir string // WorkDir is the path to the directory containing the link file. This is where the copy of the included file will be placed.

	LinkFilePath    string // LinkFilePath is the absolute path of the linked file
	LinkChecksum    string
	LinkPackageName string // Package where the link file is located

	TargetRelPath string // TargetRelPath is the relative path to the target file, this will be the path where the content of the file is copied to

	IncludedFilePath             string // IncludedFilePath is the path to the included file, this is the content of the link file
	IncludedFileContentsChecksum string // IncludedFileContentsChecksum is the checksum of the included file contents, this is the second field in the link file
	IncludedPackageName          string // IncludedPackageName is the package where the included file is located

	UpToDate bool // UpToDate indicates whether the content of the included file matches the checksum in the link file
}

// newLinkedFile creates a new Link struct from the given absolute path to a link file.
// root is the repository root, used to validate paths and access files securely
func newLinkedFile(repoRoot *os.Root, linkFilePath string) (*Link, error) {

	workDir := filepath.Dir(linkFilePath)

	var linkPackageName string
	linkPackageRoot, err := packages.FindPackageRootFrom(workDir)
	if err != nil {
		return nil, fmt.Errorf("could not find package root for link file %s: %w", linkFilePath, err)
	}
	if linkPackageRoot != "" {
		linkPackageName = filepath.Base(linkPackageRoot)
	} else {
		// if the link file is not in a package, we consider the workdir as the package root
		linkPackageRoot = workDir
	}

	linkFileRelativePath, err := filepath.Rel(linkPackageRoot, linkFilePath)
	if err != nil {
		return nil, fmt.Errorf("could not get relative path: %w", err)
	}

	// read the content of the .link file, extract the included file path and checksum
	firstLine, err := readFirstLine(linkFilePath)
	if err != nil {
		return nil, err
	}
	fields := strings.Fields(firstLine)
	if len(fields) == 0 {
		return nil, fmt.Errorf("link file %s is empty or has no valid content", linkFilePath)
	} else if len(fields) > 2 {
		return nil, fmt.Errorf("link file %s has invalid format: expected 1 or 2 fields, got %d", linkFilePath, len(fields))
	}
	includedFileRelPath := fields[0]
	// having the checksum is optional
	var linkfileChecksum string
	if len(fields) == 2 {
		linkfileChecksum = fields[1]
	}

	// includedFilePath represents the absolute path to the included file which content we want to copy to the link file
	// resolves the path of the included file relative to the link file
	includedFilePath := filepath.Clean(filepath.Join(workDir, filepath.FromSlash(includedFileRelPath)))

	// get relative path of the included file from the root (packages root or repository root)
	includedFilePathRelFromRoot, err := filepath.Rel(repoRoot.Name(), includedFilePath)
	if err != nil {
		return nil, fmt.Errorf("could not get relative path: %w", err)
	}
	// check the file exists
	if _, err := repoRoot.Stat(includedFilePathRelFromRoot); err != nil {
		return nil, err
	}

	// check if checksum is updated
	cs, err := getLinkedFileChecksumFromRoot(repoRoot, includedFilePathRelFromRoot)
	if err != nil {
		return nil, fmt.Errorf("could not collect file %s: %w", includedFilePathRelFromRoot, err)
	}

	var includedPackageName string
	includedPackageRoot, err := packages.FindPackageRootFrom(filepath.Dir(includedFilePath))
	if err != nil {
		return nil, fmt.Errorf("could not find package root for included file %s: %w", includedFilePath, err)
	}
	if includedPackageRoot != "" {
		includedPackageName = filepath.Base(includedPackageRoot)
	}

	return &Link{
		WorkDir:                      workDir,
		LinkFilePath:                 linkFilePath,
		LinkChecksum:                 linkfileChecksum,
		LinkPackageName:              linkPackageName,
		IncludedFilePath:             includedFileRelPath,
		IncludedFileContentsChecksum: cs,
		IncludedPackageName:          includedPackageName,
		UpToDate:                     cs == linkfileChecksum,
		TargetRelPath:                strings.TrimSuffix(linkFileRelativePath, linkExtension),
	}, nil
}

// updateChecksum function updates the checksum of the linked file.
// It returns true if the checksum was updated, false if it was already up-to-date.
func (l *Link) updateChecksum() (bool, error) {
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
	if err := writeFile(l.LinkFilePath, []byte(newContent)); err != nil {
		return false, fmt.Errorf("could not update checksum for link file %s: %w", l.LinkFilePath, err)
	}
	l.LinkChecksum = l.IncludedFileContentsChecksum
	l.UpToDate = true
	return true, nil
}

// includeLinkedFiles function includes linked files from the source directory to the target directory.
// It returns a slice of Link structs representing the included files.
// It also updates the checksum of the linked files.
func includeLinkedFiles(root *os.Root, fromDir, toDir string) ([]Link, error) {
	links, err := listLinkedFiles(root, fromDir)
	if err != nil {
		return nil, fmt.Errorf("including linked files failed: %w", err)
	}
	for _, l := range links {
		if _, err := l.updateChecksum(); err != nil {
			return nil, fmt.Errorf("could not update checksum for file %s: %w", l.LinkFilePath, err)
		}
		// targetFilePath is the path where the content of the file is copied to
		targetFilePath := filepath.Join(toDir, l.TargetRelPath)

		// from l.IncludedFilePath, we just need the path name without .link suffix and to be relative to the toDir instead of the package root
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

// listLinkedFiles returns a slice of Link structs representing linked files
// within the given directory.
//
// - fromDir should be relative to the repository root.
func listLinkedFiles(root *os.Root, fromDir string) ([]Link, error) {
	var linkFilesPaths []string
	if err := filepath.Walk(
		filepath.Join(root.Name(), fromDir),
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() && strings.HasSuffix(info.Name(), linkExtension) {
				linkFilesPaths = append(linkFilesPaths, path)
			}
			return nil
		}); err != nil {
		return nil, err
	}

	links := make([]Link, len(linkFilesPaths))

	for i, f := range linkFilesPaths {
		l, err := newLinkedFile(root, filepath.FromSlash(f))
		if err != nil {
			return nil, fmt.Errorf("could not initialize linked file %s: %w", f, err)
		}
		links[i] = *l
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

// areLinkedFilesUpToDate function checks if all the linked files are up-to-date.
func areLinkedFilesUpToDate(root *os.Root, fromDir string) ([]Link, error) {
	links, err := listLinkedFiles(root, fromDir)
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

// updateLinkedFilesChecksums function updates the checksums of the linked files.
// It returns a slice of updated links.
// If no links were updated, it returns an empty slice.
func updateLinkedFilesChecksums(root *os.Root, fromDir string) ([]Link, error) {
	links, err := listLinkedFiles(root, fromDir)
	if err != nil {
		return nil, fmt.Errorf("updating linked files checksums failed: %w", err)
	}

	var updatedLinks []Link
	for _, l := range links {
		updated, err := l.updateChecksum()
		if err != nil {
			return nil, fmt.Errorf("updating linked files checksums failed: %w", err)
		}
		if updated {
			updatedLinks = append(updatedLinks, l)
		}
	}

	return updatedLinks, nil
}

// linkedFilesByPackageFrom function returns a slice of PackageLinks containing linked files grouped by package.
// Each PackageLinks contains the package name and a slice of linked file paths.
//
// - fromDir should be relative to the repository root.
func linkedFilesByPackageFrom(root *os.Root, fromDir string) ([]PackageLinks, error) {
	// we list linked files from all the root directory
	// to check which ones are linked to the 'fromDir' package
	links, err := listLinkedFiles(root, ".")
	if err != nil {
		return nil, fmt.Errorf("listing linked files failed: %w", err)
	}

	var packageName string
	packageRoot, err := packages.FindPackageRootFrom(filepath.Join(root.Name(), fromDir))
	if err != nil {
		return nil, fmt.Errorf("finding package root failed: %w", err)
	}
	if packageRoot != "" {
		packageName = filepath.Base(packageRoot)
	}
	byPackageMap := map[string][]string{}
	for _, l := range links {
		if l.LinkPackageName == l.IncludedPackageName ||
			packageName != l.IncludedPackageName {
			continue
		}
		byPackageMap[l.LinkPackageName] = append(byPackageMap[l.LinkPackageName], l.LinkFilePath)
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
