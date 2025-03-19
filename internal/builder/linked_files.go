// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package builder

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/elastic/elastic-package/internal/logger"
)

type link struct {
	filePath         string
	includedFilePath string
}

func includeSharedFiles(packageRoot, destinationDir string) error {
	links, err := getLinks(packageRoot)
	if err != nil {
		return fmt.Errorf("could not list link files: %w", err)
	}

	if len(links) == 0 {
		return nil
	}

	logger.Debugf("Package has linked files defined")

	// scope any possible operation in the packages/ folder
	dirRoot, err := os.OpenRoot(filepath.Join(packageRoot, ".."))
	if err != nil {
		return fmt.Errorf("could not open root: %w", err)
	}

	for _, l := range links {
		b, err := collectFile(dirRoot.FS().(fs.ReadFileFS), l.includedFilePath)
		if err != nil {
			return fmt.Errorf("could not collect file %v: %w", l.includedFilePath, err)
		}
		cleanPath := strings.TrimSuffix(l.filePath, ".link")
		toFilePath := strings.Replace(
			cleanPath,
			packageRoot,
			destinationDir,
			1,
		)
		if err := writeFile(toFilePath, b); err != nil {
			return fmt.Errorf("could not write file %v: %w", toFilePath, err)
		}
		logger.Debugf("%v included in package", cleanPath)
	}

	return nil
}

func collectFile(root fs.ReadFileFS, path string) ([]byte, error) {
	b, err := root.ReadFile(filepath.FromSlash(path))
	if err != nil {
		return nil, err
	}
	return b, nil
}

func writeFile(to string, b []byte) error {
	return os.WriteFile(filepath.FromSlash(to), b, 0644)
}

func getLinks(packageRoot string) ([]link, error) {
	var linkFiles []string
	if err := filepath.Walk(packageRoot, func(path string, info os.FileInfo, err error) error {
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

	links := make([]link, len(linkFiles))

	for i, f := range linkFiles {
		firstLine, err := readFirstLine(f)
		if err != nil {
			return nil, err
		}
		links[i].filePath = f
		links[i].includedFilePath = firstLine
	}

	return links, nil
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
