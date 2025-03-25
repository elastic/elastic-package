// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package builder

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

	"github.com/elastic/elastic-package/internal/files"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/packages"
)

type Link struct {
	Path     string
	Checksum string

	TargetFilePath string

	IncludedFilePath             string
	IncludedFileContents         []byte
	IncludedFileContentsChecksum string

	UpToDate bool
}

// AreLinkedFilesUpToDate function checks if all the linked files are up-to-date.
func AreLinkedFilesUpToDate(fromDir string) ([]Link, error) {
	links, err := collectLinkedFiles(fromDir, "")
	if err != nil {
		return nil, fmt.Errorf("including linked files failed: %w", err)
	}

	var outdated []Link
	for _, l := range links {
		logger.Debugf("Check if %s is up-to-date", l.Path)
		if !l.UpToDate {
			outdated = append(outdated, l)
		}
	}

	return outdated, nil
}

func IncludeLinkedFiles(fromDir, toDir string) ([]Link, error) {
	links, err := collectLinkedFiles(fromDir, toDir)
	if err != nil {
		return nil, fmt.Errorf("including linked files failed: %w", err)
	}

	for _, l := range links {
		if err := writeFile(l.TargetFilePath, l.IncludedFileContents); err != nil {
			return nil, fmt.Errorf("could not write file %v: %w", l.TargetFilePath, err)
		}
		if !l.UpToDate {
			newContent := fmt.Sprintf("%v %v", l.IncludedFilePath, l.IncludedFileContentsChecksum)
			if err := writeFile(l.Path, []byte(newContent)); err != nil {
				return nil, fmt.Errorf("could not update checksum for file %v: %w", l.Path, err)
			}
		}
		logger.Debugf("%v included in package", l.TargetFilePath)
	}

	return links, nil
}

func UpdateLinkedFilesChecksums(fromDir string) ([]Link, error) {
	links, err := collectLinkedFiles(fromDir, "")
	if err != nil {
		return nil, fmt.Errorf("updating linked files failed: %w", err)
	}

	for _, l := range links {
		if !l.UpToDate {
			newContent := fmt.Sprintf("%v %v", l.IncludedFilePath, l.IncludedFileContentsChecksum)
			if err := writeFile(l.Path, []byte(newContent)); err != nil {
				return nil, fmt.Errorf("could not update checksum for file %v: %w", l.Path, err)
			}
			logger.Debugf("%v updated", l.Path)
		}
	}

	return links, nil
}

func ListPackagesWithLinkedFilesFrom(includedPath string) ([]string, error) {
	defer func() {
		if err := os.Chdir(filepath.Dir(includedPath)); err != nil {
			logger.Errorf("could not change directory: %w", err)
		}
	}()

	rootPath, err := files.FindRepositoryRootDirectory()
	if err != nil {
		return nil, fmt.Errorf("root not found: %w", err)
	}

	links, err := collectLinkedFiles(rootPath, "")
	if err != nil {
		return nil, fmt.Errorf("updating linked files failed: %w", err)
	}

	dirRoot, err := os.OpenRoot(rootPath)
	if err != nil {
		return nil, fmt.Errorf("could not open root: %w", err)
	}

	m := map[string]struct{}{}
	for _, l := range links {
		if _, err := dirRoot.Stat(l.IncludedFilePath); os.IsNotExist(err) {
			continue
		}
		if err := os.Chdir(filepath.Dir(l.Path)); err != nil {
			return nil, fmt.Errorf("could not change directory: %w", err)
		}
		p, found, err := packages.FindPackageRoot()
		if !found || err != nil {
			if err != nil {
				logger.Errorf("could not find package root directory: %w", err)
			}
			continue
		}
		m[filepath.Base(p)] = struct{}{}
	}

	packages := make([]string, 0, len(m))
	for p := range m {
		packages = append(packages, p)
	}
	slices.Sort(packages)
	return packages, nil
}

func collectLinkedFiles(fromDir, toDir string) ([]Link, error) {
	links, root, err := getLinksAndRoot(fromDir, toDir)
	if err != nil {
		return nil, err
	}

	for i := range links {
		l := links[i]
		b, cs, err := collectFile(root, l.IncludedFilePath)
		if err != nil {
			return nil, fmt.Errorf("could not collect file %v: %w", l.IncludedFilePath, err)
		}
		if l.Checksum == cs {
			links[i].UpToDate = true
		}
		links[i].IncludedFileContents = b
		links[i].IncludedFileContentsChecksum = cs
	}

	return links, nil
}

func getLinksFrom(fromDir, toDir string) ([]Link, error) {
	var linkFiles []string
	if err := filepath.Walk(fromDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && strings.HasSuffix(info.Name(), ".link") {
			linkFiles = append(linkFiles, path)
		}
		return nil
	}); err != nil {
		return nil, err
	}

	links := make([]Link, len(linkFiles))

	for i, f := range linkFiles {
		firstLine, err := readFirstLine(f)
		if err != nil {
			return nil, err
		}
		links[i].Path = f
		links[i].TargetFilePath = strings.TrimSuffix(f, ".link")
		// if a destination dir is set we replace the source dir with the destination dir
		if toDir != "" {
			links[i].TargetFilePath = strings.Replace(
				links[i].TargetFilePath,
				fromDir,
				toDir,
				1,
			)
		}
		fields := strings.Fields(firstLine)
		links[i].IncludedFilePath = fields[0]
		if len(fields) == 2 {
			links[i].Checksum = fields[1]
		}
	}

	return links, nil
}

func getLinksAndRoot(fromDir, toDir string) ([]Link, fs.ReadFileFS, error) {
	links, err := getLinksFrom(fromDir, toDir)
	if err != nil {
		return nil, nil, fmt.Errorf("could not list link files: %w", err)
	}

	if len(links) == 0 {
		return nil, nil, nil
	}

	logger.Debugf("Package has linked files defined")

	rootPath, err := files.FindRepositoryRootDirectory()
	if err != nil {
		return nil, nil, fmt.Errorf("root not found: %w", err)
	}

	// scope any possible operation to the repository folder
	dirRoot, err := os.OpenRoot(rootPath)
	if err != nil {
		return nil, nil, fmt.Errorf("could not open root: %w", err)
	}

	return links, dirRoot.FS().(fs.ReadFileFS), nil
}

func collectFile(root fs.ReadFileFS, path string) ([]byte, string, error) {
	b, err := root.ReadFile(filepath.FromSlash(path))
	if err != nil {
		return nil, "", err
	}
	cs, err := checksum(b)
	if err != nil {
		return nil, "", err
	}
	return b, cs, nil
}

func writeFile(to string, b []byte) error {
	if _, err := os.Stat(filepath.Dir(to)); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(to), 0700); err != nil {
			return err
		}
	}
	return os.WriteFile(filepath.FromSlash(to), b, 0644)
}

func readFirstLine(filePath string) (string, error) {
	file, err := os.Open(filePath)
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
