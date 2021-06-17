// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package externalfields

import (
	"io/ioutil"
	"path/filepath"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/logger"
)

// Resolve function transforms all fields files into resolved form (no dependencies).
func Resolve(packageRoot, destinationDir string) error {
	bm, ok, err := readBuildManifest(packageRoot)
	if err != nil {
		return errors.Wrap(err, "can't read build manifest")
	}
	if !ok {
		logger.Debugf("Build manifest hasn't been defined for the package")
		return nil
	}
	if !bm.hasDependencies() {
		logger.Debugf("Package doesn't have any external dependencies defined")
		return nil
	}

	logger.Debugf("Package has external dependencies defined")
	fdm, err := createFieldDependencyManager(bm.Dependencies)
	if err != nil {
		return errors.Wrap(err, "can't create field dependency manager")
	}

	fieldsFile, err := filepath.Glob(filepath.Join(destinationDir, "data_stream", "*", "fields", "*"))
	if err != nil {
		return err
	}
	for _, file := range fieldsFile {
		data, err := ioutil.ReadFile(file)
		if err != nil {
			return err
		}

		rel, _ := filepath.Rel(destinationDir, file)
		output, injected, err := fdm.resolve(data)
		if err != nil {
			return err
		} else if injected {
			logger.Debugf("%s: source file has been changed", rel)

			err = ioutil.WriteFile(file, output, 0644)
			if err != nil {
				return err
			}
		} else {
			logger.Debugf("%s: source file hasn't been changed", rel)
		}
	}
	return nil
}