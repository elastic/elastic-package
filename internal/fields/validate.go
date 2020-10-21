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
	fis, err := ioutil.ReadDir(fieldsDir)
	if err != nil {
		return nil, errors.Wrapf(err, "reading directory with fields failed (path: %s)", fieldsDir)
	}

	var fields []fieldDefinition
	for _, fi := range fis {
		f := filepath.Join(fieldsDir, fi.Name())
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
func (v *Validator) ValidateDocumentBody(body json.RawMessage) error {
	var c common.MapStr
	err := json.Unmarshal(body, &c)
	if err != nil {
		return errors.Wrap(err, "unmarshalling document body failed")
	}
	return v.validateMapElement("", c)
}

// ValidateDocumentMap validates the provided document as common.MapStr.
func (v *Validator) ValidateDocumentMap(body common.MapStr) error {
	return v.validateMapElement("", body)
}

func (v *Validator) validateMapElement(root string, elem common.MapStr) error {
	var err error
	for name, val := range elem {
		key := strings.TrimLeft(root+"."+name, ".")

		switch val.(type) {
		case []map[string]interface{}:
			for _, m := range val.([]map[string]interface{}) {
				err = v.validateMapElement(key, m)
				if err != nil {
					return err
				}
			}
		case map[string]interface{}:
			err = v.validateMapElement(key, val.(map[string]interface{}))
			if err != nil {
				return err
			}
		default:
			err = v.validateElementFormat(key, val)
			if err != nil {
				return err
			}
			break
		}
	}
	return nil
}

func (v *Validator) validateElementFormat(key string, val interface{}) error {
	if key == "" {
		return nil // root key is always valid
	}

	definition := findElementDefinitionInSlice("", key, v.schema)
	if definition != nil {
		return nil // TODO check field type
	}
	return fmt.Errorf(`field "%s" is not defined`, key)
}

func findElementDefinitionInSlice(root, searchedKey string, fieldDefinitions []fieldDefinition) *fieldDefinition {
	for _, def := range fieldDefinitions {
		key := strings.TrimLeft(root+"."+def.Name, ".")
		if compareKeys(key, def, searchedKey) {
			return &def
		}

		if len(def.Fields) == 0 {
			continue
		}

		fd := findElementDefinitionInSlice(key, searchedKey, def.Fields)
		if fd != nil {
			return fd
		}
	}
	return nil
}

func compareKeys(key string, def fieldDefinition, searchedKey string) bool {
	k := strings.ReplaceAll(key, ".", "\\.")
	k = strings.ReplaceAll(k, "*", "[^.]+")

	// Workaround for potential geo_point, as "lot" and "lat" fields are not present in field definitions.
	if def.Type == "geo_point" {
		k += "\\.(lon|lat)"
	}

	k = fmt.Sprintf("^%s$", k)
	matched, err := regexp.MatchString(k, searchedKey)
	if err != nil {
		panic(err)
	}
	return matched
}
