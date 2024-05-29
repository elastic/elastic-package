// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package files

import (
	"compress/flate"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/mholt/archiver/v3"
)

// Zip function creates the .zip archive from the source path (built package content).
func Zip(sourcePath, destinationFile string, logger *slog.Logger) error {
	logger.Debug("Compress using archiver.Zip", slog.String("destination", destinationFile))

	z := archiver.Zip{
		CompressionLevel:       flate.DefaultCompression,
		MkdirAll:               false,
		SelectiveCompression:   true,
		ContinueOnError:        false,
		OverwriteExisting:      true,
		ImplicitTopLevelFolder: false,
	}

	// Create a temporary work directory to properly name the root directory in the archive, e.g. aws-1.0.1
	tempDir, err := os.MkdirTemp("", "elastic-package-")
	if err != nil {
		return fmt.Errorf("can't prepare a temporary directory: %w", err)
	}
	defer os.RemoveAll(tempDir)
	workDir := filepath.Join(tempDir, folderNameFromFileName(destinationFile))
	err = os.MkdirAll(workDir, 0755)
	if err != nil {
		return fmt.Errorf("can't prepare work directory: %s: %w", workDir, err)
	}

	logger.Debug("Create work directory for archiving", slog.String("work.dir", workDir))
	err = CopyAll(sourcePath, workDir)
	if err != nil {
		return fmt.Errorf("can't create a work directory (path: %s): %w", workDir, err)
	}

	err = z.Archive([]string{workDir}, destinationFile)
	if err != nil {
		return fmt.Errorf("can't archive source directory (source path: %s): %w", sourcePath, err)
	}
	return nil
}

// folderNameFromFileName returns the folder name from the destination file.
// Based on mholt/archiver: https://github.com/mholt/archiver/blob/d35d4ce7c5b2411973fb7bd96ca1741eb011011b/archiver.go#L397
func folderNameFromFileName(filename string) string {
	base := filepath.Base(filename)
	firstDot := strings.LastIndex(base, ".")
	if firstDot > -1 {
		return base[:firstDot]
	}
	return base
}
