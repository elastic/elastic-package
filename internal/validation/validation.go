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

// ValidateSourceFromPath validates a package source tree — checked out from version
// control, not yet built. Source-only artifacts (_dev/, .link files, external: ecs
// references) are permitted. Validation errors are filtered against the package's
// validation.yml config; the first return value is the remaining errors after filtering,
// the second is the errors that were filtered out.
func ValidateSourceFromPath(packageRoot string) (error, error) {
	v, err := validator.NewFromPath(validator.ModeSource, packageRoot)
	if err != nil {
		return err, nil
	}
	allErrors := v.Validate()
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

// ValidateBuiltFromPath validates a built (unzipped) package directory. Source-only
// artifacts (_dev/, .link files, external: ecs references) are rejected. Validation
// errors are filtered against the package's validation.yml config; the first return
// value is the remaining errors after filtering, the second is the errors that were
// filtered out.
func ValidateBuiltFromPath(packageRoot string) (error, error) {
	v, err := validator.NewFromPath(validator.ModeBuild, packageRoot)
	if err != nil {
		return err, nil
	}
	allErrors := v.Validate()
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

// ValidateBuiltFromZip validates a built package zip archive. Zip files are always
// treated as built artifacts; source-only artifacts are rejected. Validation errors
// are filtered against the package's validation.yml config; the first return value
// is the remaining errors after filtering, the second is the errors that were
// filtered out.
func ValidateBuiltFromZip(zipPackagePath string) (error, error) {
	v, err := validator.NewFromZip(zipPackagePath)
	if err != nil {
		return fmt.Errorf("failed to open zip for validation (%s): %w", zipPackagePath, err), nil
	}
	// v.Validate() closes the zip reader it owns.
	allErrors := v.Validate()
	if allErrors == nil {
		return nil, nil
	}
	// Open a separate, independent zip reader for filterErrors.
	fsys, err := zip.OpenReader(zipPackagePath)
	if err != nil {
		return fmt.Errorf("failed to open zip file (%s): %w", zipPackagePath, err), nil
	}
	defer fsys.Close()
	// fsFromPackageZip navigates into the single package subdirectory so that
	// filterErrors can locate validation.yml at the package root, not the zip root.
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

// TODO: follow-up issue — move this logic into specerrors as an exported function so consumers don't reimplement it.
func filterErrors(allErrors error, fsys fs.FS) (specerrors.FilterResult, error) {
	var errs specerrors.ValidationErrors
	if !errors.As(allErrors, &errs) {
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
