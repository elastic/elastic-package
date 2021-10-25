// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package files

import (
	"compress/flate"
	"os"
	"path/filepath"

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
		ImplicitTopLevelFolder: true,
	}

	// It's impossible to select the root directory with archiver's options, so to prevent creating a common
	// root directory ("1.0.1" for "build/integrations/aws/1.0.1"), we need to list all items in the package root.
	listed, err := listFilesInRoot(sourcePath)
	if err != nil {
		return errors.Wrapf(err, "can't list files in root (path: %s)", sourcePath)
	}

	err = z.Archive(listed, destinationFile)
	if err != nil {
		return errors.Wrapf(err, "can't archive source directory (source path: %s)", sourcePath)
	}
	return nil
}

func listFilesInRoot(sourcePath string) ([]string, error) {
	dirEntries, err := os.ReadDir(sourcePath)
	if err != nil {
		return nil, errors.Wrap(err, "can't list source path")
	}

	var paths []string
	for _, de := range dirEntries {
		paths = append(paths, filepath.Join(sourcePath, de.Name()))
	}
	return paths, nil
}
