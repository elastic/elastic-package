// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package files

import (
	"bufio"
	"bytes"
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

var _ fs.FS = (*LinksFS)(nil)

// LinksFS is a filesystem that handles linked files.
// It wraps another filesystem and checks for linked files with the ".link" extension.
// If a linked file is found, it reads the link file to determine the target file
// and its checksum. If the target file is up to date, it returns the target file.
// Otherwise, it returns an error.
type LinksFS struct {
	workDir string
	inner   fs.FS
}

// NewLinksFS creates a new LinksFS.
func NewLinksFS(workDir string) *LinksFS {
	return &LinksFS{workDir: workDir, inner: os.DirFS(workDir)}
}

// Open opens a file in the filesystem.
func (lfs *LinksFS) Open(name string) (fs.File, error) {
	name, err := filepath.Rel(lfs.workDir, name)
	if err != nil {
		return nil, fmt.Errorf("could not get relative path: %w", err)
	}
	fmt.Println(name)
	if filepath.Ext(name) != linkExtension {
		return lfs.inner.Open(name)
	}
	pathName := filepath.Join(lfs.workDir, name)
	l, err := NewLinkedFile(pathName)
	if err != nil {
		return nil, err
	}
	if !l.UpToDate {
		return nil, fmt.Errorf("linked file %s is not up to date", name)
	}
	includedPath := filepath.Join(lfs.workDir, filepath.Dir(name), l.IncludedFilePath)
	return os.Open(includedPath)
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
// It contains the path to the link file, the checksum of the linked file,
// the path to the target file, and the checksum of the included file contents.
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
func NewLinkedFile(linkFilePath string) (Link, error) {
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
	l.IncludedFilePath = fields[0]
	if len(fields) == 2 {
		l.LinkChecksum = fields[1]
	}

	pathName := filepath.Join(l.WorkDir, filepath.FromSlash(l.IncludedFilePath))
	if _, err := os.Stat(pathName); err != nil {
		return Link{}, err
	}

	notInRoot, err := pathIsInRepositoryRoot(pathName)
	if err != nil {
		return Link{}, fmt.Errorf("could not check if path %v is in repository root: %w", pathName, err)
	}
	if !notInRoot {
		return Link{}, fmt.Errorf("path %v escapes the repository root", pathName)
	}

	cs, err := getLinkedFileChecksum(pathName)
	if err != nil {
		return Link{}, fmt.Errorf("could not collect file %v: %w", l.IncludedFilePath, err)
	}
	if l.LinkChecksum == cs {
		l.UpToDate = true
	}
	l.IncludedFileContentsChecksum = cs

	if includedPackageRoot, _, _ := packages.FindPackageRootFrom(filepath.Dir(pathName)); includedPackageRoot != "" {
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
		return false, fmt.Errorf("file path is empty for file %v", l.IncludedFilePath)
	}
	if l.IncludedFileContentsChecksum == "" {
		return false, fmt.Errorf("checksum is empty for file %v", l.IncludedFilePath)
	}
	newContent := fmt.Sprintf("%v %v", filepath.ToSlash(l.IncludedFilePath), l.IncludedFileContentsChecksum)
	if err := WriteFile(filepath.Join(l.WorkDir, l.LinkFilePath), []byte(newContent)); err != nil {
		return false, fmt.Errorf("could not update checksum for file %v: %w", l.LinkFilePath, err)
	}
	l.LinkChecksum = l.IncludedFileContentsChecksum
	l.UpToDate = true
	return true, nil
}

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
func IncludeLinkedFiles(fromDir, toDir string) ([]Link, error) {
	links, err := ListLinkedFiles(fromDir)
	if err != nil {
		return nil, fmt.Errorf("including linked files failed: %w", err)
	}
	for _, l := range links {
		if _, err := l.UpdateChecksum(); err != nil {
			return nil, fmt.Errorf("could not update checksum for file %v: %w", l.LinkFilePath, err)
		}
		targetFilePath := l.TargetFilePath(toDir)
		if err := CopyFile(
			filepath.Join(l.WorkDir, filepath.FromSlash(l.IncludedFilePath)),
			targetFilePath,
		); err != nil {
			return nil, fmt.Errorf("could not write file %v: %w", targetFilePath, err)
		}
	}

	return links, nil
}

// ListLinkedFiles function returns a slice of Link structs representing linked files.
func ListLinkedFiles(fromDir string) ([]Link, error) {
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
		l, err := NewLinkedFile(filepath.FromSlash(f))
		if err != nil {
			return nil, fmt.Errorf("could not initialize linked file %v: %w", f, err)
		}
		links[i] = l
	}

	return links, nil
}

// CopyFile function copies a file from to to inside the root.
func CopyFile(from, to string) error {
	from = filepath.FromSlash(from)
	source, err := os.Open(from)
	if err != nil {
		return err
	}
	defer source.Close()

	to = filepath.FromSlash(to)
	dir := filepath.Dir(to)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return err
		}
	}
	destination, err := os.Create(to)
	if err != nil {
		return err
	}
	defer destination.Close()

	_, err = io.Copy(destination, source)
	return err
}

// WriteFile function writes a byte slice to a file inside the root.
func WriteFile(to string, b []byte) error {
	to = filepath.FromSlash(to)
	if _, err := os.Stat(filepath.Dir(to)); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(to), 0700); err != nil {
			return err
		}
	}
	return os.WriteFile(to, b, 0644)
}

// AreLinkedFilesUpToDate function checks if all the linked files are up-to-date.
func AreLinkedFilesUpToDate(fromDir string) ([]Link, error) {
	links, err := ListLinkedFiles(fromDir)
	if err != nil {
		return nil, fmt.Errorf("including linked files failed: %w", err)
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
func UpdateLinkedFilesChecksums(fromDir string) ([]Link, error) {
	links, err := ListLinkedFiles(fromDir)
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

// LinkedFilesByPackageFrom function returns a slice of maps containing linked files grouped by package.
// Each map contains the package name as the key and a slice of linked file paths as the value.
func LinkedFilesByPackageFrom(fromDir string) ([]map[string][]string, error) {
	root, err := FindRepositoryRoot()
	if err != nil {
		return nil, err
	}
	links, err := ListLinkedFiles(root.Name())
	if err != nil {
		return nil, fmt.Errorf("including linked files failed: %w", err)
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

	var byPackage []map[string][]string
	for _, p := range packages {
		m := map[string][]string{p: byPackageMap[p]}
		byPackage = append(byPackage, m)
	}
	return byPackage, nil
}

func getLinkedFileChecksum(path string) (string, error) {
	b, err := os.ReadFile(filepath.FromSlash(path))
	if err != nil {
		return "", err
	}
	cs, err := checksum(b)
	if err != nil {
		return "", err
	}
	return cs, nil
}

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

func checksum(b []byte) (string, error) {
	hash := sha256.New()
	if _, err := io.Copy(hash, bytes.NewReader(b)); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func pathIsInRepositoryRoot(path string) (bool, error) {
	path = filepath.FromSlash(path)
	root, err := FindRepositoryRoot()
	if err != nil {
		return false, err
	}
	if filepath.IsAbs(path) {
		path, err = filepath.Rel(root.Name(), path)
		if err != nil {
			return false, fmt.Errorf("could not get relative path: %w", err)
		}
	}

	if _, err := root.Stat(path); err != nil {
		return false, nil
	}
	return true, nil
}
