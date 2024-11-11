// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package fields

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/google/go-cmp/cmp"

	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/multierror"
)

// CreateValidatorForMappings function creates a validator for the mappings.
func CreateValidatorForMappings(opts ...ValidatorOption) (v *Validator, err error) {
	v = new(Validator)
	v.injectFieldsOptions.IncludeValidationSettings = false
	for _, opt := range opts {
		if err := opt(v); err != nil {
			return nil, err
		}
	}

	// Load just fields from ECS
	finder := packageRoot{}
	if !v.disabledDependencyManagement {
		packageRoot, found, err := finder.FindPackageRoot()
		if err != nil {
			return nil, fmt.Errorf("can't find package root: %w", err)
		}
		if !found {
			return nil, errors.New("package root not found and dependency management is enabled")
		}
		_, v.Schema, err = initDependencyManagement(packageRoot, v.specVersion, v.enabledImportAllECSSchema)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize dependency management: %w", err)
		}
	}
	return v, nil
}

type mappingDefinitions map[string]any

func (v *Validator) ValidateIndexMappings(ctx context.Context) multierror.Error {
	var errs multierror.Error
	actualDynamicTemplates, actualMappings, err := v.loadMappingsFromES(ctx)
	if err != nil {
		errs = append(errs, fmt.Errorf("failed to load mappings from ES (data stream %s): %w", v.dataStreamName, err))
		return errs
	}

	previewDynamicTemplates, previewMappings, err := v.getIndexTemplatePreview(ctx)
	if err != nil {
		errs = append(errs, fmt.Errorf("failed to load mappings from index template preview (%s): %w", v.indexTemplateName, err))
		return errs
	}

	// Code from comment posted in https://github.com/google/go-cmp/issues/224
	transformJSON := cmp.FilterValues(func(x, y []byte) bool {
		return json.Valid(x) && json.Valid(y)
	}, cmp.Transformer("ParseJSON", func(in []byte) (out interface{}) {
		if err := json.Unmarshal(in, &out); err != nil {
			panic(err) // should never occur given previous filter to ensure valid JSON
		}
		return out
	}))

	// Compare dynamic templates, this should always be the same in preview and after ingesting documents
	if diff := cmp.Diff(previewDynamicTemplates, actualDynamicTemplates, transformJSON); diff != "" {
		errs = append(errs, fmt.Errorf("dynamic templates are different (data stream %s):\n%s", v.dataStreamName, diff))
	}

	// Compare actual mappings:
	// - If there are the same exact mapping definitions, everything should be good
	// - If the same mapping exists in both, but they have different "type" (anything else to check), there is some issue
	// - If there is a new mapping,
	//     - Does this come from some dynamic template? ECS componente template or dynamic templates defined in the package? This mapping is valid
	//         - conditions found in current dynamic templates: match, path_match, path_unmatch, match_mapping_type, unmatch_mapping_type
	//     - if it does not match, there should be some issue and it should be reported
	//     - If the mapping is a constant_keyword type (e.g. data_stream.dataset), how to check the value?
	//         - if the constant_keyword is defined in the preview, it should be the same
	if diff := cmp.Diff(actualMappings, previewMappings, transformJSON); diff == "" {
		logger.Debugf("No changes found in mappings")
		return errs.Unique()
	}

	// Validate mapping definitions:
	// - Are there any mapping with different type ?
	// - Are there any new mapping definitions not present in the preview?
	var rawPreview mappingDefinitions
	err = json.Unmarshal(previewMappings, &rawPreview)
	if err != nil {
		errs = append(errs, fmt.Errorf("failed to unmarshal preview mappings (index template %s): %w", v.indexTemplateName, err))
		return errs.Unique()
	}
	var rawActual mappingDefinitions
	err = json.Unmarshal(actualMappings, &rawActual)
	if err != nil {
		errs = append(errs, fmt.Errorf("failed to unmarshal actual mappings (data stream %s): %w", v.dataStreamName, err))
		return errs.Unique()
	}

	mappingErrs := compareMappings("", rawPreview, rawActual, v.Schema)
	errs = append(errs, mappingErrs...)

	if len(errs) > 0 {
		return errs.Unique()
	}

	return nil
}

func mappingParameter(field string, definition mappingDefinitions) string {
	fieldValue, ok := definition[field]
	if !ok {
		return ""
	}
	value, ok := fieldValue.(string)
	if !ok {
		return ""
	}
	return value
}

func isEmptyObject(definition mappingDefinitions) bool {
	// Example:
	//  "_tmp": {
	//    "type": "object"
	//  },
	if len(definition) != 1 {
		return false
	}
	return mappingParameter("type", definition) == "object"
}

func isConstantKeywordType(definition mappingDefinitions) bool {
	fieldType, ok := definition["type"]
	if !ok {
		return false
	}
	value, ok := fieldType.(string)
	if !ok {
		return false
	}
	return value == "constant_keyword"
}

func isObject(definition mappingDefinitions) bool {
	if len(definition) > 2 {
		// there could also be a "dynamic: true" in the mapping
		return false
	}
	field, ok := definition["properties"]
	if !ok {
		return false
	}
	if _, ok = field.(map[string]any); !ok {
		return false
	}
	return true
}

func isObjectDynamic(definition mappingDefinitions) bool {
	// Example:
	//  "labels": {
	//    "type": "object",
	//    "dynamic": "true"
	//  },
	if len(definition) != 2 {
		return false
	}
	fieldType := mappingParameter("type", definition)
	fieldDynamic := mappingParameter("dynamic", definition)

	if fieldType != "object" {
		return false
	}
	if fieldDynamic != "true" {
		return false
	}
	return true
}

func isMultiFields(definition mappingDefinitions) bool {
	_, ok := definition["type"]
	if !ok {
		return false
	}
	field, ok := definition["fields"]
	if !ok {
		return false
	}
	if _, ok = field.(map[string]any); !ok {
		return false
	}
	return true
}

func validateMappingInECS(currentPath string, definition mappingDefinitions, ecsSchema []FieldDefinition) error {
	ecsDefinition := FindElementDefinition(currentPath, ecsSchema)
	if ecsDefinition == nil {
		logger.Warnf("Path not found in ECS: %q", currentPath)
		return fmt.Errorf("missing definition for path")
	}

	logger.Debugf(">> Mario > Comparing ECS type %q with actual type %q", ecsDefinition.Type, mappingParameter("type", definition))
	if ecsDefinition.Type != mappingParameter("type", definition) {
		return fmt.Errorf("not matching mapping type with ECS")
	}
	logger.Debugf("Path FOUND in ECS: %q", currentPath)
	return nil
}

func flattenMappings(path string, definition mappingDefinitions) (mappingDefinitions, error) {
	newDefs := mappingDefinitions{}
	// Add all multi_fields
	if isMultiFields(definition) {
		multifields, err := getMappingDefinitionsField("fields", definition)
		if err != nil {
			return nil, multierror.Error{fmt.Errorf("invalid multi_field mapping %q: %w", path, err)}
		}
		for key, object := range multifields {
			currentPath := fmt.Sprintf("%s.%s", path, key)
			def, ok := object.(map[string]any)
			if !ok {
				return nil, multierror.Error{fmt.Errorf("invalid multi_field mapping type: %q", path)}
			}
			newDefs[currentPath] = mappingDefinitions(def)
		}
		return newDefs, nil
	}
	if !isObject(definition) {
		// logger.Debugf("Adding: %s", path)
		newDefs[path] = definition
		return newDefs, nil
	}

	childMappings, ok := definition["properties"].(map[string]any)
	if !ok {
		// it should not happen
		// it is already checked within isObject function
		return nil, fmt.Errorf("invalid type for properties in path: %s", path)
	}

	for key, object := range childMappings {
		currentPath := fmt.Sprintf("%s.%s", path, key)
		if path == "" {
			currentPath = key
		}

		// multi_fields are already managed above
		// there is no need to manage that case here
		value, ok := object.(map[string]any)
		if ok {
			logger.Debugf(">>> Mario > Checking flattenning path: %s", currentPath)
			other, err := flattenMappings(currentPath, mappingDefinitions(value))
			if err != nil {
				return nil, err
			}
			for i, v := range other {
				// logger.Debugf("> Adding: %s", i)
				newDefs[i] = v
			}
		}
	}

	return newDefs, nil
}

func getMappingDefinitionsField(field string, definition mappingDefinitions) (mappingDefinitions, error) {
	anyValue := definition[field]
	object, ok := anyValue.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("unexpected type found for %s: %T ", field, anyValue)
	}
	return object, nil
}

func compareMappings(path string, preview, actual mappingDefinitions, ecsSchema []FieldDefinition) multierror.Error {
	var errs multierror.Error

	if isConstantKeywordType(actual) {
		if isConstantKeywordType(preview) {
			if mappingParameter("value", preview) == "" {
				// skip validating value if preview does not have that parameter defined
				return nil
			}
			actualValue := mappingParameter("value", actual)
			previewValue := mappingParameter("value", preview)
			if previewValue != actualValue {
				return multierror.Error{fmt.Errorf("constant_keyword value in preview %q does not match the actual mapping value %q for path: %q", previewValue, actualValue, path)}
			}
			return nil
		}
		errs = append(errs, fmt.Errorf("invalid type for %q: no constant_keyword type set in preview mapping", path))
		if len(errs) == 0 {
			return nil
		}
		return errs.Unique()
	}

	if isObjectDynamic(actual) {
		logger.Debugf("Not fields ingested under path: %s.*", path)
		return errs.Unique()
	}

	if isObject(actual) {
		if isObjectDynamic(preview) {
			// TODO: Skip for now, it should be required to compare with dynamic templates
			logger.Debugf("Pending to check with dynamic templates path: %s", path)
			return errs.Unique()
		} else if !isObject(preview) {
			errs = append(errs, fmt.Errorf("not found properties in preview mappings for path: %s", path))
			return errs.Unique()
		}
		previewProperties, err := getMappingDefinitionsField("properties", preview)
		if err != nil {
			errs = append(errs, fmt.Errorf("found invalid properties type in preview mappings for path %q: %w", path, err))
		}
		actualProperties, err := getMappingDefinitionsField("properties", actual)
		if err != nil {
			errs = append(errs, fmt.Errorf("found invalid properties type in actual mappings for path %q: %w", path, err))
		}
		logger.Debugf(">>> Comparing field with properties (object): %q", path)
		compareErrors := compareMappings(path, mappingDefinitions(previewProperties), mappingDefinitions(actualProperties), ecsSchema)
		errs = append(errs, compareErrors...)

		if len(errs) == 0 {
			return nil
		}
		return errs.Unique()
	}

	if isMultiFields(actual) {
		if !isMultiFields(preview) {
			errs = append(errs, fmt.Errorf("not found multi_fields in preview mappings for path: %s", path))
			return errs.Unique()
		}
		previewFields, err := getMappingDefinitionsField("fields", preview)
		if err != nil {
			errs = append(errs, fmt.Errorf("found invalid multi_fields type in preview mappings for path %q: %w", path, err))
		}
		actualFields, err := getMappingDefinitionsField("fields", actual)
		if err != nil {
			errs = append(errs, fmt.Errorf("found invalid multi_fields type in actual mappings for path %q: %w", path, err))
		}
		logger.Debugf(">>> Comparing multi_fields: %q", path)
		compareErrors := compareMappings(path, mappingDefinitions(previewFields), mappingDefinitions(actualFields), ecsSchema)
		errs = append(errs, compareErrors...)
		// not returning here to keep validating the other fields of this object if any
	}

	// Compare all the other fields
	containsMultifield := isMultiFields(actual)
	for key, value := range actual {
		if containsMultifield && key == "fields" {
			// already checked
			continue
		}
		currentPath := fmt.Sprintf("%s.%s", path, key)
		if path == "" {
			currentPath = key
		}
		if skipValidationForField(currentPath) {
			logger.Debugf("Skipped checking property mapping due to path: %s", currentPath)
			continue
		}

		// This key does not exist in the preview mapping
		if _, ok := preview[key]; !ok {
			logger.Warnf("missing key %q in path %q (pending to check dynamic templates)", key, path)

			if childField, ok := value.(map[string]any); ok {
				if isEmptyObject(mappingDefinitions(childField)) {
					// TODO: Should this be raised as an error instead?
					logger.Debugf("field %q is an empty object and it does not exist in the preview", currentPath)

					continue
				}

				logger.Warnf("calculating flatten fields for %s", currentPath)
				flattenFields, err := flattenMappings(currentPath, childField)
				if err != nil {
					errs = append(errs, err)
					return errs
				}

				for fieldPath, object := range flattenFields {
					logger.Debugf("- %s", fieldPath)

					def, ok := object.(mappingDefinitions)
					if !ok {
						logger.Warnf(">> Mario > %s: %+v", fieldPath, object)
						errs = append(errs, fmt.Errorf("invalid field definition/mapping for path: %q", fieldPath))
						continue
					}
					// TODO: validate mapping with dynamic templates first than validating with ECS
					// just raise an error if both validation processes fail

					// are all fields under this key defined in ECS
					err = validateMappingInECS(fieldPath, def, ecsSchema)
					if err != nil {
						errs = append(errs, fmt.Errorf("field %q is undefined: %w", fieldPath, err))
					}
				}

				// errs = append(errs, fmt.Errorf("missing key %q (pending to check dynamic templates)", currentPath))
			}

			continue
		}

		fieldErrs := validateFieldMapping(preview, key, value, currentPath, ecsSchema)
		errs = append(errs, fieldErrs...)
	}
	if len(errs) == 0 {
		return nil
	}
	return errs
}

func validateFieldMapping(preview mappingDefinitions, key string, value any, currentPath string, ecsSchema []FieldDefinition) multierror.Error {
	var errs multierror.Error
	previewValue := preview[key]
	switch value.(type) {
	case map[string]any:
		// validate field
		previewField, ok := previewValue.(map[string]any)
		if !ok {
			errs = append(errs, fmt.Errorf("unexpected type in preview mappings for path: %q", currentPath))
		}
		actualField, ok := value.(map[string]any)
		if !ok {
			errs = append(errs, fmt.Errorf("unexpected type in actual mappings for path: %q", currentPath))
		}
		logger.Debugf(">>>> Comparing Mappings map[string]any: path %s", currentPath)
		errs = append(errs, compareMappings(currentPath, mappingDefinitions(previewField), mappingDefinitions(actualField), ecsSchema)...)
	case any:
		// validate each setting/parameter of the mapping
		// Skip: mappings should not be able to update, if a mapping exist in both preview and actual, they should be the same.

		// logger.Debugf("Checking mapping Values %s:\nPreview (%T):\n%s\nActual (%T):\n%s\n", currentPath, previewValue, previewValue, value, value)
		// if previewValue != value {
		// 	errs = append(errs, fmt.Errorf("unexpected value found in mapping for field %q: preview mappings value (%q) different from the actual mappings value (%q): %q", currentPath, previewValue, value, value))
		// }
	}
	return errs
}

func (v *Validator) loadMappingsFromES(ctx context.Context) (json.RawMessage, json.RawMessage, error) {
	mappingResp, err := v.esAPI.Indices.GetMapping(
		v.esAPI.Indices.GetMapping.WithContext(ctx),
		v.esAPI.Indices.GetMapping.WithIndex(v.dataStreamName),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get field mapping for data stream %q: %w", v.dataStreamName, err)
	}
	defer mappingResp.Body.Close()
	if mappingResp.IsError() {
		return nil, nil, fmt.Errorf("error getting mapping: %s", mappingResp)
	}
	body, err := io.ReadAll(mappingResp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("error reading mapping body: %w", err)
	}

	type mappings struct {
		DynamicTemplates json.RawMessage `json:"dynamic_templates"`
		Properties       json.RawMessage `json:"properties"`
	}

	mappingsRaw := map[string]struct {
		Mappings mappings `json:"mappings"`
	}{}

	if err := json.Unmarshal(body, &mappingsRaw); err != nil {
		return nil, nil, fmt.Errorf("error unmarshaling mappings: %w", err)
	}

	if len(mappingsRaw) != 1 {
		return nil, nil, fmt.Errorf("exactly 1 mapping was expected, got %d", len(mappingsRaw))
	}

	var mappingsDefinition mappings
	for _, v := range mappingsRaw {
		mappingsDefinition = v.Mappings
	}

	logger.Debugf(">>>> Mario >> Actual mappings (Properties):\n%s", mappingsDefinition.Properties)
	return mappingsDefinition.DynamicTemplates, mappingsDefinition.Properties, nil
}

func (v *Validator) getIndexTemplatePreview(ctx context.Context) (json.RawMessage, json.RawMessage, error) {
	logger.Debugf("Simulate Index Template (%s)", v.indexTemplateName)
	resp, err := v.esAPI.Indices.SimulateTemplate(
		v.esAPI.Indices.SimulateTemplate.WithContext(ctx),
		v.esAPI.Indices.SimulateTemplate.WithName(v.indexTemplateName),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get field mapping for data stream %q: %w", v.indexTemplateName, err)
	}
	defer resp.Body.Close()
	if resp.IsError() {
		return nil, nil, fmt.Errorf("error getting mapping: %s", resp)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("error reading mapping body: %w", err)
	}

	type mappingsIndexTemplate struct {
		DynamicTemplates json.RawMessage `json:"dynamic_templates"`
		Properties       json.RawMessage `json:"properties"`
	}

	type indexTemplateSimulated struct {
		// Settings json.RawMessage       `json:"settings"`
		Mappings mappingsIndexTemplate `json:"mappings"`
	}

	type previewTemplate struct {
		Template indexTemplateSimulated `json:"template"`
	}

	var preview previewTemplate

	logger.Debugf(">>>> Mario > Index template JSON:\n%s", string(body))

	if err := json.Unmarshal(body, &preview); err != nil {
		return nil, nil, fmt.Errorf("error unmarshaling mappings: %w", err)
	}

	logger.Debugf(">>>> Mario >> Index template preview (Properties):\n%s", preview.Template.Mappings.Properties)
	return preview.Template.Mappings.DynamicTemplates, preview.Template.Mappings.Properties, nil
}
