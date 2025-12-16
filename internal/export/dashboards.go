// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package export

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Masterminds/semver/v3"
	"github.com/stretchr/testify/assert/yaml"

	"github.com/elastic/elastic-package/internal/common"
	"github.com/elastic/elastic-package/internal/kibana"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/packages"
)

// Dashboards method exports selected dashboards with references objects. All Kibana objects are saved to local files
// in appropriate directories.
func Dashboards(ctx context.Context, kibanaClient *kibana.Client, dashboardsIDs []string) error {
	packageRoot, err := packages.MustFindPackageRoot()
	if err != nil {
		return fmt.Errorf("locating package root failed: %w", err)
	}
	logger.Debugf("Package root found: %s", packageRoot)

	m, err := packages.ReadPackageManifestFromPackageRoot(packageRoot)
	if err != nil {
		return fmt.Errorf("reading package manifest failed (path: %s): %w", packageRoot, err)
	}

	versionInfo, err := kibanaClient.Version()
	if err != nil {
		return fmt.Errorf("getting Kibana version information: %w", err)
	}
	if err := checkKibanaVersion(versionInfo); err != nil {
		return fmt.Errorf("cannot import from this Kibana version: %w", err)
	}

	objects, err := kibanaClient.Export(ctx, dashboardsIDs)
	if err != nil {
		return fmt.Errorf("exporting dashboards using Kibana client failed: %w", err)
	}

	sharedTags, err := readSharedTagsFile(packageRoot)
	if err != nil {
		return fmt.Errorf("reading shared tags file failed: %w", err)
	}

	transformContext := &transformationContext{
		packageName: m.Name,
		sharedTags:  sharedTags,
	}

	objects, err = applyTransformations(transformContext, objects)
	if err != nil {
		return fmt.Errorf("can't transform Kibana objects: %w", err)
	}

	err = saveObjectsToFiles(packageRoot, objects)
	if err != nil {
		return fmt.Errorf("can't save Kibana objects: %w", err)
	}
	return nil
}

func checkKibanaVersion(info kibana.VersionInfo) error {
	version, err := semver.NewVersion(info.Number)
	if err != nil {
		return fmt.Errorf("cannot parse version %s: %w", info.Number, err)
	}

	minVersion := semver.MustParse("8.8.0")
	maxVersion := semver.MustParse("8.10.0")
	if version.LessThan(maxVersion) && !version.LessThan(minVersion) {
		// See:
		// - https://github.com/elastic/elastic-package/issues/1354
		// - https://github.com/elastic/kibana/pull/161969
		return fmt.Errorf("packages with dashboards exported since Kibana %s may not be installed till %s, please export the dashboard/s from a different version", minVersion, maxVersion)
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
			removeFleetManagedTags,
			removeDuplicateSharedTags,
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
			return fmt.Errorf("marshalling Kibana object failed (ID: %s): %w", id.(string), err)
		}

		// Create target directory
		targetDir := filepath.Join(packageRoot, "kibana", aType.(string))
		err = os.MkdirAll(targetDir, 0755)
		if err != nil {
			return fmt.Errorf("creating target directory failed (path: %s): %w", targetDir, err)
		}

		// Save object to file
		objectPath := filepath.Join(targetDir, id.(string)+".json")
		err = os.WriteFile(objectPath, b, 0644)
		if err != nil {
			return fmt.Errorf("writing to file failed: %w", err)
		}
	}
	return nil
}

func readSharedTagsFile(packageRoot string) ([]string, error) {
	b, err := os.ReadFile(filepath.Join(packageRoot, "kibana", "tags.yml"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// No shared tags file, return empty list
			return nil, nil
		}
		return nil, fmt.Errorf("reading shared tags file failed: %w", err)
	}
	var sharedTags []struct {
		Text string `yaml:"text"`
	}
	err = yaml.Unmarshal(b, &sharedTags)
	if err != nil {
		return nil, fmt.Errorf("unmarshalling shared tags file failed: %w", err)
	}
	tags := make([]string, 0, len(sharedTags))
	for _, tag := range sharedTags {
		tags = append(tags, tag.Text)
	}
	return tags, nil
}
