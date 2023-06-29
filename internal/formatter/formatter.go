// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package formatter

import (
	"fmt"
	"os"
	"path/filepath"
)

type formatter func(content []byte) ([]byte, bool, error)

var formatters = map[string]formatter{
	".json": JSONFormatter,
	".yaml": YAMLFormatter,
	".yml":  YAMLFormatter,
}

// Format method formats files inside of the integration directory.
func Format(packageRoot string, failFast bool) error {
	err := filepath.Walk(packageRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() && info.Name() == "ingest_pipeline" {
			return filepath.SkipDir
		}
		if info.IsDir() {
			return nil
		}
		err = formatFile(path, failFast)
		if err != nil {
			return fmt.Errorf("formatting file failed (path: %s): %w", path, err)
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("walking through the integration files failed: %w", err)
	}
	return nil
}

func formatFile(path string, failFast bool) error {
	file := filepath.Base(path)
	ext := filepath.Ext(file)

	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading file content failed: %w", err)
	}

	format, defined := formatters[ext]
	if !defined {
		return nil // no errors returned as we have few files that will be never formatted (png, svg, log, etc.)
	}

	newContent, alreadyFormatted, err := format(content)
	if err != nil {
		return fmt.Errorf("formatting file content failed: %w", err)
	}

	if alreadyFormatted {
		return nil
	}

	if failFast {
		return fmt.Errorf("file is not formatted (path: %s)", path)
	}

	err = os.WriteFile(path, newContent, 0755)
	if err != nil {
		return fmt.Errorf("rewriting file failed (path: %s): %w", path, err)
	}
	return nil
}
