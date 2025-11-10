// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package files

import (
	"archive/zip"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/elastic/elastic-package/internal/logger"
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

	out, err := os.Create(destinationFile)
	if err != nil {
		return err
	}
	defer out.Close()

	z := zip.NewWriter(out)
	err = z.AddFS(os.DirFS(tempDir))
	if err != nil {
		return fmt.Errorf("failed to add built folder to package zip: %w", err)
	}
	// No need to z.Flush() because z.Close() already does it.
	err = z.Close()
	if err != nil {
		return fmt.Errorf("failed to write data to zip file: %w", err)
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
