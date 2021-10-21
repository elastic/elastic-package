// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package files

import (
	"compress/flate"
	"os"

	"github.com/elastic/elastic-package/internal/logger"

	"github.com/mholt/archiver/v3"
	"github.com/pkg/errors"
)

// Zip function creates the .zip archive from the source path (built package content).
func Zip(sourcePath, destinationFile string) error {
	logger.Debugf("Compress using archiver.Zip (destination: %s)", destinationFile)

	logger.Debugf("Remove old .zip artifact first (destination: %s)", destinationFile)
	err := os.Remove(destinationFile)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return errors.Wrap(err, "can't remove old .zip artifact")
	}

	z := archiver.Zip{
		CompressionLevel:       flate.DefaultCompression,
		MkdirAll:               true,
		SelectiveCompression:   true,
		ContinueOnError:        false,
		OverwriteExisting:      false,
		ImplicitTopLevelFolder: false,
	}
	err = z.Archive([]string{sourcePath}, destinationFile)
	if err != nil {
		return errors.Wrapf(err, "can't archive source directory (source path: %s)", sourcePath)
	}
	return nil
}
