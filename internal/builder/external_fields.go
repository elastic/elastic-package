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

const importedFieldsFile = "imported_elastic_package.yml"

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

	err = importFields(fdm, destinationDir)

	return err
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

func importFields(fdm *fields.DependencyManager, destinationDir string) error {
	dataStreamFieldsFolders, err := filepath.Glob(filepath.Join(destinationDir, "data_stream", "*", "fields"))
	if err != nil {
		return err
	}

	packageFieldsFolders, err := filepath.Glob(filepath.Join(destinationDir, "fields"))
	if err != nil {
		return err
	}

	importedFields, err := fieldsProcessorsData()
	if err != nil {
		return err
	}
	var importedFieldsMap []common.MapStr
	err = yaml.Unmarshal(importedFields, &importedFieldsMap)
	if err != nil {
		return err
	}
	var folderFields = append(packageFieldsFolders, dataStreamFieldsFolders...)
	for _, folder := range folderFields {
		logger.Debugf("Checking folder %s", folder)
		file := filepath.Join(folder, importedFieldsFile)
		rel, _ := filepath.Rel(destinationDir, file)

		// read all defined fields
		fieldFiles, err := filepath.Glob(filepath.Join(folder, "*.yml"))
		if err != nil {
			return err
		}
		var allDefinedFields []common.MapStr
		for _, file := range fieldFiles {
			logger.Debugf("Add fields file %s", file)
			var currentFields []common.MapStr
			content, err := os.ReadFile(file)
			if err != nil {
				return err
			}
			err = yaml.Unmarshal(content, &currentFields)
			if err != nil {
				return err
			}

			allDefinedFields = append(allDefinedFields, currentFields...)
		}

		// remove from importedFields the ones already defined
		var toSaveFields []common.MapStr
		for _, elem := range importedFieldsMap {
			nameToImport, _ := elem.GetValue("name")
			logger.Debugf("Check element  %s", nameToImport.(string))
			found := false
			for _, defined := range allDefinedFields {
				nameDefined, _ := defined.GetValue("name")

				if nameToImport == nameDefined {
					found = true
					break
				}
			}
			if !found {
				toSaveFields = append(toSaveFields, elem)
				continue
			}
			logger.Debugf("Skipped field, already defined:  %s", nameToImport)
		}

		// write files
		content, err := yaml.Marshal(&toSaveFields)
		if err != nil {
			return err
		}
		output, injected, err := injectFields(fdm, content)
		if err != nil {
			return err
		}

		if injected {
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
