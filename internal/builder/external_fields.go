// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package builder

import (
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"

	"github.com/elastic/elastic-package/internal/common"
	"github.com/elastic/elastic-package/internal/fields"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/packages/buildmanifest"
)

func resolveExternalFields(packageRoot, destinationDir string) error {
	bm, ok, err := buildmanifest.ReadBuildManifest(packageRoot)
	if err != nil {
		return errors.Wrap(err, "can't read build manifest")
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
		return errors.Wrap(err, "can't create field dependency manager")
	}

	fieldsFile, err := filepath.Glob(filepath.Join(destinationDir, "data_stream", "*", "fields", "*.yml"))
	if err != nil {
		return err
	}
	for _, file := range fieldsFile {
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
		return nil, false, errors.Wrap(err, "can't unmarshal source file")
	}

	f, changed, err := fdm.InjectFields(f)
	if err != nil {
		return nil, false, errors.Wrap(err, "can't resolve fields")
	}
	if !changed {
		return content, false, nil
	}

	content, err = yaml.Marshal(&f)
	if err != nil {
		return nil, false, errors.Wrap(err, "can't marshal source file")
	}
	return content, true, nil
}
