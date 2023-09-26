// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package validation

import (
	"archive/zip"
	"fmt"
	"io/fs"
	"os"

	"github.com/elastic/package-spec/v2/code/go/pkg/specerrors"
	"github.com/elastic/package-spec/v2/code/go/pkg/specerrors/processors"
	"github.com/elastic/package-spec/v2/code/go/pkg/validator"

	"github.com/elastic/elastic-package/internal/logger"
)

const validationConfigPath = "validation.yml"

func ValidateFromPath(rootPath string) error {
	return validator.ValidateFromPath(rootPath)
}

func ValidateFromZip(packagePath string) error {
	return validator.ValidateFromPath(packagePath)
}

func ValidateAndFilterFromPath(rootPath string) (error, error) {
	allErrors := validator.ValidateFromPath(rootPath)
	if allErrors == nil {
		return nil, nil
	}

	fsys := os.DirFS(rootPath)
	errors, skipped, err := filterErrors(allErrors, fsys, validationConfigPath)
	if err != nil {
		return err, nil
	}
	return errors, skipped
}

func ValidateAndFilterFromZip(packagePath string) (error, error) {
	allErrors := validator.ValidateFromZip(packagePath)
	if allErrors == nil {
		return nil, nil
	}

	fsys, err := zip.OpenReader(packagePath)
	if err != nil {
		return fmt.Errorf("failed to open zip file (%s): %w", packagePath, err), nil
	}
	defer fsys.Close()

	fsZip, err := fsFromPackageZip(fsys)
	if err != nil {
		return fmt.Errorf("failed to extract filesystem from zip file (%s): %w", packagePath, err), nil
	}

	errors, skipped, err := filterErrors(allErrors, fsZip, validationConfigPath)
	if err != nil {
		return err, nil
	}
	return errors, skipped
}

func fsFromPackageZip(fsys fs.FS) (fs.FS, error) {
	dirs, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, fmt.Errorf("failed to read root directory in zip file fs: %w", err)
	}
	if len(dirs) != 1 {
		return nil, fmt.Errorf("a single directory is expected in zip file, %d found", len(dirs))
	}

	subDir, err := fs.Sub(fsys, dirs[0].Name())
	if err != nil {
		return nil, err
	}
	return subDir, nil
}

func filterErrors(allErrors error, fsys fs.FS, configPath string) (error, error, error) {
	errs, ok := allErrors.(specerrors.ValidationErrors)
	if !ok {
		return allErrors, nil, nil
	}

	_, err := fs.Stat(fsys, configPath)
	if err != nil {
		logger.Debugf("file not found: %s", configPath)
		return allErrors, nil, nil
	}

	config, err := processors.LoadConfigFilter(fsys, configPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read config filter: %w", err)
	}

	filter := processors.NewFilter(config)

	filteredErrors, skippedErrors, err := filter.Run(errs)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to filter errors: %w", err)
	}
	return filteredErrors, skippedErrors, nil
}
