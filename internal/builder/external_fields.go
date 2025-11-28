// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package builder

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/Masterminds/semver/v3"

	"github.com/elastic/elastic-package/internal/common"
	"github.com/elastic/elastic-package/internal/fields"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/packages/buildmanifest"
)

var semver3_0_0 = semver.MustParse("3.0.0")

func resolveExternalFields(packageRoot, buildPackageRoot string) error {
	bm, ok, err := buildmanifest.ReadBuildManifest(packageRoot)
	if err != nil {
		return fmt.Errorf("can't read build manifest: %w", err)
	}
	if !ok {
		logger.Debugf("Build manifest hasn't been defined for the package")
		return nil
	}
	if !bm.HasDependencies() {
		logger.Debugf("Package doesn't have any external dependencies defined")
		return nil
	}

	logger.Debugf("Package has external dependencies defined")
	fdm, err := fields.CreateFieldDependencyManager(bm.Dependencies)
	if err != nil {
		return fmt.Errorf("can't create field dependency manager: %w", err)
	}

	fieldsFiles, err := listAllFieldsFiles(buildPackageRoot)
	if err != nil {
		return fmt.Errorf("failed to list fields files under \"%s\": %w", buildPackageRoot, err)
	}

	manifest, err := packages.ReadPackageManifestFromPackageRoot(packageRoot)
	if err != nil {
		return fmt.Errorf("failed to read package manifest from \"%s\"", packageRoot)
	}
	sv, err := semver.NewVersion(manifest.SpecVersion)
	if err != nil {
		return fmt.Errorf("failed to obtain spec version from package manifest in \"%s\"", packageRoot)
	}
	var options fields.InjectFieldsOptions
	if !sv.LessThan(semver3_0_0) {
		options.DisallowReusableECSFieldsAtTopLevel = true
	}

	for _, file := range fieldsFiles {
		data, err := os.ReadFile(file)
		if err != nil {
			return err
		}

		rel, _ := filepath.Rel(buildPackageRoot, file)
		output, injected, err := injectFields(fdm, data, options)
		if err != nil {
			return err
		} else if injected {
			logger.Debugf("%s: source file has been changed", rel)

			err = os.WriteFile(file, output, 0644)
			if err != nil {
				return err
			}
		} else {
			logger.Tracef("%s: source file hasn't been changed", rel)
		}
	}

	return nil
}

func listAllFieldsFiles(packageRoot string) ([]string, error) {
	patterns := []string{
		// Package fields
		filepath.Join(packageRoot, "fields", "*.yml"),
		// Data stream fields
		filepath.Join(packageRoot, "data_stream", "*", "fields", "*.yml"),
		// Transform fields
		filepath.Join(packageRoot, "elasticsearch", "transform", "*", "fields", "*.yml"),
	}

	var paths []string
	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return nil, err
		}
		paths = append(paths, matches...)
	}

	return paths, nil
}

func injectFields(fdm *fields.DependencyManager, content []byte, options fields.InjectFieldsOptions) ([]byte, bool, error) {
	var f []common.MapStr
	err := yaml.Unmarshal(content, &f)
	if err != nil {
		return nil, false, fmt.Errorf("can't unmarshal source file: %w", err)
	}

	f, changed, err := fdm.InjectFieldsWithOptions(f, options)
	if err != nil {
		return nil, false, fmt.Errorf("can't resolve fields: %w", err)
	}
	if !changed {
		return content, false, nil
	}

	content, err = yaml.Marshal(&f)
	if err != nil {
		return nil, false, fmt.Errorf("can't marshal source file: %w", err)
	}
	return content, true, nil
}
