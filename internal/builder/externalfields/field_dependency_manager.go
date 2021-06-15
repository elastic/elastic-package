// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package externalfields

import (
	"github.com/elastic/elastic-package/internal/fields"
	"github.com/elastic/elastic-package/internal/logger"

	"github.com/pkg/errors"
)

type fieldDependencyManager struct {
	schema []fields.FieldDefinition
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

func buildFieldsSchema(deps dependencies) ([]fields.FieldDefinition, error) {
	var schema []fields.FieldDefinition
	ecsSchema, err := loadECSFieldsSchema(deps.ECS)
	if err != nil {
		return nil, errors.Wrap(err, "can't load fields")
	}
	schema = append(schema, ecsSchema...)
	return schema, nil
}

func loadECSFieldsSchema(dep ecsDependency) ([]fields.FieldDefinition, error) {
	var schema []fields.FieldDefinition
	if dep.Reference == "" {
		logger.Debugf("ECS dependency isn't defined")
		return schema, nil
	}

	logger.Debugf("Pulling ECS dependency, reference: %s", dep.Reference)

	return schema, nil
}

func (fdm *fieldDependencyManager) resolveFile(content []byte) ([]byte, bool, error) {
	panic("not implemented")
}
