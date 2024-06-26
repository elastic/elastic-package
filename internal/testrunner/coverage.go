// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package testrunner

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// GenerateBasePackageCoverageReport generates a coverage report where all files under the root path are
// marked as not covered. It ignores files under _dev directories.
func GenerateBasePackageCoverageReport(pkgName, rootPath, format string) (CoverageReport, error) {
	var coverage CoverageReport
	err := filepath.WalkDir(rootPath, func(match string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "_dev" {
				return fs.SkipDir
			}
			return nil
		}

		fileCoverage, err := GenerateBaseFileCoverageReport(pkgName, match, format, false)
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
		return nil, fmt.Errorf("failed to walk package directory %s: %w", rootPath, err)
	}
	return coverage, nil
}

// GenerateBaseFileCoverageReport generates a coverage report for a given file, where all the file is marked as covered or uncovered.
func GenerateBaseFileCoverageReport(pkgName, path, format string, covered bool) (CoverageReport, error) {
	switch format {
	case "cobertura":
		return generateBaseCoberturaFileCoverageReport(pkgName, path, covered)
	case "generic":
		return generateBaseGenericFileCoverageReport(pkgName, path, covered)
	default:
		return nil, fmt.Errorf("unknwon coverage format %s", format)
	}
}

// GenerateBaseFileCoverageReport generates a coverage report for all the files matching any of the given patterns. The complete
// files are marked as fully covered or uncovered depending on the given value.
func GenerateBaseFileCoverageReportGlob(pkgName string, patterns []string, format string, covered bool) (CoverageReport, error) {
	var coverage CoverageReport
	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return nil, err
		}

		for _, match := range matches {
			fileCoverage, err := GenerateBaseFileCoverageReport(pkgName, match, format, covered)
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

func generateBaseCoberturaFileCoverageReport(pkgName, path string, covered bool) (*CoberturaCoverage, error) {
	ext := filepath.Ext(path)
	class := CoberturaClass{
		Name:     pkgName + "." + strings.TrimSuffix(filepath.Base(path), ext),
		Filename: path,
	}
	pkg := CoberturaPackage{
		Name: pkgName,
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

	lines := int64(0)
	hits := int64(0)
	if covered {
		hits = 1
	}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines += 1
		line := CoberturaLine{
			Number: int(lines),
			Hits:   hits,
		}
		class.Lines = append(class.Lines, &line)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read file: %v", err)
	}

	coverage.LinesValid = lines
	coverage.LinesCovered = lines * hits

	return &coverage, nil
}

func generateBaseGenericFileCoverageReport(_, path string, covered bool) (*GenericCoverage, error) {
	file := GenericFile{
		Path: path,
	}
	coverage := GenericCoverage{
		Version:   1,
		Timestamp: time.Now().UnixNano(),
		TestType:  fmt.Sprintf("Coverage for %s", path),
		Files: []*GenericFile{
			&file,
		},
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %v", err)
	}
	defer f.Close()

	lineNumber := int64(0)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lineNumber += 1
		line := GenericLine{
			LineNumber: lineNumber,
			Covered:    covered,
		}
		file.Lines = append(file.Lines, &line)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read file: %v", err)
	}

	return &coverage, nil
}
