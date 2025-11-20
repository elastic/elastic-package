// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package testrunner

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/elastic/elastic-package/internal/files"
)

// GenerateBasePackageCoverageReport generates a coverage report where all files under the root path are
// marked as not covered. It ignores files under _dev directories.
func GenerateBasePackageCoverageReport(packageName, packageRoot, format string) (CoverageReport, error) {
	root, err := files.FindRepositoryRoot()
	if err != nil {
		return nil, fmt.Errorf("failed to find repository root directory: %w", err)
	}

	var coverage CoverageReport
	err = filepath.WalkDir(packageRoot, func(match string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "_dev" {
				return fs.SkipDir
			}
			return nil
		}

		// Exclude changelog from coverage reports, as changelogs are frequently modified and not
		// relevant to tests.
		if d.Name() == "changelog.yml" && filepath.Dir(match) == filepath.Clean(root.Name()) {
			return nil
		}

		fileCoverage, err := generateBaseFileCoverageReport(root, packageName, match, format, false)
		if err != nil {
			return fmt.Errorf("failed to generate base coverage for \"%s\": %w", match, err)
		}
		if coverage == nil {
			coverage = fileCoverage
			return nil
		}

		err = coverage.Merge(fileCoverage)
		if err != nil {
			return fmt.Errorf("cannot merge coverages: %w", err)
		}

		return nil
	})
	// If the directory is not found, give it as valid, will return an empty coverage. This is also useful for mocked tests.
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("failed to walk package directory %s: %w", packageRoot, err)
	}
	return coverage, nil
}

// GenerateBaseFileCoverageReport generates a coverage report for a given file, where all the file is marked as covered or uncovered.
func GenerateBaseFileCoverageReport(packageName, path, format string, covered bool) (CoverageReport, error) {
	root, err := files.FindRepositoryRoot()
	if err != nil {
		return nil, fmt.Errorf("failed to find repository root directory: %w", err)
	}

	return generateBaseFileCoverageReport(root, packageName, path, format, covered)
}

// GenerateBaseFileCoverageReport generates a coverage report for all the files matching any of the given patterns. The complete
// files are marked as fully covered or uncovered depending on the given value.
func GenerateBaseFileCoverageReportGlob(packageName string, patterns []string, format string, covered bool) (CoverageReport, error) {
	root, err := files.FindRepositoryRoot()
	if err != nil {
		return nil, fmt.Errorf("failed to find repository root directory: %w", err)
	}

	var coverage CoverageReport
	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return nil, err
		}

		for _, match := range matches {
			fileCoverage, err := generateBaseFileCoverageReport(root, packageName, match, format, covered)
			if err != nil {
				return nil, fmt.Errorf("failed to generate base coverage for \"%s\": %w", match, err)
			}
			if coverage == nil {
				coverage = fileCoverage
				continue
			}

			err = coverage.Merge(fileCoverage)
			if err != nil {
				return nil, fmt.Errorf("cannot merge coverages: %w", err)
			}
		}
	}
	return coverage, nil
}

func generateBaseFileCoverageReport(root *os.Root, packageName, path, format string, covered bool) (CoverageReport, error) {
	switch format {
	case "cobertura":
		return generateBaseCoberturaFileCoverageReport(root, packageName, path, covered)
	case "generic":
		return generateBaseGenericFileCoverageReport(root, packageName, path, covered)
	default:
		return nil, fmt.Errorf("unknwon coverage format %s", format)
	}
}

func generateBaseCoberturaFileCoverageReport(root *os.Root, packageName, path string, covered bool) (*CoberturaCoverage, error) {
	coveragePath, err := filepath.Rel(root.Name(), path)
	if err != nil {
		return nil, fmt.Errorf("failed to obtain path inside repository for %s", path)
	}
	ext := filepath.Ext(path)
	class := CoberturaClass{
		Name:     packageName + "." + strings.TrimSuffix(filepath.Base(path), ext),
		Filename: coveragePath,
	}
	pkg := CoberturaPackage{
		Name: packageName,
		Classes: []*CoberturaClass{
			&class,
		},
	}
	coverage := CoberturaCoverage{
		Sources: []*CoberturaSource{
			{
				Path: path,
			},
		},
		Packages: []*CoberturaPackage{
			&pkg,
		},
		Timestamp: time.Now().UnixNano(),
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %v", err)
	}
	defer f.Close()

	hits := int64(0)
	if covered {
		hits = 1
	}
	lines, err := countReaderLines(f)
	if err != nil {
		return nil, fmt.Errorf("failed to count lines in file: %w", err)
	}
	for i := range lines {
		line := CoberturaLine{
			Number: i + 1,
			Hits:   hits,
		}
		class.Lines = append(class.Lines, &line)
	}
	coverage.LinesValid = int64(lines)
	coverage.LinesCovered = int64(lines) * hits

	return &coverage, nil
}

func generateBaseGenericFileCoverageReport(root *os.Root, _, path string, covered bool) (*GenericCoverage, error) {
	coveragePath, err := filepath.Rel(root.Name(), path)
	if err != nil {
		return nil, fmt.Errorf("failed to obtain path inside repository for %s", path)
	}
	file := GenericFile{
		Path: coveragePath,
	}
	coverage := GenericCoverage{
		Version:   1,
		Timestamp: time.Now().UnixNano(),
		TestType:  fmt.Sprintf("Coverage for %s", coveragePath),
		Files: []*GenericFile{
			&file,
		},
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %v", err)
	}
	defer f.Close()

	lines, err := countReaderLines(f)
	if err != nil {
		return nil, fmt.Errorf("failed to count lines in file: %w", err)
	}
	for i := range lines {
		line := GenericLine{
			LineNumber: int64(i) + 1,
			Covered:    covered,
		}
		file.Lines = append(file.Lines, &line)
	}

	return &coverage, nil
}

func countReaderLines(r io.Reader) (int, error) {
	count := 0
	buffered := bufio.NewReader(r)
	for {
		c, _, err := buffered.ReadRune()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return 0, fmt.Errorf("failed to read rune: %w", err)
		}
		if c != '\n' {
			continue
		}
		count += 1
	}
	return count, nil
}
