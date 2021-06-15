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

	"github.com/elastic/elastic-package/internal/fields"
	"github.com/elastic/elastic-package/internal/logger"
)

const (
	gitReferencePrefix = "git@"
	ecsSchemaURL       = "https://raw.githubusercontent.com/elastic/ecs/%s/generated/beats/fields.ecs.yml"
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
	var f []struct {
		Fields []fields.FieldDefinition
	}
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

func (fdm *fieldDependencyManager) resolveFile(content []byte) ([]byte, bool, error) {

	panic("not implemented")
}
