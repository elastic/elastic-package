// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package fields

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"

	"github.com/elastic/elastic-package/internal/common"
	"github.com/elastic/elastic-package/internal/configuration/locations"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/packages/buildmanifest"
)

const (
	ecsSchemaName      = "ecs"
	gitReferencePrefix = "git@"

	ecsSchemaFile = "fields.ecs.yml"
	ecsSchemaURL  = "https://raw.githubusercontent.com/elastic/ecs/%s/generated/beats/%s"
)

// DependencyManager is responsible for resolving external field dependencies.
type DependencyManager struct {
	schema map[string][]FieldDefinition
}

// CreateFieldDependencyManager function creates a new instance of the DependencyManager.
func CreateFieldDependencyManager(deps buildmanifest.Dependencies) (*DependencyManager, error) {
	schema, err := buildFieldsSchema(deps)
	if err != nil {
		return nil, errors.Wrap(err, "can't build fields schema")
	}
	return &DependencyManager{
		schema: schema,
	}, nil
}

func buildFieldsSchema(deps buildmanifest.Dependencies) (map[string][]FieldDefinition, error) {
	schema := map[string][]FieldDefinition{}
	ecsSchema, err := loadECSFieldsSchema(deps.ECS)
	if err != nil {
		return nil, errors.Wrap(err, "can't load fields")
	}
	schema[ecsSchemaName] = ecsSchema
	return schema, nil
}

func loadECSFieldsSchema(dep buildmanifest.ECSDependency) ([]FieldDefinition, error) {
	if dep.Reference == "" {
		logger.Debugf("ECS dependency isn't defined")
		return nil, nil
	}

	gitReference, err := asGitReference(dep.Reference)
	if err != nil {
		return nil, errors.Wrap(err, "can't process the value as Git reference")
	}

	loc, err := locations.NewLocationManager()
	if err != nil {
		return nil, errors.Wrap(err, "error fetching profile path")
	}

	cachedSchemaPath := filepath.Join(loc.FieldsCacheDir(), ecsSchemaName, gitReference, ecsSchemaFile)
	content, err := os.ReadFile(cachedSchemaPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, errors.Wrapf(err, "can't read cached schema (path: %s)", cachedSchemaPath)
	}
	if errors.Is(err, os.ErrNotExist) {
		logger.Debugf("Pulling ECS dependency using reference: %s", dep.Reference)

		url := fmt.Sprintf(ecsSchemaURL, gitReference, ecsSchemaFile)
		logger.Debugf("Schema URL: %s", url)
		resp, err := http.Get(url)
		if err != nil {
			return nil, errors.Wrapf(err, "can't download the online schema (URL: %s)", url)
		}
		defer resp.Body.Close()
		if resp.StatusCode == 404 {
			return nil, fmt.Errorf("unsatisfied ECS dependency, reference defined in build manifest doesn't exist (HTTP 404, URL: %s)", url)
		} else if resp.StatusCode != 200 {
			return nil, fmt.Errorf("unexpected HTTP status code: %d", resp.StatusCode)
		}

		content, err = io.ReadAll(resp.Body)
		if err != nil {
			return nil, errors.Wrapf(err, "can't read schema content (URL: %s)", url)
		}
		logger.Debugf("Downloaded %d bytes", len(content))

		cachedSchemaDir := filepath.Dir(cachedSchemaPath)
		err = os.MkdirAll(cachedSchemaDir, 0755)
		if err != nil {
			return nil, errors.Wrapf(err, "can't create cache directories for schema (path: %s)", cachedSchemaDir)
		}

		logger.Debugf("Cache downloaded schema: %s", cachedSchemaPath)
		err = os.WriteFile(cachedSchemaPath, content, 0644)
		if err != nil {
			return nil, errors.Wrapf(err, "can't write cached schema (path: %s)", cachedSchemaPath)
		}
	}

	var f []FieldDefinition
	err = yaml.Unmarshal(content, &f)
	if err != nil {
		return nil, errors.Wrap(err, "unmarshalling field body failed")
	}
	return f[0].Fields, nil
}

func asGitReference(reference string) (string, error) {
	if !strings.HasPrefix(reference, gitReferencePrefix) {
		return "", errors.New(`invalid Git reference ("git@" prefix expected)`)
	}
	return reference[len(gitReferencePrefix):], nil
}

// InjectFields function replaces external field references with target definitions.
func (dm *DependencyManager) InjectFields(defs []common.MapStr) ([]common.MapStr, bool, error) {
	return dm.injectFieldsWithRoot("", defs)
}

func (dm *DependencyManager) injectFieldsWithRoot(root string, defs []common.MapStr) ([]common.MapStr, bool, error) {
	var updated []common.MapStr
	var changed bool
	for _, def := range defs {
		fieldPath := buildFieldPath(root, def)

		external, _ := def.GetValue("external")
		if external != nil {
			imported, err := dm.ImportField(external.(string), fieldPath)
			if err != nil {
				return nil, false, errors.Wrap(err, "can't import field")
			}

			transformed := transformImportedField(imported)
			originalName, _ := def.GetValue("name")
			transformed.Put("name", originalName)
			updated = append(updated, transformed)
			changed = true
			continue
		}

		fields, _ := def.GetValue("fields")
		if fields != nil {
			fieldsMs, err := common.ToMapStrSlice(fields)
			if err != nil {
				return nil, false, errors.Wrap(err, "can't convert fields")
			}
			updatedFields, fieldsChanged, err := dm.injectFieldsWithRoot(fieldPath, fieldsMs)
			if err != nil {
				return nil, false, err
			}

			if fieldsChanged {
				changed = true
			}

			def.Put("fields", updatedFields)
		}
		updated = append(updated, def)
	}
	return updated, changed, nil
}

// ImportField method resolves dependency on a single external field using available schemas.
func (dm *DependencyManager) ImportField(schemaName, fieldPath string) (FieldDefinition, error) {
	if dm == nil {
		return FieldDefinition{}, fmt.Errorf(`importing external field "%s": external fields not allowed because dependencies file "_dev/build/build.yml" is missing`, fieldPath)
	}
	schema, ok := dm.schema[schemaName]
	if !ok {
		return FieldDefinition{}, fmt.Errorf(`schema "%s" is not defined as package depedency`, schemaName)
	}

	imported := FindElementDefinition(fieldPath, schema)
	if imported == nil {
		return FieldDefinition{}, fmt.Errorf("field definition not found in schema (name: %s)", fieldPath)
	}
	return *imported, nil
}

func buildFieldPath(root string, field common.MapStr) string {
	path := root
	if root != "" {
		path += "."
	}

	fieldName, _ := field.GetValue("name")
	path = path + fieldName.(string)
	return path
}

func transformImportedField(fd FieldDefinition) common.MapStr {
	m := common.MapStr{
		"name":        fd.Name,
		"description": fd.Description,
		"type":        fd.Type,
	}

	if len(fd.Fields) > 0 {
		var t []common.MapStr
		for _, f := range fd.Fields {
			i := transformImportedField(f)
			t = append(t, i)
		}
		m.Put("fields", t)
	}
	return m
}
