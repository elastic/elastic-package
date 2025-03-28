// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package builder

import (
	"fmt"

	"github.com/elastic/elastic-package/internal/files"
	"github.com/elastic/elastic-package/internal/logger"
)

func IncludeLinkedFiles(fromDir, toDir string) ([]files.Link, error) {
	links, err := files.ListLinkedFiles(fromDir)
	if err != nil {
		return nil, fmt.Errorf("including linked files failed: %w", err)
	}

	for _, l := range links {
		l.ReplaceTargetFilePathDirectory(fromDir, toDir)

		updated, err := l.UpdateChecksum()
		if err != nil {
			return nil, fmt.Errorf("could not update checksum for file %v: %w", l.LinkFilePath, err)
		}

		if updated {
			if err := files.CopyFile(l.IncludedFilePath, l.TargetFilePath); err != nil {
				return nil, fmt.Errorf("could not write file %v: %w", l.TargetFilePath, err)
			}
			logger.Debugf("%v included in package", l.TargetFilePath)
		}
	}

	return links, nil
}
