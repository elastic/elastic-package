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

var fieldsToEncode = []string{
	"attributes.kibanaSavedObjectMeta.searchSourceJSON",
	"attributes.layerListJSON",
	"attributes.mapStateJSON",
	"attributes.optionsJSON",
	"attributes.panelsJSON",
	"attributes.uiStateJSON",
	"attributes.visState",
}

func encodeDashboards(destinationDir string) error {
	savedObjects, err := filepath.Glob(filepath.Join(destinationDir, "kibana", "*", "*"))
	if err != nil {
		return err
	}
	for _, file := range savedObjects {

		data, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		output, changed, err := encodedSavedObject(data)
		if err != nil {
			return err
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
func encodedSavedObject(data []byte) ([]byte, bool, error) {
	savedObject := common.MapStr{}
	err := json.Unmarshal(data, &savedObject)
	if err != nil {
		return nil, false, fmt.Errorf("unmarshalling saved object failed: %w", err)
	}

	var changed bool
	for _, v := range fieldsToEncode {
		out, err := savedObject.GetValue(v)
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
		_, err = savedObject.Put(v, string(r))
		if err != nil {
			return nil, false, fmt.Errorf("can't put value to the saved object: %w", err)
		}
		changed = true
	}
	return []byte(savedObject.StringToPrint()), changed, nil
}
