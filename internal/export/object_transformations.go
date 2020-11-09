// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package export

import (
	"encoding/json"
	"strings"

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

func standardizeObjectProperties(object common.MapStr) (common.MapStr, error) {
	for key, value := range object {
		if key == "title" {
			_, err := object.Put(key, standardizeTitleProperty(value.(string)))
			if err != nil {
				return nil, errors.Wrapf(err, "can't update field (key: %s)", key)
			}
			continue
		}

		switch value.(type) {
		case map[string]interface{}:
			newValue, err := standardizeObjectProperties(value.(map[string]interface{}))
			if err != nil {
				return nil, errors.Wrapf(err, "can't standardize object (key: %s)", key)
			}

			_, err = object.Put(key, newValue)
			if err != nil {
				return nil, errors.Wrapf(err, "can't update field (key: %s)", key)
			}
		case []map[string]interface{}:
			vArr := value.([]map[string]interface{})
			for i, obj := range vArr {
				newValue, err := standardizeObjectProperties(obj)
				if err != nil {
					return nil, errors.Wrapf(err, "can't standardize object (array index: %d)", i)
				}
				vArr[i] = newValue
			}

			_, err := object.Put(key, vArr)
			if err != nil {
				return nil, errors.Wrapf(err, "can't update field (key: %s)", key)
			}
		}
	}
	return object, nil
}

func standardizeTitleProperty(title string) string {
	if strings.HasSuffix(title," ECS") {
		return strings.ReplaceAll(title, " ECS", "")
	}
	return title
}