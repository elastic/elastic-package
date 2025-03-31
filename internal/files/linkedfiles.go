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

type Link struct {
	LinkFilePath string
	LinkChecksum string

	TargetFilePath string

	IncludedFilePath             string
	IncludedFileContentsChecksum string

	UpToDate bool
}

func newLinkedFile(root *os.Root, linkFilePath string) (Link, error) {
	var l Link
	firstLine, err := readFirstLine(linkFilePath)
	if err != nil {
		return Link{}, err
	}
	l.LinkFilePath = linkFilePath
	l.TargetFilePath = strings.TrimSuffix(linkFilePath, linkExtension)
	fields := strings.Fields(firstLine)
	l.IncludedFilePath = fields[0]
	if len(fields) == 2 {
		l.LinkChecksum = fields[1]
	}

	cs, err := getLinkedFileChecksum(root.FS().(fs.ReadFileFS), l.IncludedFilePath)
	if err != nil {
		return Link{}, fmt.Errorf("could not collect file %v: %w", l.IncludedFilePath, err)
	}
	if l.LinkChecksum == cs {
		l.UpToDate = true
	}
	l.IncludedFileContentsChecksum = cs

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
	if err := writeFile(l.LinkFilePath, []byte(newContent)); err != nil {
		return false, fmt.Errorf("could not update checksum for file %v: %w", l.LinkFilePath, err)
	}
	l.LinkChecksum = l.IncludedFileContentsChecksum
	l.UpToDate = true
	return true, nil
}

// ReplaceTargetFilePathDirectory function replaces the target file path directory.
func (l *Link) ReplaceTargetFilePathDirectory(fromDir, toDir string) {
	// if a destination dir is set we replace the source dir with the destination dir
	if toDir == "" {
		return
	}
	l.TargetFilePath = strings.Replace(
		l.TargetFilePath,
		fromDir,
		toDir,
		1,
	)
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
	rootPath, err := FindRepositoryRootDirectory()
	if err != nil {
		return nil, fmt.Errorf("locating repository root failed: %w", err)
	}
	links, err := ListLinkedFiles(rootPath)
	if err != nil {
		return nil, fmt.Errorf("including linked files failed: %w", err)
	}

	packageRoot, _, _ := packages.FindPackageRootFrom(fromDir)
	packageName := filepath.Base(packageRoot)
	byPackageMap := map[string][]string{}
	for _, l := range links {
		linkPackageRoot, _, _ := packages.FindPackageRootFrom(l.LinkFilePath)
		linkPackageName := filepath.Base(linkPackageRoot)
		includedPackageRoot, _, _ := packages.FindPackageRootFrom(filepath.Join(rootPath, l.IncludedFilePath))
		includedPackageName := filepath.Base(includedPackageRoot)
		if linkPackageName == includedPackageName ||
			packageName != includedPackageName {
			continue
		}
		byPackageMap[linkPackageName] = append(byPackageMap[linkPackageName], l.LinkFilePath)
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

// ListLinkedFiles function returns a slice of Link structs representing linked files.
func ListLinkedFiles(fromDir string) ([]Link, error) {
	root, err := FindRepositoryRoot()
	if err != nil {
		return nil, fmt.Errorf("could not get root: %w", err)
	}

	links, err := getLinksFrom(root, fromDir)
	if err != nil {
		return nil, err
	}

	return links, nil
}

func CopyFile(from, to string) error {
	from = filepath.FromSlash(from)
	source, err := os.Open(from)
	if err != nil {
		return err
	}
	defer source.Close()

	to = filepath.FromSlash(to)
	if _, err := os.Stat(filepath.Dir(to)); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(to), 0700); err != nil {
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

func writeFile(to string, b []byte) error {
	to = filepath.FromSlash(to)
	if _, err := os.Stat(filepath.Dir(to)); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(to), 0700); err != nil {
			return err
		}
	}
	return os.WriteFile(to, b, 0644)
}

func getLinksFrom(root *os.Root, fromDir string) ([]Link, error) {
	var linkFiles []string
	if err := filepath.Walk(fromDir, func(path string, info os.FileInfo, err error) error {
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
		l, err := newLinkedFile(root, f)
		if err != nil {
			return nil, fmt.Errorf("could not create linked file %v: %w", f, err)
		}
		links[i] = l
	}

	return links, nil
}

func getLinkedFileChecksum(root fs.ReadFileFS, path string) (string, error) {
	b, err := root.ReadFile(filepath.FromSlash(path))
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
