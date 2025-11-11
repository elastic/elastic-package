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

	"github.com/elastic/elastic-package/internal/builder"
	"github.com/elastic/elastic-package/internal/testrunner"
	"github.com/elastic/elastic-package/internal/testrunner/reporters/formats"
)

func init() {
	testrunner.RegisterReporterOutput(ReportOutputFile, reportToFile)
}

const (
	// ReportOutputFile reports test results to files in a folder
	ReportOutputFile testrunner.TestReportOutput = "file"
)

func reportToFile(pkg, workDir string, report string, testType testrunner.TestType, format testrunner.TestReportFormat) error {
	dest, err := testReportsDir(workDir)
	if err != nil {
		return fmt.Errorf("could not determine test reports folder: %w", err)
	}
	// Create test reports folder if it doesn't exist
	_, err = os.Stat(dest)
	if err != nil && errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(dest, 0755); err != nil {
			return fmt.Errorf("could not create test reports folder: %w", err)
		}
	}

	ext := "txt"
	if format == formats.ReportFormatXUnit {
		ext = "xml"
	}

	fileName := fmt.Sprintf("%s-%s-%d.%s", pkg, testType, time.Now().UnixNano(), ext)
	filePath := filepath.Join(dest, fileName)

	if err := os.WriteFile(filePath, []byte(report+"\n"), 0644); err != nil {
		return fmt.Errorf("could not write report file: %w", err)
	}

	return nil
}

// testReportsDir returns the location of the directory to store test reports.
func testReportsDir(workDir string) (string, error) {
	buildDir, err := builder.BuildDirectory(workDir)
	if err != nil {
		return "", fmt.Errorf("locating build directory failed: %w", err)
	}
	return filepath.Join(buildDir, "test-results"), nil
}
