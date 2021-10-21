// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package files

import (
	"compress/flate"

	"github.com/elastic/elastic-package/internal/logger"

	"github.com/mholt/archiver/v3"
	"github.com/pkg/errors"
)

// Zip function creates the .zip archive from the source path (built package content).
func Zip(sourcePath, destinationFile string) error {
	z := archiver.Zip{
		CompressionLevel:       flate.DefaultCompression,
		MkdirAll:               true,
		SelectiveCompression:   true,
		ContinueOnError:        false,
		OverwriteExisting:      false,
		ImplicitTopLevelFolder: false,
	}

	logger.Debug("Compress using archiver.Zip (destination: %s)", destinationFile)
	err := z.Archive([]string{sourcePath}, destinationFile)
	if err != nil {
		return errors.Wrapf(err, "can't archive source directory (source path: %s)", sourcePath)
	}
	return nil
}
