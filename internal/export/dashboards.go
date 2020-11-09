package export

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/common"
	"github.com/elastic/elastic-package/internal/kibana/dashboards"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/packages"
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

func Dashboards(kibanaDashboardsClient *dashboards.Client, dashboardsIDs []string) error {
	objects, err := kibanaDashboardsClient.Export(dashboardsIDs)
	if err != nil {
		return errors.Wrap(err, "exporting dashboards using Kibana client failed")
	}

	objects = filterObjectsBySupportedType(objects)
	objects, err = decodeObjects(objects)
	if err != nil {
		return errors.Wrap(err, "can't decode Kibana objects")
	}

	for _, object := range objects {
		id, _ := object.GetValue("id")
		aType, _ := object.GetValue("type")
		fmt.Println(id, aType)
	}

	err = saveObjectsToFiles(objects)
	if err != nil {
		return errors.Wrap(err, "can't save Kibana objects")
	}
	return nil
}

func filterObjectsBySupportedType(objects []common.MapStr) []common.MapStr {
	var filtered []common.MapStr
	for _, object := range objects {
		aType, _ := object.GetValue("type")
		switch aType {
		case "index-pattern": // unsupported types
		default:
			filtered = append(filtered, object)
		}
	}
	return filtered
}

func decodeObjects(objects []common.MapStr) ([]common.MapStr, error) {
	var decoded []common.MapStr
	for _, object := range objects {
		d, err := decodeObject(object)
		if err != nil {
			id, _ := object.GetValue("id")
			return nil, errors.Wrapf(err, "object decoding failed (ID: %s)", id)
		}

		decoded = append(decoded, d)
	}
	return decoded, nil
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

func saveObjectsToFiles(objects []common.MapStr) error {
	logger.Debug("Save Kibana objects to files")

	root, found, err := packages.FindPackageRoot()
	if err != nil {
		return errors.Wrap(err, "locating package root failed")
	}
	if !found {
		return errors.New("package root not found")
	}
	logger.Debugf("Package root found: %s", root)

	for _, object := range objects {
		id, err := object.GetValue("id")
		if err != nil {
			return errors.Wrap(err, "can't find object ID")
		}

		aType, err := object.GetValue("type")
		if err != nil {
			return errors.Wrap(err, "can't find object type")
		}

		// Marshal object to byte content
		b, err := json.MarshalIndent(&object, "", "    ")
		if err != nil {
			return errors.Wrapf(err, "marshalling Kibana object failed (ID: %s)", id.(string))
		}

		// Create target directory
		targetDir := filepath.Join(root, "kibana", aType.(string))
		err = os.MkdirAll(targetDir, 0755)
		if err != nil {
			return errors.Wrapf(err, "creating target directory failed (path: %s)", targetDir)
		}

		// Save object to file
		objectPath := filepath.Join(targetDir, id.(string)+".json")
		err = ioutil.WriteFile(objectPath, b, 0644)
		if err != nil {
			return errors.Wrap(err, "writing to file failed")
		}
	}
	return nil
}
