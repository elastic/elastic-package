// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package outputs

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/pkg/errors"

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
		return errors.Wrap(err, "could not determine benchmark reports folder")
	}

	// Create benchmark reports folder if it doesn't exist
	_, err = os.Stat(dest)
	if err != nil && errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(dest, 0755); err != nil {
			return errors.Wrapf(err, "could not create benchmark reports folder")
		}
	}

	ext := "txt"
	if format == formats.ReportFormatXUnit {
		ext = "xml"
	}
	fileName := fmt.Sprintf("%s_%d.%s", pkg, time.Now().UnixNano(), ext)
	filePath := filepath.Join(dest, fileName)

	if err := os.WriteFile(filePath, []byte(report+"\n"), 0644); err != nil {
		return errors.Wrapf(err, "could not write benchmark report file")
	}

	return nil
}

// reportsDir returns the location of the directory to store reports.
func reportsDir() (string, error) {
	buildDir, err := builder.BuildDirectory()
	if err != nil {
		return "", errors.Wrap(err, "locating build directory failed")
	}
	const folder = "benchmark-results"
	return filepath.Join(buildDir, folder), nil
}
