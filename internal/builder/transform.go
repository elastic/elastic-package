// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package builder

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/elastic/elastic-package/internal/packages"
)

// resolveTransformDefinitions processes all transform definition files in the given destination directory.
// It reads each file, applies templating to set the final ingest pipeline name, and writes the processed
// content back to the same file.
func resolveTransformDefinitions(destinationDir string) error {
	files, err := filepath.Glob(filepath.Join(destinationDir, "elasticsearch", "transform", "*", "transform.yml"))
	if err != nil {
		return fmt.Errorf("failed matching files with transform definitions: %w", err)
	}

	for _, file := range files {
		stat, err := os.Stat(file)
		if err != nil {
			return fmt.Errorf("stat failed for transform definition file %q: %w", file, err)
		}
		contents, _, err := packages.ReadTransformDefinitionFile(file, destinationDir)
		if err != nil {
			return fmt.Errorf("failed reading transform definition file %q: %w", file, err)
		}

		err = os.WriteFile(file, contents, stat.Mode())
		if err != nil {
			return err
		}
	}
	return nil
}
