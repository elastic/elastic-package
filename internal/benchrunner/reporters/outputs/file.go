// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package outputs

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/elastic/elastic-package/internal/benchrunner"
	"github.com/elastic/elastic-package/internal/benchrunner/reporters/formats"
	"github.com/elastic/elastic-package/internal/builder"
)

func init() {
	benchrunner.RegisterReporterOutput(ReportOutputFile, reportToFile)
}

const (
	// ReportOutputFile reports benchmark results to files in a folder
	ReportOutputFile benchrunner.BenchReportOutput = "file"
)

func reportToFile(pkg, report string, format benchrunner.BenchReportFormat) error {
	dest, err := reportsDir()
	if err != nil {
		return fmt.Errorf("could not determine benchmark reports folder: %w", err)
	}

	// Create benchmark reports folder if it doesn't exist
	_, err = os.Stat(dest)
	if err != nil && errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(dest, 0755); err != nil {
			return fmt.Errorf("could not create benchmark reports folder: %w", err)
		}
	}

	ext := "txt"
	if format == formats.ReportFormatJSON {
		ext = "json"
	}
	fileName := fmt.Sprintf("%s_%d.%s", pkg, time.Now().UnixNano(), ext)
	filePath := filepath.Join(dest, fileName)

	if err := os.WriteFile(filePath, []byte(report+"\n"), 0644); err != nil {
		return fmt.Errorf("could not write benchmark report file: %w", err)
	}

	return nil
}

// reportsDir returns the location of the directory to store reports.
func reportsDir() (string, error) {
	buildDir, err := builder.BuildDirectory()
	if err != nil {
		return "", fmt.Errorf("locating build directory failed: %w", err)
	}
	const folder = "benchmark-results"
	return filepath.Join(buildDir, folder), nil
}
