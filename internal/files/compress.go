// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package files

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/elastic/elastic-package/internal/logger"

	"github.com/mholt/archives"
)

// Zip function creates the .zip archive from the source path (built package content).
func Zip(ctx context.Context, sourcePath, destinationFile string) error {
	logger.Debugf("Compress using archives.Zip (destination: %s)", destinationFile)

	// Create a temporary work directory to properly name the root directory in the archive, e.g. aws-1.0.1
	tempDir, err := os.MkdirTemp("", "elastic-package-")
	if err != nil {
		return fmt.Errorf("can't prepare a temporary directory: %w", err)
	}

	folderName := folderNameFromFileName(destinationFile)

	defer os.RemoveAll(tempDir)
	workDir := filepath.Join(tempDir, folderName)
	err = os.MkdirAll(workDir, 0o755)
	if err != nil {
		return fmt.Errorf("can't prepare work directory: %s: %w", workDir, err)
	}

	logger.Debugf("Create work directory for archiving: %s", workDir)
	err = CopyAll(sourcePath, workDir)
	if err != nil {
		return fmt.Errorf("can't create a work directory (path: %s): %w", workDir, err)
	}

	filenames := map[string]string{
		workDir: folderName,
	}

	files, err := archives.FilesFromDisk(ctx, nil, filenames)
	if err != nil {
		return fmt.Errorf("failed to get files from disk: %w", err)
	}

	out, err := os.Create(destinationFile)
	if err != nil {
		return err
	}
	defer out.Close()

	z := archives.Zip{
		SelectiveCompression: true,
		ContinueOnError:      false,
	}

	err = z.Archive(ctx, out, files)
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
