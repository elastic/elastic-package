// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package export

import (
	"encoding/json"
	"fmt"

	"github.com/elastic/elastic-package/internal/common"
)

const (
	panelsAttribute           = "attributes.panelsJSON"
	embeddableConfigAttribute = "embeddableConfig"
)

var encodedFields = []string{
	"attributes.controlGroupInput.ignoreParentSettingsJSON",
	"attributes.controlGroupInput.panelsJSON",
	"attributes.kibanaSavedObjectMeta.searchSourceJSON",
	"attributes.layerListJSON",
	"attributes.mapStateJSON",
	"attributes.optionsJSON",
	"attributes.uiStateJSON",
	"attributes.visState",
	panelsAttribute,
}

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

	object, err := decodeEmbeddedPanels(ctx, object)
	if err != nil {
		return nil, err
	}
	return object, nil
}

func decodeEmbeddedPanels(ctx *transformationContext, object common.MapStr) (common.MapStr, error) {
	embeddedPanelsValue, err := object.GetValue(panelsAttribute)
	if err == common.ErrKeyNotFound {
		return object, nil
	}
	if err != nil {
		return nil, fmt.Errorf("retrieving embedded panels failed: %w", err)
	}
	embeddedPanels, ok := embeddedPanelsValue.([]map[string]any)
	if !ok {
		return nil, fmt.Errorf("expected list of panels, found %T", embeddedPanelsValue)
	}
	for i, panel := range embeddedPanels {
		embeddableConfigValue, ok := panel[embeddableConfigAttribute]
		if !ok {
			continue
		}
		embeddableConfig, ok := embeddableConfigValue.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("embeddable config is not a map, found %T", embeddableConfigValue)
		}
		embeddableConfig, err = decodeObject(ctx, common.MapStr(embeddableConfig))
		if err != nil {
			return nil, fmt.Errorf("decoding embedded object failed: %w", err)
		}

		panel[embeddableConfigAttribute] = embeddableConfig
		embeddedPanels[i] = panel
	}
	object.Put(panelsAttribute, embeddedPanels)

	return object, nil
}
