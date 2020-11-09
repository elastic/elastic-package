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

		var target interface{}
		var err error
		var updated bool

		switch value.(type) {
		case map[string]interface{}:
			target, err = standardizeObjectProperties(value.(map[string]interface{}))
			if err != nil {
				return nil, errors.Wrapf(err, "can't standardize object (key: %s)", key)
			}
			updated = true
		case []map[string]interface{}:
			arr := value.([]map[string]interface{})
			for i, obj := range arr {
				newValue, err := standardizeObjectProperties(obj)
				if err != nil {
					return nil, errors.Wrapf(err, "can't standardize object (array index: %d)", i)
				}
				arr[i] = newValue
			}
			target = arr
			updated = true
		}

		if !updated {
			continue
		}

		_, err = object.Put(key, target)
		if err != nil {
			return nil, errors.Wrapf(err, "can't update field (key: %s)", key)
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
