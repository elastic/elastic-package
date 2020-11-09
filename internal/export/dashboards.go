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

func Dashboards(kibanaDashboardsClient *dashboards.Client, dashboardsIDs []string) error {
	objects, err := kibanaDashboardsClient.Export(dashboardsIDs)
	if err != nil {
		return errors.Wrap(err, "exporting dashboards using Kibana client failed")
	}

	objects = filterObjectsBySupportedType(objects)

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
