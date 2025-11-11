// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package outputs

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/elastic/elastic-package/internal/benchrunner/reporters"
	"github.com/elastic/elastic-package/internal/builder"
	"github.com/elastic/elastic-package/internal/multierror"
)

func init() {
	reporters.RegisterOutput(ReportOutputFile, reportToFile)
}

const (
	// ReportOutputFile reports benchmark results to files in a folder
	ReportOutputFile reporters.Output = "file"
)

func reportToFile(report reporters.Reportable) error {
	multiReport, ok := report.(reporters.MultiReportable)
	if !ok {
		return reportSingle(report)
	}

	var merr multierror.Error
	for _, r := range multiReport.Split() {
		reportableFile, ok := r.(reporters.ReportableFile)
		if !ok {
			continue
		}

		if err := reportSingle(reportableFile); err != nil {
			merr = append(merr, err)
		}
	}

	if len(merr) > 0 {
		return merr
	}

	return nil
}

func reportSingle(report reporters.Reportable) error {
	reportableFile, ok := report.(reporters.ReportableFile)
	if !ok {
		return errors.New("this output requires a reportable file")
	}

	dest, err := reportsDir()
	if err != nil {
		return fmt.Errorf("could not determine benchmark reports folder: %w", err)
	}

	// If filename contains folders, be sure we create them properly
	dir := filepath.Dir(reportableFile.Filename())
	if dir != reportableFile.Filename() {
		dest = filepath.Join(dest, dir)
	}

	// Create benchmark reports folder if it doesn't exist
	_, err = os.Stat(dest)
	if err != nil && errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(dest, 0755); err != nil {
			return fmt.Errorf("could not create benchmark reports folder: %w", err)
		}
	}

	filePath := filepath.Join(dest, filepath.Base(reportableFile.Filename()))

	if err := os.WriteFile(filePath, append(reportableFile.Report(), byte('\n')), 0644); err != nil {
		return fmt.Errorf("could not write benchmark report file: %w", err)
	}

	return nil
}

// reportsDir returns the location of the directory to store reports.
func reportsDir() (string, error) {
	buildDir, err := builder.BuildDirectory(".")
	if err != nil {
		return "", fmt.Errorf("locating build directory failed: %w", err)
	}
	const folder = "benchmark-results"
	return filepath.Join(buildDir, folder), nil
}
