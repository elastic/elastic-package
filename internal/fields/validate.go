// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package fields

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/elastic/elastic-package/internal/multierror"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"

	"github.com/elastic/elastic-package/internal/common"
)

// Validator is responsible for fields validation.
type Validator struct {
	schema               []FieldDefinition
	numericKeywordFields map[string]struct{}
}

// ValidatorOption represents an optional flag that can be passed to  CreateValidatorForDataStream.
type ValidatorOption func(*Validator) error

// WithNumericKeywordFields configures the validator to accept specific fields to have numeric-type
// while defined as keyword or constant_keyword.
func WithNumericKeywordFields(fields []string) ValidatorOption {
	return func(v *Validator) error {
		v.numericKeywordFields = make(map[string]struct{}, len(fields))
		for _, field := range fields {
			v.numericKeywordFields[field] = struct{}{}
		}
		return nil
	}
}

// CreateValidatorForDataStream function creates a validator for the data stream.
func CreateValidatorForDataStream(dataStreamRootPath string, opts ...ValidatorOption) (v *Validator, err error) {
	v = new(Validator)
	for _, opt := range opts {
		if err := opt(v); err != nil {
			return nil, err
		}
	}
	v.schema, err = LoadFieldsForDataStream(dataStreamRootPath)
	if err != nil {
		return nil, errors.Wrapf(err, "can't load fields for data stream (path: %s)", dataStreamRootPath)
	}
	return v, nil
}

// LoadFieldsForDataStream function loads fields defined for the given data stream.
func LoadFieldsForDataStream(dataStreamRootPath string) ([]FieldDefinition, error) {
	fieldsDir := filepath.Join(dataStreamRootPath, "fields")
	fileInfos, err := ioutil.ReadDir(fieldsDir)
	if err != nil {
		return nil, errors.Wrapf(err, "reading directory with fields failed (path: %s)", fieldsDir)
	}

	var fields []FieldDefinition
	for _, fileInfo := range fileInfos {
		f := filepath.Join(fieldsDir, fileInfo.Name())
		body, err := ioutil.ReadFile(f)
		if err != nil {
			return nil, errors.Wrap(err, "reading fields file failed")
		}

		var u []FieldDefinition
		err = yaml.Unmarshal(body, &u)
		if err != nil {
			return nil, errors.Wrap(err, "unmarshalling field body failed")
		}
		fields = append(fields, u...)
	}
	return fields, nil
}

// ValidateDocumentBody validates the provided document body.
func (v *Validator) ValidateDocumentBody(body json.RawMessage) multierror.Error {
	var c common.MapStr
	err := json.Unmarshal(body, &c)
	if err != nil {
		var errs multierror.Error
		errs = append(errs, errors.Wrap(err, "unmarshalling document body failed"))
		return errs
	}

	errs := v.validateMapElement("", c)
	if len(errs) == 0 {
		return nil
	}
	return errs
}

// ValidateDocumentMap validates the provided document as common.MapStr.
func (v *Validator) ValidateDocumentMap(body common.MapStr) multierror.Error {
	errs := v.validateMapElement("", body)
	if len(errs) == 0 {
		return nil
	}
	return errs
}

func (v *Validator) validateMapElement(root string, elem common.MapStr) multierror.Error {
	var errs multierror.Error
	for name, val := range elem {
		key := strings.TrimLeft(root+"."+name, ".")

		switch val.(type) {
		case []map[string]interface{}:
			for _, m := range val.([]map[string]interface{}) {
				err := v.validateMapElement(key, m)
				if err != nil {
					errs = append(errs, err...)
				}
			}
		case map[string]interface{}:
			if isFieldTypeFlattened(key, v.schema) {
				// Do not traverse into objects with flattened data types
				// because the entire object is mapped as a single field.
				continue
			}
			err := v.validateMapElement(key, val.(map[string]interface{}))
			if err != nil {
				errs = append(errs, err...)
			}
		default:
			err := v.validateScalarElement(key, val)
			if err != nil {
				errs = append(errs, err)
			}
		}
	}
	return errs
}

func (v *Validator) validateScalarElement(key string, val interface{}) error {
	if key == "" {
		return nil // root key is always valid
	}

	definition := findElementDefinition(key, v.schema)
	if definition == nil && skipValidationForField(key) {
		return nil // generic field, let's skip validation for now
	}
	if definition == nil {
		return fmt.Errorf(`field "%s" is undefined`, key)
	}

	// Convert numeric keyword fields to string for validation.
	if _, found := v.numericKeywordFields[key]; found && isNumericKeyword(*definition, val) {
		val = fmt.Sprintf("%q", val)
	}

	err := parseElementValue(key, *definition, val)
	if err != nil {
		return errors.Wrap(err, "parsing field value failed")
	}
	return nil
}

func isNumericKeyword(definition FieldDefinition, val interface{}) bool {
	_, isNumber := val.(float64)
	return isNumber && (definition.Type == "keyword" || definition.Type == "constant_keyword")
}

// skipValidationForField skips field validation (field presence) of special fields. The special fields are present
// in every (most?) documents collected by Elastic Agent, but aren't defined in any integration in `fields.yml` files.
// FIXME https://github.com/elastic/elastic-package/issues/147
func skipValidationForField(key string) bool {
	return isFieldFamilyMatching("agent", key) ||
		isFieldFamilyMatching("elastic_agent", key) ||
		isFieldFamilyMatching("cloud", key) || // too many common fields
		isFieldFamilyMatching("event", key) || // too many common fields
		isFieldFamilyMatching("host", key) || // too many common fields
		isFieldFamilyMatching("metricset", key) || // field is deprecated
		isFieldFamilyMatching("event.module", key) // field is deprecated
}

func isFieldFamilyMatching(family, key string) bool {
	return key == family || strings.HasPrefix(key, family+".")
}

func isFieldTypeFlattened(key string, fieldDefinitions []FieldDefinition) bool {
	definition := findElementDefinition(key, fieldDefinitions)
	return definition != nil && "flattened" == definition.Type
}

func findElementDefinitionForRoot(root, searchedKey string, FieldDefinitions []FieldDefinition) *FieldDefinition {
	for _, def := range FieldDefinitions {
		key := strings.TrimLeft(root+"."+def.Name, ".")
		if compareKeys(key, def, searchedKey) {
			return &def
		}

		if len(def.Fields) == 0 {
			continue
		}

		fd := findElementDefinitionForRoot(key, searchedKey, def.Fields)
		if fd != nil {
			return fd
		}
	}
	return nil
}

func findElementDefinition(searchedKey string, fieldDefinitions []FieldDefinition) *FieldDefinition {
	return findElementDefinitionForRoot("", searchedKey, fieldDefinitions)
}

func compareKeys(key string, def FieldDefinition, searchedKey string) bool {
	k := strings.ReplaceAll(key, ".", "\\.")
	k = strings.ReplaceAll(k, "*", "[^.]+")

	// Workaround for potential geo_point, as "lon" and "lat" fields are not present in field definitions.
	if def.Type == "geo_point" {
		k += "\\.(lon|lat)"
	}

	k = fmt.Sprintf("^%s$", k)
	matched, err := regexp.MatchString(k, searchedKey)
	if err != nil {
		panic(errors.Wrapf(err, "regexp built using the given field/key (%s) is invalid", k))
	}
	return matched
}

func parseElementValue(key string, definition FieldDefinition, val interface{}) error {
	val, ok := ensureSingleElementValue(val)
	if !ok {
		return nil // it's an array, but it's not possible to extract the single value.
	}

	var valid bool
	switch definition.Type {
	case "date", "ip", "constant_keyword", "keyword", "text":
		var valStr string
		valStr, valid = val.(string)
		if !valid || definition.Pattern == "" {
			break
		}

		valid, err := regexp.MatchString(definition.Pattern, valStr)
		if err != nil {
			return errors.Wrap(err, "invalid pattern")
		}
		if !valid {
			return fmt.Errorf("field %q's value, %s, does not match the expected pattern: %s", key, valStr, definition.Pattern)
		}
	case "float", "long", "double":
		_, valid = val.(float64)
	default:
		valid = true // all other types are considered valid not blocking validation
	}

	if !valid {
		return fmt.Errorf("field %q's Go type, %T, does not match the expected field type: %s (field value: %v)", key, val, definition.Type, val)
	}
	return nil
}

// ensureSingleElementValue extracts single entity from a potential array, which is a valid field representation
// in Elasticsearch. For type assertion we need a single value.
func ensureSingleElementValue(val interface{}) (interface{}, bool) {
	arr, isArray := val.([]interface{})
	if !isArray {
		return val, true
	}
	if len(arr) > 0 {
		return arr[0], true
	}
	return nil, false // false: empty array, can't deduce single value type
}
