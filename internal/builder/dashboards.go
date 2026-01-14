// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package builder

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/elastic/elastic-package/internal/common"
)

const (
	panelsAttribute           = "attributes.panelsJSON"
	embeddableConfigAttribute = "embeddableConfig"
)

var fieldsToEncode = []string{
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

func encodeDashboards(buildPackageRoot string) error {
	savedObjects, err := filepath.Glob(filepath.Join(buildPackageRoot, "kibana", "*", "*"))
	if err != nil {
		return err
	}
	for _, file := range savedObjects {
		data, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		output, changed, err := encodeSavedObject(data)
		if err != nil {
			return fmt.Errorf("encoding %s: %w", file, err)
		}

		if changed {
			err = os.WriteFile(file, output, 0644)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// encodeSavedObject encodes all the fields inside a saved object
// which are stored in encoded JSON in Kibana.
// The reason is that for versioning it is much nicer to have the full
// json so only on packaging this is changed.
func encodeSavedObject(data []byte) ([]byte, bool, error) {
	savedObject := common.MapStr{}
	err := json.Unmarshal(data, &savedObject)
	if err != nil {
		return nil, false, fmt.Errorf("unmarshalling saved object failed: %w", err)
	}

	object, changed, err := encodeObjectMapStr(savedObject)
	if err != nil {
		return nil, false, err
	}

	return []byte(object.StringToPrint()), changed, nil
}

func encodeObjectMapStr(object common.MapStr) (common.MapStr, bool, error) {
	object, changed, err := encodeEmbeddedPanels(object)
	if err != nil {
		return nil, false, err
	}
	for _, v := range fieldsToEncode {
		out, err := object.GetValue(v)
		// This means the key did not exists, no conversion needed.
		if err != nil {
			continue
		}

		// It may happen that some objects existing in example directory might be already encoded.
		// In this case skip encoding the field and move to the next one.
		_, isString := out.(string)
		if isString {
			continue
		}

		// Marshal the value to encode it properly.
		r, err := json.Marshal(&out)
		if err != nil {
			return nil, false, err
		}
		_, err = object.Put(v, string(r))
		if err != nil {
			return nil, false, fmt.Errorf("can't put value to the saved object: %w", err)
		}
		changed = true
	}
	return object, changed, nil
}

func encodeEmbeddedPanels(object common.MapStr) (common.MapStr, bool, error) {
	embeddedPanelsValue, err := object.GetValue(panelsAttribute)
	if err == common.ErrKeyNotFound {
		return object, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("retrieving embedded panels failed: %w", err)
	}
	_, isEncoded := embeddedPanelsValue.(string)
	if isEncoded {
		// This is already encoded, probably exported with an old version of elastic-package, do nothing.
		return object, false, nil
	}
	embeddedPanels, ok := embeddedPanelsValue.([]any)
	if !ok {
		return nil, false, fmt.Errorf("expected list of panels, found %T", embeddedPanelsValue)
	}

	changed := false
	for i, panelValue := range embeddedPanels {
		panel, ok := panelValue.(map[string]any)
		if !ok {
			return nil, false, fmt.Errorf("expected panel in map format, found %T", panel)
		}
		embeddableConfigValue, ok := panel[embeddableConfigAttribute]
		if !ok {
			continue
		}
		embeddableConfig, ok := embeddableConfigValue.(map[string]any)
		if !ok {
			return nil, false, fmt.Errorf("embeddable config is not a map, found %T", embeddableConfigValue)
		}
		embeddableConfig, embeddableChanged, err := encodeObjectMapStr(common.MapStr(embeddableConfig))
		if err != nil {
			return nil, false, fmt.Errorf("econding embedded object failed: %w", err)
		}
		if embeddableChanged {
			changed = true
		}

		panel[embeddableConfigAttribute] = embeddableConfig
		embeddedPanels[i] = panel
	}
	object.Put(panelsAttribute, embeddedPanels)

	return object, changed, nil
}
