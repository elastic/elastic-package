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
)

// Zip function creates the .zip archive from the source path (built package content).
func Zip(ctx context.Context, sourcePath, destinationFile string) error {
	out, err := os.Create(destinationFile)
	if err != nil {
		return err
	}
	defer out.Close()

	folderName := folderNameFromFileName(destinationFile)
	z := zip.NewWriter(out)
	fs := newFSWithPrefix(os.DirFS(sourcePath), folderName)
	err = z.AddFS(fs)
	if err != nil {
		return fmt.Errorf("failed to add files to package zip: %w", err)
	}
	// No need to z.Flush() because z.Close() already does it.
	err = z.Close()
	if err != nil {
		return fmt.Errorf("failed to write data to zip file: %w", err)
	}
	return nil
}

// folderNameFromFileName returns the folder name from the destination file,
// Based on mholt/archiver: https://github.com/mholt/archiver/blob/d35d4ce7c5b2411973fb7bd96ca1741eb011011b/archiver.go#L397
func folderNameFromFileName(filename string) string {
	base := filepath.Base(filename)
	firstDot := strings.LastIndex(base, ".")
	if firstDot > -1 {
		return base[:firstDot]
	}
	return base
}
