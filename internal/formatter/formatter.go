// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package formatter

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Masterminds/semver/v3"
	"github.com/elastic/elastic-package/internal/packages"
)

type formatter func(content []byte) ([]byte, bool, error)

func newFormatter(specVersion semver.Version, ext string) formatter {
	switch ext {
	case ".json":
		return JSONFormatterBuilder(specVersion)
	case ".yaml", ".yml":
		return YAMLFormatter
	default:
		return nil
	}
}

// Format method formats files inside of the integration directory.
func Format(packageRoot string, failFast bool) error {
	manifest, err := packages.ReadPackageManifestFromPackageRoot(packageRoot)
	if err != nil {
		return fmt.Errorf("failed to read package manifest: %w", err)
	}
	specVersion, err := semver.NewVersion(manifest.SpecVersion)
	if err != nil {
		return fmt.Errorf("failed to parse package format version %q: %w", manifest.SpecVersion, err)
	}
	err = filepath.Walk(packageRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() && info.Name() == "ingest_pipeline" {
			return filepath.SkipDir
		}
		if info.IsDir() {
			return nil
		}
		err = formatFile(path, failFast, *specVersion)
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

func formatFile(path string, failFast bool, specVersion semver.Version) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading file content failed: %w", err)
	}

	ext := filepath.Ext(filepath.Base(path))
	format := newFormatter(specVersion, ext)
	if format == nil {
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
