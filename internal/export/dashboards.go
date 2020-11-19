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
	"github.com/elastic/elastic-package/internal/kibana"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/packages"
)

// Dashboards method exports selected dashboards with references objects. All Kibana objects are saved to local files
// in appropriate directories.
func Dashboards(kibanaClient *kibana.Client, dashboardsIDs []string) error {
	packageRoot, err := packages.MustFindPackageRoot()
	if err != nil {
		return errors.Wrap(err, "locating package root failed")
	}
	logger.Debugf("Package root found: %s", packageRoot)

	m, err := packages.ReadPackageManifestFromPackageRoot(packageRoot)
	if err != nil {
		return errors.Wrapf(err, "reading package manifest failed (path: %s)", packageRoot)
	}

	objects, err := kibanaClient.Export(dashboardsIDs)
	if err != nil {
		return errors.Wrap(err, "exporting dashboards using Kibana client failed")
	}

	ctx := &transformationContext{
		packageName: m.Name,
	}

	objects, err = applyTransformations(ctx, objects)
	if err != nil {
		return errors.Wrap(err, "can't transform Kibana objects")
	}

	err = saveObjectsToFiles(packageRoot, objects)
	if err != nil {
		return errors.Wrap(err, "can't save Kibana objects")
	}
	return nil
}

func applyTransformations(ctx *transformationContext, objects []common.MapStr) ([]common.MapStr, error) {
	return newObjectTransformer().
		withContext(ctx).
		withTransforms(filterUnsupportedTypes,
			decodeObject,
			stripObjectProperties,
			standardizeObjectProperties,
			standardizeObjectID).
		transform(objects)
}

func saveObjectsToFiles(packageRoot string, objects []common.MapStr) error {
	logger.Debug("Save Kibana objects to files")

	for _, object := range objects {
		id, _ := object.GetValue("id")
		aType, _ := object.GetValue("type")

		// Marshal object to byte content
		b, err := json.MarshalIndent(&object, "", "    ")
		if err != nil {
			return errors.Wrapf(err, "marshalling Kibana object failed (ID: %s)", id.(string))
		}

		// Create target directory
		targetDir := filepath.Join(packageRoot, "kibana", aType.(string))
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
