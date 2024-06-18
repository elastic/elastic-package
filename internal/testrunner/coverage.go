// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package testrunner

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

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

func GenerateBaseFileCoverageReportGlob(pkgName string, patterns []string, format string, covered bool) (CoverageReport, error) {
	var coverage CoverageReport
	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return nil, err
		}

		for _, match := range matches {
			fileCoverage, err := GenerateBaseFileCoverageReport(pkgName, match, format, true)
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
