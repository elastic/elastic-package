// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package files

import (
	"fmt"
	"os"
	"path/filepath"
)

// RemoveContent method wipes out the directory content.
func RemoveContent(dir string) error {
	fis, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("readdir failed (path: %s): %w", dir, err)
	}

	for _, fi := range fis {
		p := filepath.Join(dir, fi.Name())
		err = os.RemoveAll(p)
		if err != nil {
			return fmt.Errorf("removing resource failed (path: %s): %w", p, err)
		}
	}
	return nil
}
