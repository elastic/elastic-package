// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package files

import (
	"fmt"
	"os"
)

// ClearDir method removes all items from the destination directory.
// Internally it deletes and recreates the directory.
func ClearDir(destinationPath string) error {
	err := os.RemoveAll(destinationPath)
	if err != nil {
		return fmt.Errorf("removing directory failed (path: %s): %w", destinationPath, err)
	}

	err = os.MkdirAll(destinationPath, 0755)
	if err != nil {
		return fmt.Errorf("creating directory failed (path: %s): %w", destinationPath, err)
	}
	return nil
}
