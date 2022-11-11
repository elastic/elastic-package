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

	"github.com/elastic/elastic-package/internal/reportgenerator"
)

func init() {
	reportgenerator.RegisterReportOutput(OutputFile, writeToFile)
}

const (
	// OutputFile reports to a file
	OutputFile reportgenerator.ReportOutput = "file"
)

func writeToFile(report []byte, format, destPath string) error {
	// Create benchmark reports folder if it doesn't exist
	_, err := os.Stat(destPath)
	if err != nil && errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(destPath, 0755); err != nil {
			return fmt.Errorf("could not create benchmark reports folder: %w", err)
		}
	}

	ext := format
	fileName := fmt.Sprintf("report_%d.%s", time.Now().UnixNano(), ext)
	filePath := filepath.Join(destPath, fileName)

	if err := os.WriteFile(filePath, report, 0644); err != nil {
		return fmt.Errorf("could not write benchmark report file: %w", err)
	}

	return nil
}
