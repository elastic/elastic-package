// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package validation

import (
	"archive/zip"
	"fmt"
	"io/fs"
	"os"

	"github.com/elastic/elastic-package/internal/logger"
	ve "github.com/elastic/package-spec/v2/code/go/pkg/errors"
	"github.com/elastic/package-spec/v2/code/go/pkg/errors/processors"
	"github.com/elastic/package-spec/v2/code/go/pkg/validator"
)

const configErrorsPath = "errors.yml"

func ValidateFromPath(rootPath string) error {
	return validator.ValidateFromPath(rootPath)
}

func ValidateFromZip(packagePath string) error {
	return validator.ValidateFromPath(packagePath)
}

func ValidateAndFilterFromPath(rootPath string) error {
	allErrors := validator.ValidateFromPath(rootPath)
	if allErrors == nil {
		return nil
	}

	fsys := os.DirFS(rootPath)
	errors, err := filterErrors(allErrors, fsys, configErrorsPath)
	if err != nil {
		return err
	}
	return errors
}

func ValidateAndFilterFromZip(packagePath string) error {
	allErrors := validator.ValidateFromZip(packagePath)
	if allErrors == nil {
		return nil
	}

	fsys, err := zip.OpenReader(packagePath)
	if err != nil {
		return fmt.Errorf("failed to open zip file (%s): %w", packagePath, err)
	}
	defer fsys.Close()

	fsZip, err := fsFromPackageZip(fsys)
	if err != nil {
		return fmt.Errorf("failed to extract filesystem from zip file (%s): %w", packagePath, err)
	}

	errors, err := filterErrors(allErrors, fsZip, configErrorsPath)
	if err != nil {
		return err
	}
	return errors
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

func filterErrors(allErrors error, fsys fs.FS, configPath string) (error, error) {
	errs, ok := allErrors.(ve.ValidationErrors)
	if !ok {
		return allErrors, nil
	}

	_, err := fs.Stat(fsys, configPath)
	if err != nil {
		logger.Debugf("file not found: %s", configPath)
		return allErrors, nil
	}

	config, err := processors.LoadConfigFilter(fsys, configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config filter: %w", err)
	}

	filter := processors.NewFilter(config)

	filteredErrors, _, err := filter.Run(errs)
	if err != nil {
		return nil, fmt.Errorf("failed to filter errors: %w", err)
	}
	return filteredErrors, nil
}
