// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package validation

import (
	"archive/zip"
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/elastic/package-spec/v3/code/go/pkg/specerrors"
	"github.com/elastic/package-spec/v3/code/go/pkg/validator"
)

func ValidateFromPath(rootPath string) error {
	return validator.ValidateFromPath(rootPath)
}

func ValidateFromZip(packagePath string) error {
	return validator.ValidateFromZip(packagePath)
}

func ValidateAndFilterFromPath(packageRoot string) (error, error) {
	allErrors := validator.ValidateFromPath(packageRoot)
	if allErrors == nil {
		return nil, nil
	}

	fsys := os.DirFS(packageRoot)
	result, err := filterErrors(allErrors, fsys)
	if err != nil {
		return err, nil
	}
	return result.Processed, result.Removed
}

func ValidateAndFilterFromZip(zipPackagePath string) (error, error) {
	allErrors := validator.ValidateFromZip(zipPackagePath)
	if allErrors == nil {
		return nil, nil
	}

	fsys, err := zip.OpenReader(zipPackagePath)
	if err != nil {
		return fmt.Errorf("failed to open zip file (%s): %w", zipPackagePath, err), nil
	}
	defer fsys.Close()

	fsZip, err := fsFromPackageZip(fsys)
	if err != nil {
		return fmt.Errorf("failed to extract filesystem from zip file (%s): %w", zipPackagePath, err), nil
	}

	result, err := filterErrors(allErrors, fsZip)
	if err != nil {
		return err, nil
	}
	return result.Processed, result.Removed
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

func filterErrors(allErrors error, fsys fs.FS) (specerrors.FilterResult, error) {
	errs, ok := allErrors.(specerrors.ValidationErrors)
	if !ok {
		return specerrors.FilterResult{Processed: allErrors, Removed: nil}, nil
	}

	config, err := specerrors.LoadConfigFilter(fsys)
	if err != nil && errors.Is(err, fs.ErrNotExist) {
		return specerrors.FilterResult{Processed: allErrors, Removed: nil}, nil
	}
	if err != nil {
		return specerrors.FilterResult{Processed: allErrors, Removed: nil},
			fmt.Errorf("failed to read config filter: %w", err)
	}
	if config == nil {
		return specerrors.FilterResult{Processed: allErrors, Removed: nil}, nil
	}

	filter := specerrors.NewFilter(config)

	result, err := filter.Run(errs)
	if err != nil {
		return specerrors.FilterResult{Processed: allErrors, Removed: nil},
			fmt.Errorf("failed to filter errors: %w", err)
	}
	return result, nil

}
