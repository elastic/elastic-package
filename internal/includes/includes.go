// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package includes

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/go-ucfg/yaml"
	"github.com/pmezard/go-difflib/difflib"
)

// IncludesFileEntry contains a file reference to include.
type IncludesFileEntry struct {
	From     string `config:"from"`
	To       string `config:"to"`
	UpToDate bool   `config:"-"`
	Diff     string `config:"-"`
	Error    error  `config:"-"`
}

type IncludesFile struct {
	Include []IncludesFileEntry `config:"include"`
}

// IncludeSharedFiles function collects any necessary files to include in the package.
func IncludeSharedFiles() ([]IncludesFileEntry, error) {
	packageRoot, err := packages.MustFindPackageRoot()
	if err != nil {
		return nil, fmt.Errorf("package root not found: %w", err)
	}

	// scope any possible operation in the packages/ folder
	dirRoot, err := os.OpenRoot(filepath.Join(packageRoot, ".."))
	if err != nil {
		return nil, fmt.Errorf("could not open root: %w", err)
	}

	includes, err := readIncludes(packageRoot)
	if err != nil {
		return nil, fmt.Errorf("could not read includes.yml: %w", err)
	}

	for _, f := range includes {
		b, err := collectFile(dirRoot.FS().(fs.ReadFileFS), f)
		if err != nil {
			return nil, fmt.Errorf("could not collect file %q: %w", filepath.FromSlash(f.From), err)
		}
		if err := writeFile(packageRoot, f.To, b); err != nil {
			return nil, fmt.Errorf("could not write destination file %q: %w", filepath.Join(packageRoot, filepath.FromSlash(f.To)), err)
		}
	}
	return includes, nil
}

// AreFilesUpToDate function checks if all the included files are up-to-date.
func AreFilesUpToDate() ([]IncludesFileEntry, error) {
	packageRoot, err := packages.MustFindPackageRoot()
	if err != nil {
		return nil, fmt.Errorf("package root not found: %w", err)
	}

	includes, err := readIncludes(packageRoot)
	if err != nil {
		return nil, fmt.Errorf("could not read includes.yml: %w", err)
	}

	var outdated bool
	for i := 0; i < len(includes); i++ {
		f := includes[i]
		uptodate, diff, err := isFileUpToDate(packageRoot, f)
		if !uptodate || err != nil {
			includes[i].UpToDate = uptodate
			includes[i].Diff = diff
			includes[i].Error = err
			outdated = true
		}
	}

	if outdated {
		return includes, fmt.Errorf("files do not match")
	}
	return includes, nil
}

func isFileUpToDate(packageRoot string, includedFile IncludesFileEntry) (bool, string, error) {
	logger.Debugf("Check if %s is up-to-date", includedFile.To)

	// scope any possible operation in the packages/ folder
	dirRoot, err := os.OpenRoot(filepath.Join(packageRoot, ".."))
	if err != nil {
		return false, "", fmt.Errorf("could not open root: %w", err)
	}

	newFile, err := collectFile(dirRoot.FS().(fs.ReadFileFS), includedFile)
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

func collectFile(root fs.ReadFileFS, includedFile IncludesFileEntry) ([]byte, error) {
	b, err := root.ReadFile(filepath.FromSlash(includedFile.From))
	if err != nil {
		return nil, err
	}
	return b, nil
}

func writeFile(packageRoot, to string, b []byte) error {
	filePath := filepath.Join(packageRoot, filepath.FromSlash(to))
	return os.WriteFile(filePath, b, 0644)
}

func readIncludes(packageRoot string) ([]IncludesFileEntry, error) {
	includesPath := filepath.Join(packageRoot, "_dev", "build", "build.yml")

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
	return includesFile.Include, nil
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
