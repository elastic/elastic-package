// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package export

import (
	"encoding/json"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/common"
)

func filterUnsupportedTypes(object common.MapStr) (common.MapStr, error) {
	aType, err := object.GetValue("type")
	if err != nil {
		return nil, errors.Wrap(err, "missing object type")
	}

	switch aType {
	case "index-pattern": // unsupported types
		return nil, nil
	default:
		return object, nil
	}
}

func decodeObject(object common.MapStr) (common.MapStr, error) {
	for _, fieldToDecode := range encodedFields {
		v, err := object.GetValue(fieldToDecode)
		if err == common.ErrKeyNotFound {
			continue
		} else if err != nil {
			return nil, errors.Wrapf(err, "retrieving value failed (key: %s)", fieldToDecode)
		}

		var target interface{}
		var single common.MapStr
		var array []common.MapStr

		err = json.Unmarshal([]byte(v.(string)), &single)
		if err == nil {
			target = single
		} else {
			err = json.Unmarshal([]byte(v.(string)), &array)
			if err != nil {
				return nil, errors.Wrapf(err, "can't unmarshal encoded field (key: %s)", fieldToDecode)
			}
			target = array
		}
		_, err = object.Put(fieldToDecode, target)
		if err != nil {
			return nil, errors.Wrapf(err, "can't update field (key: %s)", fieldToDecode)
		}
	}
	return object, nil
}

func stripObjectProperties(object common.MapStr) (common.MapStr, error) {
	err := object.Delete("namespaces")
	if err != nil {
		return nil, errors.Wrapf(err, "removing field \"namespaces\" failed")
	}

	err = object.Delete("updated_at")
	if err != nil {
		return nil, errors.Wrapf(err, "removing field \"updated_at\" failed")
	}

	err = object.Delete("version")
	if err != nil {
		return nil, errors.Wrapf(err, "removing field \"version\" failed")
	}
	return object, nil
}
