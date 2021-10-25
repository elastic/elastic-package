// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package files

import (
	"compress/flate"
	"os"
	"path/filepath"
	"strings"

	"github.com/elastic/elastic-package/internal/logger"

	"github.com/mholt/archiver/v3"
	"github.com/pkg/errors"
)

// Zip function creates the .zip archive from the source path (built package content).
func Zip(sourcePath, destinationFile string) error {
	logger.Debugf("Compress using archiver.Zip (destination: %s)", destinationFile)

	z := archiver.Zip{
		CompressionLevel:       flate.DefaultCompression,
		MkdirAll:               false,
		SelectiveCompression:   true,
		ContinueOnError:        false,
		OverwriteExisting:      true,
		ImplicitTopLevelFolder: false,
	}

	// Create a temporary work directory to properly name the root directory in the archive, e.g. aws-1.0.1
	tempDir := filepath.Join(os.TempDir(), folderNameFromFileName(destinationFile))
	os.MkdirAll(tempDir, 0755)
	defer os.RemoveAll(tempDir)

	logger.Debugf("Create temporary directory for archiving: %s", tempDir)
	err := CopyAll(sourcePath, tempDir)
	if err != nil {
		return errors.Wrapf(err, "can't create a temporary work directory (path: %s)", tempDir)
	}

	err = z.Archive([]string{tempDir}, destinationFile)
	if err != nil {
		return errors.Wrapf(err, "can't archive source directory (source path: %s)", sourcePath)
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
