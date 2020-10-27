// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package fields

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"

	"github.com/elastic/elastic-package/internal/multierror"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"

	"github.com/elastic/elastic-package/internal/common"
)

// Validator is responsible for fields validation.
type Validator struct {
	schema []fieldDefinition
}

// CreateValidatorForDataStream method creates a validator for the data stream.
func CreateValidatorForDataStream(dataStreamRootPath string) (*Validator, error) {
	fieldsDir := filepath.Join(dataStreamRootPath, "fields")
	fileInfos, err := ioutil.ReadDir(fieldsDir)
	if err != nil {
		return nil, errors.Wrapf(err, "reading directory with fields failed (path: %s)", fieldsDir)
	}

	var fields []fieldDefinition
	for _, fileInfo := range fileInfos {
		f := filepath.Join(fieldsDir, fileInfo.Name())
		body, err := ioutil.ReadFile(f)
		if err != nil {
			return nil, errors.Wrap(err, "reading fields file failed")
		}

		var u []fieldDefinition
		err = yaml.Unmarshal(body, &u)
		if err != nil {
			return nil, errors.Wrap(err, "unmarshalling field body failed")
		}
		fields = append(fields, u...)
	}
	return &Validator{
		schema: fields,
	}, nil
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

	definition := findElementDefinition("", key, v.schema)
	if definition == nil && skipValidationForField(key) {
		return nil // generic field, let's skip validation for now
	}
	if definition == nil {
		return fmt.Errorf(`field "%s" is undefined`, key)
	}

	err := parseElementValue(key, *definition, val)
	if err != nil {
		return errors.Wrap(err, "parsing field value failed")
	}
	return nil
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

func findElementDefinition(root, searchedKey string, fieldDefinitions []fieldDefinition) *fieldDefinition {
	for _, def := range fieldDefinitions {
		key := strings.TrimLeft(root+"."+def.Name, ".")
		if compareKeys(key, def, searchedKey) {
			return &def
		}

		if len(def.Fields) == 0 {
			continue
		}

		fd := findElementDefinition(key, searchedKey, def.Fields)
		if fd != nil {
			return fd
		}
	}
	return nil
}

func compareKeys(key string, def fieldDefinition, searchedKey string) bool {
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

func parseElementValue(key string, definition fieldDefinition, val interface{}) error {
	val, ok := ensureSingleElementValue(val)
	if !ok {
		return nil // it's an array, but it's not possible to extract the single value.
	}

	var valid bool
	switch definition.Type {
	case "date", "ip", "constant_keyword", "keyword", "text":
		_, valid = val.(string)
	case "float", "long", "double":
		_, valid = val.(float64)
	default:
		valid = true // all other types are considered valid not blocking validation
	}

	if !valid {
		return fmt.Errorf("field \"%s\" has invalid type, expected: %s, actual Go type: %s", key, definition.Type, reflect.TypeOf(val))
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
