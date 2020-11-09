// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package export

import (
	"encoding/json"
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

// Dashboards method exports selected dashboards with references objects. All Kibana objects are saved to local files
// in appropriate directories.
func Dashboards(kibanaDashboardsClient *dashboards.Client, dashboardsIDs []string) error {
	objects, err := kibanaDashboardsClient.Export(dashboardsIDs)
	if err != nil {
		return errors.Wrap(err, "exporting dashboards using Kibana client failed")
	}

	objects, err = transformObjects(objects,
		filterUnsupportedTypes,
		decodeObject,
		stripObjectProperties,
		standardizeObjectProperties)
	if err != nil {
		return errors.Wrap(err, "can't transform Kibana objects")
	}

	err = saveObjectsToFiles(objects)
	if err != nil {
		return errors.Wrap(err, "can't save Kibana objects")
	}
	return nil
}

func transformObjects(objects []common.MapStr, transforms ...func(common.MapStr) (common.MapStr, error)) ([]common.MapStr, error) {
	var decoded []common.MapStr
	var err error

	for _, object := range objects {
		for _, fn := range transforms {
			if object == nil {
				continue
			}

			object, err = fn(object)
			if err != nil {
				id, _ := object.GetValue("id")
				return nil, errors.Wrapf(err, "object transformation failed (ID: %s)", id)
			}
		}

		if object != nil {
			decoded = append(decoded, object)
		}
	}
	return decoded, nil
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
