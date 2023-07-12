// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package builder

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/elastic/elastic-package/internal/common"
	"github.com/elastic/elastic-package/internal/fields"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/packages/buildmanifest"
)

func resolveExternalFields(packageRoot, destinationDir string) error {
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

	dataStreamFieldsFiles, err := filepath.Glob(filepath.Join(destinationDir, "data_stream", "*", "fields", "*.yml"))
	if err != nil {
		return err
	}

	packageFieldsFiles, err := filepath.Glob(filepath.Join(destinationDir, "fields", "*.yml"))
	if err != nil {
		return err
	}

	var fieldsFiles = append(packageFieldsFiles, dataStreamFieldsFiles...)
	for _, file := range fieldsFiles {
		data, err := os.ReadFile(file)
		if err != nil {
			return err
		}

		rel, _ := filepath.Rel(destinationDir, file)
		output, injected, err := injectFields(fdm, data)
		if err != nil {
			return err
		} else if injected {
			logger.Debugf("%s: source file has been changed", rel)

			err = os.WriteFile(file, output, 0644)
			if err != nil {
				return err
			}
		} else {
			logger.Debugf("%s: source file hasn't been changed", rel)
		}
	}
	return nil
}

func injectFields(fdm *fields.DependencyManager, content []byte) ([]byte, bool, error) {
	var f []common.MapStr
	err := yaml.Unmarshal(content, &f)
	if err != nil {
		return nil, false, fmt.Errorf("can't unmarshal source file: %w", err)
	}

	f, changed, err := fdm.InjectFields(f)
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
