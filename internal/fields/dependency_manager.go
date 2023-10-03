// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package fields

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/elastic/elastic-package/internal/common"
	"github.com/elastic/elastic-package/internal/configuration/locations"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/packages/buildmanifest"
)

const (
	ecsSchemaName      = "ecs"
	gitReferencePrefix = "git@"
	localFilePrefix    = "file://"

	ecsSchemaFile = "ecs_nested.yml"
	ecsSchemaURL  = "https://raw.githubusercontent.com/elastic/ecs/%s/generated/ecs/%s"
)

// DependencyManager is responsible for resolving external field dependencies.
type DependencyManager struct {
	schema map[string][]FieldDefinition
}

// CreateFieldDependencyManager function creates a new instance of the DependencyManager.
func CreateFieldDependencyManager(deps buildmanifest.Dependencies) (*DependencyManager, error) {
	schema, err := buildFieldsSchema(deps)
	if err != nil {
		return nil, fmt.Errorf("can't build fields schema: %w", err)
	}
	return &DependencyManager{
		schema: schema,
	}, nil
}

func buildFieldsSchema(deps buildmanifest.Dependencies) (map[string][]FieldDefinition, error) {
	schema := map[string][]FieldDefinition{}
	ecsSchema, err := loadECSFieldsSchema(deps.ECS)
	if err != nil {
		return nil, fmt.Errorf("can't load fields: %w", err)
	}
	schema[ecsSchemaName] = ecsSchema
	return schema, nil
}

func loadECSFieldsSchema(dep buildmanifest.ECSDependency) ([]FieldDefinition, error) {
	if dep.Reference == "" {
		logger.Debugf("ECS dependency isn't defined")
		return nil, nil
	}

	content, err := readECSFieldsSchemaFile(dep)
	if err != nil {
		return nil, fmt.Errorf("error reading ECS fields schema file: %w", err)
	}

	return parseECSFieldsSchema(content)
}

func readECSFieldsSchemaFile(dep buildmanifest.ECSDependency) ([]byte, error) {
	if strings.HasPrefix(dep.Reference, localFilePrefix) {
		path := strings.TrimPrefix(dep.Reference, localFilePrefix)
		return os.ReadFile(path)
	}

	gitReference, err := asGitReference(dep.Reference)
	if err != nil {
		return nil, fmt.Errorf("can't process the value as Git reference: %w", err)
	}

	loc, err := locations.NewLocationManager()
	if err != nil {
		return nil, fmt.Errorf("error fetching profile path: %w", err)
	}
	cachedSchemaPath := filepath.Join(loc.FieldsCacheDir(), ecsSchemaName, gitReference, ecsSchemaFile)
	content, err := os.ReadFile(cachedSchemaPath)
	if errors.Is(err, os.ErrNotExist) {
		logger.Debugf("Pulling ECS dependency using reference: %s", dep.Reference)

		url := fmt.Sprintf(ecsSchemaURL, gitReference, ecsSchemaFile)
		logger.Debugf("Schema URL: %s", url)
		resp, err := http.Get(url)
		if err != nil {
			return nil, fmt.Errorf("can't download the online schema (URL: %s): %w", url, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusNotFound {
			return nil, fmt.Errorf("unsatisfied ECS dependency, reference defined in build manifest doesn't exist (HTTP StatusNotFound, URL: %s)", url)
		} else if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("unexpected HTTP status code: %d", resp.StatusCode)
		}

		content, err = io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("can't read schema content (URL: %s): %w", url, err)
		}
		logger.Debugf("Downloaded %d bytes", len(content))

		cachedSchemaDir := filepath.Dir(cachedSchemaPath)
		err = os.MkdirAll(cachedSchemaDir, 0755)
		if err != nil {
			return nil, fmt.Errorf("can't create cache directories for schema (path: %s): %w", cachedSchemaDir, err)
		}

		logger.Debugf("Cache downloaded schema: %s", cachedSchemaPath)
		err = os.WriteFile(cachedSchemaPath, content, 0644)
		if err != nil {
			return nil, fmt.Errorf("can't write cached schema (path: %s): %w", cachedSchemaPath, err)
		}
	} else if err != nil {
		return nil, fmt.Errorf("can't read cached schema (path: %s): %w", cachedSchemaPath, err)
	}

	return content, nil
}

func parseECSFieldsSchema(content []byte) ([]FieldDefinition, error) {
	var fields FieldDefinitions
	err := yaml.Unmarshal(content, &fields)
	if err != nil {
		return nil, fmt.Errorf("unmarshalling field body failed: %w", err)
	}

	return fields, nil
}

func asGitReference(reference string) (string, error) {
	if !strings.HasPrefix(reference, gitReferencePrefix) {
		return "", errors.New(`invalid Git reference ("git@" prefix expected)`)
	}
	return reference[len(gitReferencePrefix):], nil
}

// InjectFieldsOptions allow to configure fields injection.
type InjectFieldsOptions struct {
	// KeepExternal can be set to true to avoid deleting the `external` parameter
	// of a field when resolving it. This helps keeping behaviours that depended
	// in previous versions on lazy resolution of external fields.
	KeepExternal bool

	// SkipEmptyFields can be set to true to skip empty groups when injecting fields.
	SkipEmptyFields bool

	// DisallowReusableECSFieldsAtTopLevel can be set to true to disallow importing reusable
	// ECS fields at the top level, when they cannot be reused there.
	DisallowReusableECSFieldsAtTopLevel bool

	// IncludeValidationSettings can be set to enable the injection of settings of imported
	// fields that are only used for validation of documents, but are not needed on built packages.
	IncludeValidationSettings bool

	root string
}

// InjectFields function replaces external field references with target definitions.
func (dm *DependencyManager) InjectFields(defs []common.MapStr) ([]common.MapStr, bool, error) {
	return dm.injectFieldsWithOptions(defs, InjectFieldsOptions{})
}

// InjectFieldsWithOptions function replaces external field references with target definitions.
// It can be configured with options.
func (dm *DependencyManager) InjectFieldsWithOptions(defs []common.MapStr, options InjectFieldsOptions) ([]common.MapStr, bool, error) {
	return dm.injectFieldsWithOptions(defs, options)
}

func (dm *DependencyManager) injectFieldsWithOptions(defs []common.MapStr, options InjectFieldsOptions) ([]common.MapStr, bool, error) {
	var updated []common.MapStr
	var changed bool
	for _, def := range defs {
		fieldPath := buildFieldPath(options.root, def)

		external, _ := def.GetValue("external")
		if external != nil {
			imported, err := dm.importField(external.(string), fieldPath)
			if err != nil {
				return nil, false, fmt.Errorf("can't import field: %w", err)
			}
			if imported.disallowAtTopLevel && options.DisallowReusableECSFieldsAtTopLevel {
				return nil, false, fmt.Errorf("field %s cannot be reused at top level", fieldPath)
			}

			transformed := transformImportedField(imported, options)

			// Allow overrides of everything, except the imported type, for consistency.
			transformed.DeepUpdate(def)

			if !options.KeepExternal {
				transformed.Delete("external")
			}

			// Allow to override the type only from keyword to constant_keyword,
			// to support the case of setting the value already in the mappings.
			if ttype, _ := transformed["type"].(string); ttype != "constant_keyword" || imported.Type != "keyword" {
				transformed["type"] = imported.Type
			}

			def = transformed
			changed = true
		} else {
			fields, _ := def.GetValue("fields")
			if fields != nil {
				fieldsMs, err := common.ToMapStrSlice(fields)
				if err != nil {
					return nil, false, fmt.Errorf("can't convert fields: %w", err)
				}
				childrenOptions := options
				childrenOptions.root = fieldPath
				updatedFields, fieldsChanged, err := dm.injectFieldsWithOptions(fieldsMs, childrenOptions)
				if err != nil {
					return nil, false, err
				}

				if fieldsChanged {
					changed = true
				}

				def.Put("fields", updatedFields)
			}
		}

		if options.SkipEmptyFields && skipField(def) {
			changed = true
			continue
		}
		updated = append(updated, def)
	}
	return updated, changed, nil
}

// skipField decides if a field should be skipped and not injected in the built fields.
func skipField(def common.MapStr) bool {
	t, _ := def.GetValue("type")
	if t == "group" {
		// Keep empty external groups for backwards compatibility in docs generation.
		external, _ := def.GetValue("external")
		if external != nil {
			return false
		}

		fields, _ := def.GetValue("fields")
		switch fields := fields.(type) {
		case nil:
			return true
		case []interface{}:
			return len(fields) == 0
		case []common.MapStr:
			return len(fields) == 0
		}
	}

	return false
}

// importField method resolves dependency on a single external field using available schemas.
func (dm *DependencyManager) importField(schemaName, fieldPath string) (FieldDefinition, error) {
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

// ImportAllFields method resolves all fields avaialble in the default ECS schema.
func (dm *DependencyManager) ImportAllFields(schemaName string) ([]FieldDefinition, error) {
	if dm == nil {
		return []FieldDefinition{}, fmt.Errorf(`importing all external fields: external fields not allowed because dependencies file "_dev/build/build.yml" is missing`)
	}
	schema, ok := dm.schema[schemaName]
	if !ok {
		return []FieldDefinition{}, fmt.Errorf(`schema "%s" is not defined as package depedency`, schemaName)
	}

	return schema, nil
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

func transformImportedField(fd FieldDefinition, options InjectFieldsOptions) common.MapStr {
	m := common.MapStr{
		"name": fd.Name,
		"type": fd.Type,
	}

	// Multi-fields don't have descriptions.
	if fd.Description != "" {
		m["description"] = fd.Description
	}

	if fd.Pattern != "" {
		m["pattern"] = fd.Pattern
	}

	if fd.Index != nil {
		m["index"] = *fd.Index
	}

	if fd.DocValues != nil {
		m["doc_values"] = *fd.DocValues
	}

	if len(fd.MultiFields) > 0 {
		var t []common.MapStr
		for _, f := range fd.MultiFields {
			i := transformImportedField(f, options)
			t = append(t, i)
		}
		m.Put("multi_fields", t)
	}

	if options.IncludeValidationSettings {
		if len(fd.Normalize) > 0 {
			m["normalize"] = fd.Normalize
		}

		if len(fd.AllowedValues) > 0 {
			m["allowed_values"] = fd.AllowedValues
		}

		if len(fd.ExpectedValues) > 0 {
			m["expected_values"] = fd.ExpectedValues
		}
	}

	return m
}
