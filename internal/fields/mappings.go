// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package fields

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/google/go-cmp/cmp"

	"github.com/elastic/elastic-package/internal/elasticsearch"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/multierror"
)

// MappingValidator is responsible for mappings validation.
type MappingValidator struct {
	// Schema contains definition records.
	Schema []FieldDefinition

	// SpecVersion contains the version of the spec used by the package.
	specVersion semver.Version

	disabledDependencyManagement bool

	enabledImportAllECSSchema bool

	disabledNormalization bool

	injectFieldsOptions InjectFieldsOptions

	esClient *elasticsearch.Client

	indexTemplateName string

	dataStreamName string

	exceptionFields []string
}

// MappingValidatorOption represents an optional flag that can be passed to  CreateValidatorForMappings.
type MappingValidatorOption func(*MappingValidator) error

// WithMappingValidatorSpecVersion enables validation dependant of the spec version used by the package.
func WithMappingValidatorSpecVersion(version string) MappingValidatorOption {
	return func(v *MappingValidator) error {
		sv, err := semver.NewVersion(version)
		if err != nil {
			return fmt.Errorf("invalid version %q: %v", version, err)
		}
		v.specVersion = *sv
		return nil
	}
}

// WithMappingValidatorDisabledDependencyManagement configures the validator to ignore external fields and won't follow dependencies.
func WithMappingValidatorDisabledDependencyManagement() MappingValidatorOption {
	return func(v *MappingValidator) error {
		v.disabledDependencyManagement = true
		return nil
	}
}

// WithMappingValidatorEnabledImportAllECSSchema configures the validator to check or not the fields with the complete ECS schema.
func WithMappingValidatorEnabledImportAllECSSChema(importSchema bool) MappingValidatorOption {
	return func(v *MappingValidator) error {
		v.enabledImportAllECSSchema = importSchema
		return nil
	}
}

// WithMappingValidatorDisableNormalization configures the validator to disable normalization.
func WithMappingValidatorDisableNormalization(disabledNormalization bool) MappingValidatorOption {
	return func(v *MappingValidator) error {
		v.disabledNormalization = disabledNormalization
		return nil
	}
}

// WithMappingValidatorInjectFieldsOptions configures fields injection.
func WithMappingValidatorInjectFieldsOptions(options InjectFieldsOptions) MappingValidatorOption {
	return func(v *MappingValidator) error {
		v.injectFieldsOptions = options
		return nil
	}
}

// WithMappingValidatorElasticsearchClient configures the Elasticsearch client.
func WithMappingValidatorElasticsearchClient(esClient *elasticsearch.Client) MappingValidatorOption {
	return func(v *MappingValidator) error {
		v.esClient = esClient
		return nil
	}
}

// WithMappingValidatorIndexTemplate configures the Index Template to query to Elasticsearch.
func WithMappingValidatorIndexTemplate(indexTemplate string) MappingValidatorOption {
	return func(v *MappingValidator) error {
		v.indexTemplateName = indexTemplate
		return nil
	}
}

// WithMappingValidatorDataStream configures the Data Stream to query in Elasticsearch.
func WithMappingValidatorDataStream(dataStream string) MappingValidatorOption {
	return func(v *MappingValidator) error {
		v.dataStreamName = dataStream
		return nil
	}
}

// WithMappingValidatorDataStream configures the Data Stream to query in Elasticsearch.
func WithMappingValidatorFallbackSchema(schema []FieldDefinition) MappingValidatorOption {
	return func(v *MappingValidator) error {
		v.Schema = schema
		return nil
	}
}

// WithMappingValidatorExceptionFields configures the fields to be ignored during the validation process.
func WithMappingValidatorExceptionFields(fields []string) MappingValidatorOption {
	return func(v *MappingValidator) error {
		v.exceptionFields = fields
		return nil
	}
}

// CreateValidatorForMappings function creates a validator for the mappings.
func CreateValidatorForMappings(fieldsParentDir string, esClient *elasticsearch.Client, opts ...MappingValidatorOption) (v *MappingValidator, err error) {
	p := packageRoot{}
	opts = append(opts, WithMappingValidatorElasticsearchClient(esClient))
	return createValidatorForMappingsAndPackageRoot(fieldsParentDir, p, opts...)
}

func createValidatorForMappingsAndPackageRoot(fieldsParentDir string, finder packageRootFinder, opts ...MappingValidatorOption) (v *MappingValidator, err error) {
	v = new(MappingValidator)
	for _, opt := range opts {
		if err := opt(v); err != nil {
			return nil, err
		}
	}

	if len(v.Schema) > 0 {
		return v, nil
	}

	fieldsDir := filepath.Join(fieldsParentDir, "fields")

	var fdm *DependencyManager
	if !v.disabledDependencyManagement {
		packageRoot, found, err := finder.FindPackageRoot()
		if err != nil {
			return nil, fmt.Errorf("can't find package root: %w", err)
		}
		if !found {
			return nil, errors.New("package root not found and dependency management is enabled")
		}
		fdm, v.Schema, err = initDependencyManagement(packageRoot, v.specVersion, v.enabledImportAllECSSchema)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize dependency management: %w", err)
		}
	}
	fields, err := loadFieldsFromDir(fieldsDir, fdm, v.injectFieldsOptions)
	if err != nil {
		return nil, fmt.Errorf("can't load fields from directory (path: %s): %w", fieldsDir, err)
	}

	v.Schema = append(fields, v.Schema...)
	return v, nil
}

func (v *MappingValidator) ValidateIndexMappings(ctx context.Context) multierror.Error {
	var errs multierror.Error
	logger.Debugf("Get Mappings from data stream (%s)", v.dataStreamName)
	actualDynamicTemplates, actualMappings, err := v.esClient.DataStreamMappings(ctx, v.dataStreamName)
	if err != nil {
		errs = append(errs, fmt.Errorf("failed to load mappings from ES (data stream %s): %w", v.dataStreamName, err))
		return errs
	}

	logger.Debugf("Simulate Index Template (%s)", v.indexTemplateName)
	previewDynamicTemplates, previewMappings, err := v.esClient.SimulateIndexTemplate(ctx, v.indexTemplateName)
	if err != nil {
		errs = append(errs, fmt.Errorf("failed to load mappings from index template preview (%s): %w", v.indexTemplateName, err))
		return errs
	}

	// Code from comment posted in https://github.com/google/go-cmp/issues/224
	transformJSON := cmp.FilterValues(func(x, y []byte) bool {
		return json.Valid(x) && json.Valid(y)
	}, cmp.Transformer("ParseJSON", func(in []byte) string {
		var tmp interface{}
		if err := json.Unmarshal(in, &tmp); err != nil {
			panic(err) // should never occur given previous filter to ensure valid JSON
		}
		out, err := json.MarshalIndent(tmp, "", " ")
		if err != nil {
			panic(err)
		}
		return string(out)
	}))

	// Compare dynamic templates, this should always be the same in preview and after ingesting documents
	if diff := cmp.Diff(previewDynamicTemplates, actualDynamicTemplates, transformJSON); diff != "" {
		errs = append(errs, fmt.Errorf("dynamic templates are different (data stream %s):\n%s", v.dataStreamName, diff))
	}

	// Compare actual mappings:
	// - If they are the same exact mapping definitions as in preview, everything should be good
	// - If the same mapping exists in both, but they have different "type", there is some issue
	// - If there is a new mapping,
	//     - It could come from a ECS definition, compare that mapping with the ECS field definitions
	//     - Does this come from some dynamic template? ECS components template or dynamic templates defined in the package? This mapping is valid
	//         - conditions found in current dynamic templates: match, path_match, path_unmatch, match_mapping_type, unmatch_mapping_type
	//     - if it does not match, there should be some issue and it should be reported
	//     - If the mapping is a constant_keyword type (e.g. data_stream.dataset), how to check the value?
	//         - if the constant_keyword is defined in the preview, it should be the same
	if diff := cmp.Diff(actualMappings, previewMappings, transformJSON); diff == "" {
		logger.Debug("No changes found in mappings")
		return errs.Unique()
	}

	var rawPreview map[string]any
	err = json.Unmarshal(previewMappings, &rawPreview)
	if err != nil {
		errs = append(errs, fmt.Errorf("failed to unmarshal preview mappings (index template %s): %w", v.indexTemplateName, err))
		return errs.Unique()
	}
	var rawActual map[string]any
	err = json.Unmarshal(actualMappings, &rawActual)
	if err != nil {
		errs = append(errs, fmt.Errorf("failed to unmarshal actual mappings (data stream %s): %w", v.dataStreamName, err))
		return errs.Unique()
	}

	mappingErrs := v.compareMappings("", false, rawPreview, rawActual)
	errs = append(errs, mappingErrs...)

	if len(errs) > 0 {
		return errs.Unique()
	}

	return nil
}

func currentMappingPath(path, key string) string {
	if path == "" {
		return key
	}
	return fmt.Sprintf("%s.%s", path, key)
}

func mappingParameter(field string, definition map[string]any) string {
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

func isEmptyObject(definition map[string]any) bool {
	// Example:
	//  "_tmp": {
	//    "type": "object"
	//  },
	if len(definition) != 1 {
		return false
	}
	return mappingParameter("type", definition) == "object"
}

func isObject(definition map[string]any) bool {
	// Example:
	// "http": {
	//   "properties": {
	// 	   "request": {
	// 	     "properties": {
	//         "method": {
	//           "type": "keyword",
	//           "ignore_above": 1024
	//         }
	//       }
	//     }
	//   }
	// }
	field, ok := definition["properties"]
	if !ok {
		return false
	}
	if _, ok = field.(map[string]any); !ok {
		return false
	}
	return true
}

func isObjectFullyDynamic(definition map[string]any) bool {
	// Example:
	//  "labels": {
	//    "type": "object",
	//    "dynamic": "true"
	//  },
	fieldType := mappingParameter("type", definition)
	fieldDynamic := mappingParameter("dynamic", definition)

	if fieldType != "object" {
		return false
	}
	if fieldDynamic != "true" {
		return false
	}

	field, ok := definition["properties"]
	if !ok {
		return true
	}
	props, ok := field.(map[string]any)
	if !ok {
		return false
	}
	// It should not have properties
	// https://www.elastic.co/guide/en/elasticsearch/reference/8.16/dynamic.html
	if len(props) != 0 {
		return false
	}
	return true
}

func isMultiFields(definition map[string]any) bool {
	// Example:
	//  "path": {
	//    "type": "keyword",
	//    "fields": {
	//      "text": {
	//        "type": "match_only_text"
	//      }
	//    }
	//  },
	fieldType := mappingParameter("type", definition)
	if fieldType == "" {
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

func isNumberTypeField(previewType, actualType string) bool {
	if slices.Contains([]string{"float", "long", "double"}, previewType) && slices.Contains([]string{"float", "long", "double"}, string(actualType)) {
		return true
	}

	return false
}

func (v *MappingValidator) validateMappingInECSSchema(currentPath string, definition map[string]any) error {
	found := FindElementDefinition(currentPath, v.Schema)
	if found == nil {
		return fmt.Errorf("missing definition for path")
	}

	if found.External != "ecs" {
		return fmt.Errorf("missing definition for path (not in ECS)")
	}

	actualType := mappingParameter("type", definition)
	if found.Type == actualType {
		return nil
	}

	// exceptions related to numbers
	if isNumberTypeField(found.Type, actualType) {
		logger.Debugf("Allowed number fields with different types (ECS %s - actual %s)", string(found.Type), string(actualType))
		return nil
	}
	// any other field to validate here?
	return fmt.Errorf("actual mapping type (%s) does not match with ECS definition type: %s", actualType, found.Type)
}

// flattenMappings returns all the mapping definitions found at "path" flattened including
// specific entries for multi fields too.
func flattenMappings(path string, definition map[string]any) (map[string]any, error) {
	newDefs := map[string]any{}
	if isMultiFields(definition) {
		multifields, err := getMappingDefinitionsField("fields", definition)
		if err != nil {
			return nil, multierror.Error{fmt.Errorf("invalid multi_field mapping %q: %w", path, err)}
		}

		// Include also the definition itself
		newDefs[path] = definition

		for key, object := range multifields {
			currentPath := currentMappingPath(path, key)
			def, ok := object.(map[string]any)
			if !ok {
				return nil, multierror.Error{fmt.Errorf("invalid multi_field mapping type: %q", path)}
			}
			newDefs[currentPath] = def
		}
		return newDefs, nil
	}

	if !isObject(definition) {
		newDefs[path] = definition
		return newDefs, nil
	}

	childMappings, ok := definition["properties"].(map[string]any)
	if !ok {
		// it should not happen, it is already checked above
		return nil, fmt.Errorf("invalid type for properties in path: %s", path)
	}

	for key, object := range childMappings {
		currentPath := currentMappingPath(path, key)
		// multi_fields are already managed above
		// there is no need to manage that case here
		value, ok := object.(map[string]any)
		if ok {
			other, err := flattenMappings(currentPath, value)
			if err != nil {
				return nil, err
			}
			for i, v := range other {
				newDefs[i] = v
			}
		}
	}

	return newDefs, nil
}

func getMappingDefinitionsField(field string, definition map[string]any) (map[string]any, error) {
	anyValue := definition[field]
	object, ok := anyValue.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("unexpected type found for %q: %T ", field, anyValue)
	}
	return object, nil
}

func validateConstantKeywordField(path string, preview, actual map[string]any) (bool, error) {
	isConstantKeyword := false
	if mappingParameter("type", actual) != "constant_keyword" {
		return isConstantKeyword, nil
	}
	isConstantKeyword = true
	if mappingParameter("type", preview) != "constant_keyword" {
		return isConstantKeyword, fmt.Errorf("invalid type for %q: no constant_keyword type set in preview mapping", path)
	}
	actualValue := mappingParameter("value", actual)
	previewValue := mappingParameter("value", preview)

	if previewValue == "" {
		// skip validating value if preview does not have that parameter defined
		return isConstantKeyword, nil
	}

	if previewValue != actualValue {
		// This should also be detected by the failure storage (if available)
		// or no documents being ingested
		return isConstantKeyword, fmt.Errorf("constant_keyword value in preview %q does not match the actual mapping value %q for path: %q", previewValue, actualValue, path)
	}
	return isConstantKeyword, nil
}

func (v *MappingValidator) compareMappings(path string, couldBeParametersDefinition bool, preview, actual map[string]any) multierror.Error {
	var errs multierror.Error

	isConstantKeywordType, err := validateConstantKeywordField(path, preview, actual)
	if err != nil {
		return multierror.Error{err}
	}
	if isConstantKeywordType {
		return nil
	}

	if slices.Contains(v.exceptionFields, path) {
		logger.Warnf("Found exception field, skip its validation: %q", path)
		return nil
	}

	if isObjectFullyDynamic(actual) {
		logger.Debugf("Dynamic object found but no fields ingested under path: \"%s.*\"", path)
		return nil
	}

	// Ensure to validate properties from an object (subfields) in the right location of the mappings
	// there could be "sub-fields" with name "properties" too
	if couldBeParametersDefinition && isObject(actual) {
		if isObjectFullyDynamic(preview) {
			// TODO: Skip for now, it should be required to compare with dynamic templates
			logger.Debugf("Pending to validate with the dynamic templates defined the path: %q", path)
			return nil
		} else if !isObject(preview) {
			errs = append(errs, fmt.Errorf("not found properties in preview mappings for path: %q", path))
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
		compareErrors := v.compareMappings(path, false, previewProperties, actualProperties)
		errs = append(errs, compareErrors...)

		if len(errs) == 0 {
			return nil
		}
		return errs.Unique()
	}

	containsMultifield := isMultiFields(actual)
	if containsMultifield {
		if !isMultiFields(preview) {
			errs = append(errs, fmt.Errorf("not found multi_fields in preview mappings for path: %q", path))
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
		compareErrors := v.compareMappings(path, false, previewFields, actualFields)
		errs = append(errs, compareErrors...)
		// not returning here to keep validating the other fields of this object if any
	}

	// Compare and validate the elements under "properties": objects or fields and its parameters
	propertiesErrs := v.validateObjectProperties(path, true, containsMultifield, preview, actual)
	errs = append(errs, propertiesErrs...)
	if len(errs) == 0 {
		return nil
	}
	return errs.Unique()
}

func (v *MappingValidator) validateObjectProperties(path string, couldBeParametersDefinition, containsMultifield bool, preview, actual map[string]any) multierror.Error {
	var errs multierror.Error
	for key, value := range actual {
		if containsMultifield && key == "fields" {
			// already checked
			continue
		}

		currentPath := currentMappingPath(path, key)
		if skipValidationForField(currentPath) {
			continue
		}

		// This key (object) does not exist in the preview mapping
		if _, ok := preview[key]; !ok {
			if childField, ok := value.(map[string]any); ok {
				if isEmptyObject(childField) {
					// TODO: Should this be raised as an error instead?
					logger.Debugf("field %q is an empty object and it does not exist in the preview", currentPath)
					continue
				}
				ecsErrors := v.validateMappingsNotInPreview(currentPath, childField)
				errs = append(errs, ecsErrors...)
				continue
			}
			// Parameter not defined
			errs = append(errs, fmt.Errorf("field %q is undefined", currentPath))
			continue
		}

		fieldErrs := v.validateObjectMappingAndParameters(preview[key], value, currentPath, true)
		errs = append(errs, fieldErrs...)
	}
	if len(errs) == 0 {
		return nil
	}
	return errs.Unique()
}

// validateMappingsNotInPreview validates the object and the nested objects in the current path with other resources
// like ECS schema, dynamic templates or local fields defined in the package (type array).
func (v *MappingValidator) validateMappingsNotInPreview(currentPath string, childField map[string]any) multierror.Error {
	var errs multierror.Error
	flattenFields, err := flattenMappings(currentPath, childField)
	if err != nil {
		errs = append(errs, err)
		return errs
	}

	for fieldPath, object := range flattenFields {
		if slices.Contains(v.exceptionFields, fieldPath) {
			logger.Warnf("Found exception field, skip its validation: %q", fieldPath)
			return nil
		}

		def, ok := object.(map[string]any)
		if !ok {
			errs = append(errs, fmt.Errorf("invalid field definition/mapping for path: %q", fieldPath))
			continue
		}

		if isEmptyObject(def) {
			logger.Debugf("Skip empty object path: %q", fieldPath)
			continue
		}

		// TODO: validate mapping with dynamic templates first than validating with ECS
		// just raise an error if both validation processes fail

		// are all fields under this key defined in ECS?
		err = v.validateMappingInECSSchema(fieldPath, def)
		if err != nil {
			logger.Warnf("undefined path %q (pending to check dynamic templates)", fieldPath)
			errs = append(errs, fmt.Errorf("field %q is undefined: %w", fieldPath, err))
		}
	}
	return errs.Unique()
}

// validateObjectMappingAndParameters validates the current object or field parameter (currentPath) comparing the values
// in the actual mapping with the values in the preview mapping.
func (v *MappingValidator) validateObjectMappingAndParameters(previewValue, actualValue any, currentPath string, couldBeParametersDefinition bool) multierror.Error {
	var errs multierror.Error
	switch actualValue.(type) {
	case map[string]any:
		// there could be other objects nested under this key/path
		previewField, ok := previewValue.(map[string]any)
		if !ok {
			errs = append(errs, fmt.Errorf("unexpected type in preview mappings for path: %q", currentPath))
		}
		actualField, ok := actualValue.(map[string]any)
		if !ok {
			errs = append(errs, fmt.Errorf("unexpected type in actual mappings for path: %q", currentPath))
		}
		errs = append(errs, v.compareMappings(currentPath, couldBeParametersDefinition, previewField, actualField)...)
	case any:
		// Validate each setting/parameter of the mapping
		// If a mapping exist in both preview and actual, they should be the same. But forcing to compare each parameter just in case
		if previewValue == actualValue {
			return nil
		}
		// Get the string representation of the types via JSON Marshalling
		previewData, err := json.Marshal(previewValue)
		if err != nil {
			errs = append(errs, fmt.Errorf("error marshalling preview value %s (path: %s): %w", previewValue, currentPath, err))
			return errs
		}

		actualData, err := json.Marshal(actualValue)
		if err != nil {
			errs = append(errs, fmt.Errorf("error marshalling actual value %s (path: %s): %w", actualValue, currentPath, err))
			return errs
		}

		// Strings from `json.Marshal` include double quotes, so they need to be removed (e.g. "\"float\"")
		previewDataString := strings.ReplaceAll(string(previewData), "\"", "")
		actualDataString := strings.ReplaceAll(string(actualData), "\"", "")
		// exceptions related to numbers
		// https://github.com/elastic/elastic-package/blob/8cc126ae5015dd336b22901c365e8c98db4e7c15/internal/fields/validate.go#L1234-L1247
		if isNumberTypeField(previewDataString, actualDataString) {
			logger.Debugf("Allowed number fields with different types (preview %s - actual %s)", previewDataString, actualDataString)
			return nil
		}

		errs = append(errs, fmt.Errorf("unexpected value found in mapping for field %q: preview mappings value (%s) different from the actual mappings value (%s)", currentPath, string(previewData), string(actualData)))
	}
	return errs
}
