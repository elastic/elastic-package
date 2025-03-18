// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package includes

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/go-ucfg/yaml"
	"github.com/pmezard/go-difflib/difflib"
)

// IncludesFileEntry contains a file reference to include.
type IncludesFileEntry struct {
	Package  string `config:"package"`
	From     string `config:"from"`
	To       string `config:"to"`
	UpToDate bool   `config:"-"`
	Diff     string `config:"-"`
	Error    error  `config:"-"`
}

type IncludesFile []IncludesFileEntry

// IncludeSharedFiles function collects any necessary files to include in the package.
func IncludeSharedFiles() (IncludesFile, error) {
	packageRoot, err := packages.MustFindPackageRoot()
	if err != nil {
		return nil, fmt.Errorf("package root not found: %w", err)
	}

	includesFile, err := readIncludes(packageRoot)
	if err != nil {
		return nil, fmt.Errorf("could not read includes.yml: %w", err)
	}

	for _, f := range includesFile {
		b, err := collectFile(packageRoot, f)
		if err != nil {
			return nil, fmt.Errorf("could not collect file %q: %w", filepath.Join(f.Package, f.From), err)
		}
		if err := writeFile(packageRoot, f.To, b); err != nil {
			return nil, fmt.Errorf("could not write destination file %q: %w", filepath.Join(packageRoot, f.To), err)
		}
	}
	return includesFile, nil
}

// AreFilesUpToDate function checks if all the included files are up-to-date.
func AreFilesUpToDate() (IncludesFile, error) {
	packageRoot, err := packages.MustFindPackageRoot()
	if err != nil {
		return nil, fmt.Errorf("package root not found: %w", err)
	}

	includesFile, err := readIncludes(packageRoot)
	if err != nil {
		return nil, fmt.Errorf("could not read includes.yml: %w", err)
	}

	var outdated bool
	for i := 0; i < len(includesFile); i++ {
		f := includesFile[i]
		uptodate, diff, err := isFileUpToDate(packageRoot, f)
		if !uptodate || err != nil {
			includesFile[i].UpToDate = uptodate
			includesFile[i].Diff = diff
			includesFile[i].Error = err
			outdated = true
		}
	}

	if outdated {
		return includesFile, fmt.Errorf("files do not match")
	}
	return includesFile, nil
}

func isFileUpToDate(packageRoot string, includedFile IncludesFileEntry) (bool, string, error) {
	logger.Debugf("Check if %s is up-to-date", includedFile.To)

	newFile, err := collectFile(packageRoot, includedFile)
	if err != nil {
		return false, "", err
	}

	existing, found, err := readFile(packageRoot, includedFile.To)
	if err != nil {
		return false, "", fmt.Errorf("reading file failed: %w", err)
	}
	if !found {
		return false, "", nil
	}
	if bytes.Equal(existing, newFile) {
		return true, "", nil
	}
	var buf bytes.Buffer
	err = difflib.WriteUnifiedDiff(&buf, difflib.UnifiedDiff{
		A:        difflib.SplitLines(string(existing)),
		B:        difflib.SplitLines(string(newFile)),
		FromFile: "want",
		ToFile:   "got",
		Context:  1,
	})
	return false, buf.String(), err
}

func collectFile(packageRoot string, includedFile IncludesFileEntry) ([]byte, error) {
	var filePath string
	if includedFile.Package != "" {
		filePath = filepath.Join("..", includedFile.Package, includedFile.From)
	} else {
		filePath = filepath.Join(packageRoot, includedFile.From)
	}
	b, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func writeFile(packageRoot, to string, b []byte) error {
	filePath := filepath.Join(packageRoot, to)
	return os.WriteFile(filePath, b, 0644)
}

func readIncludes(packageRoot string) (IncludesFile, error) {
	includesPath := filepath.Join(packageRoot, "_dev", "shared", "includes.yml")

	b, err := os.ReadFile(includesPath)
	if err != nil && errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}

	cfg, err := yaml.NewConfig(b)
	if err != nil {
		return nil, fmt.Errorf("could not load includes config: %w", err)
	}

	var includesFile IncludesFile
	if err := cfg.Unpack(&includesFile); err != nil {
		return nil, fmt.Errorf("could not parse includes config: %w", err)
	}
	return includesFile, nil
}

func readFile(packageRoot, filePath string) ([]byte, bool, error) {
	logger.Debugf("Read existing %s file (package: %s)", filePath, packageRoot)

	includesFilepath := filepath.Join(packageRoot, filePath)
	b, err := os.ReadFile(includesFilepath)
	if err != nil && errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("readfile failed (path: %s): %w", includesFilepath, err)
	}
	return b, true, err
}
