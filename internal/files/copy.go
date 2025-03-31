// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package files

import (
	"os"
	"path/filepath"

	"github.com/magefile/mage/sh"
)

var (
	defaultFoldersToSkip   = []string{"_dev", "build", ".git"}
	defaultFileGlobsToSkip = []string{".DS_Store", ".*.swp", "*.link"}
)

// CopyAll method copies files from the source to the destination skipping empty directories.
func CopyAll(sourcePath, destinationPath string) error {
	return CopyWithSkipped(sourcePath, destinationPath, []string{}, []string{})
}

// CopyWithoutDev method copies files from the source to the destination, but skips _dev directories and empty folders.
func CopyWithoutDev(sourcePath, destinationPath string) error {
	return CopyWithSkipped(sourcePath, destinationPath, defaultFoldersToSkip, defaultFileGlobsToSkip)
}

// CopyWithSkipped method copies files from the source to the destination, but skips selected directories, empty folders and selected hidden files.
func CopyWithSkipped(sourcePath, destinationPath string, skippedDirs, skippedFileGlobs []string) error {
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

		if info.IsDir() && shouldDirectoryBeSkipped(relativePath, skippedDirs) {
			return filepath.SkipDir
		}

		if info.IsDir() {
			return nil // don't create empty directories inside packages, if the directory is empty, skip it.
		}

		for _, aGlob := range skippedFileGlobs {
			matched, err := filepath.Match(aGlob, filepath.Base(relativePath))
			if err != nil {
				return err
			}
			if matched {
				return nil
			}
		}

		err = os.MkdirAll(filepath.Join(destinationPath, filepath.Dir(relativePath)), 0755)
		if err != nil {
			return err
		}

		return sh.Copy(
			filepath.Join(destinationPath, relativePath),
			filepath.Join(sourcePath, relativePath))
	})
}

// shouldDirectoryBeSkipped function checks if absolute path or last element should be skipped.
func shouldDirectoryBeSkipped(path string, skippedDirs []string) bool {
	for _, d := range skippedDirs {
		if path == d || filepath.Base(path) == d {
			return true
		}
	}
	return false
}
