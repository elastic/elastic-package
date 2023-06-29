// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package export

import (
	"encoding/json"
	"fmt"

	"github.com/elastic/elastic-package/internal/common"
)

var (
	encodedFields = []string{
		"attributes.kibanaSavedObjectMeta.searchSourceJSON",
		"attributes.layerListJSON",
		"attributes.mapStateJSON",
		"attributes.optionsJSON",
		"attributes.panelsJSON",
		"attributes.uiStateJSON",
		"attributes.visState",
	}
)

func decodeObject(ctx *transformationContext, object common.MapStr) (common.MapStr, error) {
	for _, fieldToDecode := range encodedFields {
		v, err := object.GetValue(fieldToDecode)
		if err == common.ErrKeyNotFound {
			continue
		} else if err != nil {
			return nil, fmt.Errorf("retrieving value failed (key: %s): %w", fieldToDecode, err)
		}

		var target interface{}
		var single map[string]interface{}
		var array []map[string]interface{}

		err = json.Unmarshal([]byte(v.(string)), &single)
		if err == nil {
			target = single
		} else {
			err = json.Unmarshal([]byte(v.(string)), &array)
			if err != nil {
				return nil, fmt.Errorf("can't unmarshal encoded field (key: %s): %w", fieldToDecode, err)
			}
			target = array
		}
		_, err = object.Put(fieldToDecode, target)
		if err != nil {
			return nil, fmt.Errorf("can't update field (key: %s): %w", fieldToDecode, err)
		}
	}
	return object, nil
}
