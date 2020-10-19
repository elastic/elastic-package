// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package fields

import (
	"encoding/json"
	"io/ioutil"
	"path/filepath"

	"github.com/elastic/elastic-package/internal/common"

	"gopkg.in/yaml.v3"

	"github.com/pkg/errors"
)

// Validator is responsible for fields validation.
type Validator struct {
	schema []field
}

// CreateValidatorForDataStream method creates a validator for the data stream.
func CreateValidatorForDataStream(dataStreamRootPath string) (*Validator, error) {
	fieldsDir := filepath.Join(dataStreamRootPath, "fields")
	fis, err := ioutil.ReadDir(fieldsDir)
	if err != nil {
		return nil, errors.Wrapf(err, "reading directory with fields failed (path: %s)", fieldsDir)
	}

	var fields []field
	for _, fi := range fis {
		f := filepath.Join(fieldsDir, fi.Name())
		body, err := ioutil.ReadFile(f)
		if err != nil {
			return nil, errors.Wrap(err, "reading fields file failed")
		}

		var u []field
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
	return nil // TODO
}
