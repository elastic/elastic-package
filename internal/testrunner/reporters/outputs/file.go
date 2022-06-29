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

func reportToFile(pkg, report string, format testrunner.TestReportFormat, ttype testrunner.TestReportType) error {
	dest, err := reportsDir(ttype)
	if err != nil {
		return errors.Wrap(err, "could not determine test reports folder")
	}

	// Create test reports folder if it doesn't exist
	_, err = os.Stat(dest)
	if err != nil && errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(dest, 0755); err != nil {
			return errors.Wrapf(err, "could not create %s reports folder", ttype)
		}
	}

	ext := "txt"
	if format == formats.ReportFormatXUnit {
		ext = "xml"
	}
	fileName := fmt.Sprintf("%s_%d.%s", pkg, time.Now().UnixNano(), ext)
	filePath := filepath.Join(dest, fileName)

	if err := os.WriteFile(filePath, []byte(report+"\n"), 0644); err != nil {
		return errors.Wrapf(err, "could not write %s report file", ttype)
	}

	return nil
}

// reportsDir returns the location of the directory to store reports.
func reportsDir(ttype testrunner.TestReportType) (string, error) {
	buildDir, err := builder.BuildDirectory()
	if err != nil {
		return "", errors.Wrap(err, "locating build directory failed")
	}
	var folder string
	switch ttype {
	case testrunner.ReportTypeTest:
		folder = "test-results"
	case testrunner.ReportTypeBench:
		folder = "benchmark-results"
	default:
		return "", fmt.Errorf("unsupported report type: %s", ttype)
	}
	return filepath.Join(buildDir, folder), nil
}
