// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package files

import (
	"os"
	"path/filepath"

	"github.com/magefile/mage/sh"
)

// CopyAll method copies files from the source to the destination.
func CopyAll(sourcePath, destinationPath string) error {
	return copy(sourcePath, destinationPath, []string{})
}

// CopyWithoutDev method copies files from the source to the destination, but skips _dev directories.
func CopyWithoutDev(sourcePath, destinationPath string) error {
	return copy(sourcePath, destinationPath, []string{"_dev"})
}

func copy(sourcePath, destinationPath string, skippedDirs []string) error {
	return filepath.Walk(sourcePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relativePath, err := filepath.Rel(sourcePath, path)
		if err != nil {
			return err
		}

		if relativePath == "." {
			return nil
		}

		if info.IsDir() && shouldDirectoryBeSkipped(info.Name(), skippedDirs) {
			return filepath.SkipDir
		}

		if info.IsDir() {
			return os.MkdirAll(filepath.Join(destinationPath, relativePath), 0755)
		}

		return sh.Copy(
			filepath.Join(destinationPath, relativePath),
			filepath.Join(sourcePath, relativePath))
	})
}

func shouldDirectoryBeSkipped(name string, skippedDirs []string) bool {
	for _, d := range skippedDirs {
		if name == d {
			return true
		}
	}
	return false
}
