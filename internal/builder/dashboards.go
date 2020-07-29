package builder

import (
	"encoding/json"
	"io/ioutil"
	"path/filepath"

	"github.com/pkg/errors"
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

		data, err := ioutil.ReadFile(file)
		if err != nil {
			return err
		}
		output, changed, err := encodedSavedObject(data)
		if err != nil {
			return err
		}

		if changed {
			err = ioutil.WriteFile(file, output, 0644)
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
	savedObject := mapStr{}
	err := json.Unmarshal(data, &savedObject)
	if err != nil {
		return nil, false, errors.Wrapf(err, "unmarshalling saved object failed")
	}

	var changed bool
	for _, v := range fieldsToEncode {
		out, err := savedObject.getValue(v)
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
		_, err = savedObject.put(v, string(r))
		if err != nil {
			return nil, false, errors.Wrapf(err, "can't put value to the saved object")
		}
		changed = true
	}
	return []byte(savedObject.stringToPrint()), changed, nil
}
