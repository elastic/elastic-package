// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package export

import (
	"strings"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/common"
)

func standardizeObjectProperties(object common.MapStr) (common.MapStr, error) {
	for key, value := range object {
		if key == "title" {
			_, err := object.Put(key, adjustTitleProperty(value.(string)))
			if err != nil {
				return nil, errors.Wrapf(err, "can't update field (key: %s)", key)
			}
			continue
		}

		if m, ok := value.(map[string]interface{}); ok {
			newValue, err := standardizeObjectProperties(m)
			if err != nil {
				return nil, errors.Wrapf(err, "can't standardize object (key: %s)", key)
			}

			_, err = object.Put(key, newValue)
			if err != nil {
				return nil, errors.Wrapf(err, "can't update field (key: %s)", key)
			}
			continue
		}

		if mArr, ok := value.([]map[string]interface{}); ok {
			for i, obj := range mArr {
				newValue, err := standardizeObjectProperties(obj)
				if err != nil {
					return nil, errors.Wrapf(err, "can't standardize object (array index: %d)", i)
				}
				mArr[i] = newValue
			}

			_, err := object.Put(key, mArr)
			if err != nil {
				return nil, errors.Wrapf(err, "can't update field (key: %s)", key)
			}
			continue
		}
	}
	return object, nil
}

func adjustTitleProperty(title string) string {
	if strings.HasSuffix(title, " ECS") {
		return strings.ReplaceAll(title, " ECS", "")
	}
	return title
}
