// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package externalfields

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"

	"github.com/elastic/elastic-package/internal/common"
	"github.com/elastic/elastic-package/internal/fields"
	"github.com/elastic/elastic-package/internal/logger"
)

const (
	ecsSchemaName      = "ecs"
	gitReferencePrefix = "git@"
	ecsSchemaURL       = "https://raw.githubusercontent.com/elastic/ecs/%s/generated/beats/fields.ecs.yml"
)

type fieldDependencyManager struct {
	schema map[string][]fields.FieldDefinition
}

func createFieldDependencyManager(deps dependencies) (*fieldDependencyManager, error) {
	schema, err := buildFieldsSchema(deps)
	if err != nil {
		return nil, errors.Wrap(err, "can't build fields schema")
	}
	return &fieldDependencyManager{
		schema: schema,
	}, nil
}

func buildFieldsSchema(deps dependencies) (map[string][]fields.FieldDefinition, error) {
	schema := map[string][]fields.FieldDefinition{}
	ecsSchema, err := loadECSFieldsSchema(deps.ECS)
	if err != nil {
		return nil, errors.Wrap(err, "can't load fields")
	}
	schema[ecsSchemaName] = ecsSchema
	return schema, nil
}

func loadECSFieldsSchema(dep ecsDependency) ([]fields.FieldDefinition, error) {
	if dep.Reference == "" {
		logger.Debugf("ECS dependency isn't defined")
		return nil, nil
	}

	logger.Debugf("Pulling ECS dependency using reference: %s", dep.Reference)
	gitReference, err := asGitReference(dep.Reference)
	if err != nil {
		return nil, errors.Wrap(err, "can't process the value as Git reference")
	}

	url := fmt.Sprintf(ecsSchemaURL, gitReference)
	logger.Debugf("Schema URL: %s", url)
	resp, err := http.Get(url)
	if err != nil {
		return nil, errors.Wrapf(err, "can't download the online schema (URL: %s)", url)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("unexpected HTTP status code: %d", resp.StatusCode)
	}

	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrapf(err, "can't read schema content (URL: %s)", url)
	}

	logger.Debugf("Read %d bytes", len(content))
	var f []fields.FieldDefinition
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

func (fdm *fieldDependencyManager) resolve(content []byte) ([]byte, bool, error) {
	var f []common.MapStr
	err := yaml.Unmarshal(content, &f)
	if err != nil {
		return nil, false, errors.Wrap(err, "can't unmarshal source file")
	}

	f, changed, err := fdm.injectFields("", f)
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

func (fdm *fieldDependencyManager) injectFields(root string, defs []common.MapStr) ([]common.MapStr, bool, error) {
	var updated []common.MapStr
	var changed bool
	for _, def := range defs {
		fieldPath := buildFieldPath(root, def)

		external, _ := def.GetValue("external")
		if external != nil {
			schema, ok := fdm.schema[external.(string)]
			if !ok {
				return nil, false, fmt.Errorf(`schema "%s" is not defined as package depedency`, external.(string))
			}

			imported := fields.FindElementDefinition(fieldPath, schema)
			if imported == nil {
				return nil, false, fmt.Errorf("field definition not found in schema (name: %s)", fieldPath)
			}

			updated = append(updated, transformImportedField(*imported))
			changed = true
			continue
		}

		fields, _ := def.GetValue("fields")
		if fields != nil {
			fieldsMs, err := common.ToMapStrSlice(fields)
			updatedFields, fieldsChanged, err := fdm.injectFields(fieldPath, fieldsMs)
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

func buildFieldPath(root string, field common.MapStr) string {
	path := root
	if root != "" {
		path += "."
	}

	fieldName, _ := field.GetValue("name")
	path = path + fieldName.(string)
	return path
}

func transformImportedField(fd fields.FieldDefinition) common.MapStr {
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
