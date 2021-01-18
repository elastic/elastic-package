// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package packages

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/multierror"
)

// AssetType represents the type of package asset.
type AssetType string

// Supported asset types.
const (
	AssetTypeElasticsearchIndexTemplate  AssetType = "index_template"
	AssetTypeElasticsearchIngestPipeline AssetType = "ingest_pipeline"

	AssetTypeKibanaSavedSearch   AssetType = "search"
	AssetTypeKibanaVisualization AssetType = "visualization"
	AssetTypeKibanaDashboard     AssetType = "dashboard"
	AssetTypeKibanaMap           AssetType = "map"
	AssetTypeKibanaLens          AssetType = "lens"
)

// Asset represents a package asset to be loaded into Kibana or Elasticsearch.
type Asset struct {
	ID         string    `json:"id"`
	Type       AssetType `json:"type"`
	DataStream string
}

// LoadPackageAssets parses the package contents and returns a list of assets defined by the package.
func LoadPackageAssets(pkgRootPath string) ([]Asset, error) {
	assets, err := loadKibanaAssets(pkgRootPath)
	if err != nil {
		return nil, errors.Wrap(err, "could not load kibana assets")
	}

	a, err := loadElasticsearchAssets(pkgRootPath)
	if err != nil {
		return a, errors.Wrap(err, "could not load elasticsearch assets")
	}
	assets = append(assets, a...)

	return assets, nil
}

func loadKibanaAssets(pkgRootPath string) ([]Asset, error) {
	kibanaAssetsFolderPath := filepath.Join(pkgRootPath, "kibana")

	var (
		errs multierror.Error

		assetTypes = []AssetType{
			AssetTypeKibanaDashboard,
			AssetTypeKibanaVisualization,
			AssetTypeKibanaSavedSearch,
			AssetTypeKibanaMap,
			AssetTypeKibanaLens,
		}

		assets []Asset
	)

	for _, assetType := range assetTypes {
		a, err := loadFileBasedAssets(kibanaAssetsFolderPath, assetType)
		if err != nil {
			errs = append(errs, errors.Wrapf(err, "could not load kibana %s assets", assetType))
			continue
		}

		assets = append(assets, a...)
	}

	if len(errs) > 0 {
		return nil, errs
	}

	return assets, nil
}

func loadElasticsearchAssets(pkgRootPath string) ([]Asset, error) {
	packageManifestPath := filepath.Join(pkgRootPath, PackageManifestFile)
	pkgManifest, err := ReadPackageManifest(packageManifestPath)
	if err != nil {
		return nil, errors.Wrap(err, "reading package manifest file failed")
	}

	dataStreamManifestPaths, err := filepath.Glob(filepath.Join(pkgRootPath, "data_stream", "*", DataStreamManifestFile))
	if err != nil {
		return nil, errors.Wrap(err, "could not read data stream manifest file paths")
	}

	var assets []Asset
	for _, dsManifestPath := range dataStreamManifestPaths {
		dsManifest, err := ReadDataStreamManifest(dsManifestPath)
		if err != nil {
			return nil, errors.Wrap(err, "reading data stream manifest failed")
		}

		indexTemplateName := fmt.Sprintf("%s-%s.%s", dsManifest.Type, pkgManifest.Name, dsManifest.Name)
		asset := Asset{
			ID:         indexTemplateName,
			Type:       AssetTypeElasticsearchIndexTemplate,
			DataStream: dsManifest.Name,
		}
		assets = append(assets, asset)

		if dsManifest.Type == dataStreamTypeLogs {
			ingestPipelineName := dsManifest.GetPipelineNameOrDefault()
			if ingestPipelineName == defaultPipelineName {
				ingestPipelineName = fmt.Sprintf("%s-%s.%s-%s", dsManifest.Type, pkgManifest.Name, dsManifest.Name, pkgManifest.Version)
			}
			asset = Asset{
				ID:         ingestPipelineName,
				Type:       AssetTypeElasticsearchIngestPipeline,
				DataStream: dsManifest.Name,
			}
			assets = append(assets, asset)
		}
	}

	return assets, nil
}

func loadFileBasedAssets(kibanaAssetsFolderPath string, assetType AssetType) ([]Asset, error) {
	assetsFolderPath := filepath.Join(kibanaAssetsFolderPath, string(assetType))
	_, err := os.Stat(assetsFolderPath)
	if err != nil && os.IsNotExist(err) {
		// No assets folder defined; nothing to load
		return nil, nil
	}
	if err != nil {
		return nil, errors.Wrapf(err, "error finding kibana %s assets folder", assetType)
	}

	files, err := ioutil.ReadDir(assetsFolderPath)
	if err != nil {
		return nil, errors.Wrapf(err, "could not read %s files", assetType)
	}

	var assets []Asset
	for _, f := range files {
		if f.IsDir() {
			continue
		}

		name := f.Name()
		id := strings.TrimSuffix(name, ".json")

		asset := Asset{
			ID:   id,
			Type: assetType,
		}
		assets = append(assets, asset)
	}

	return assets, nil
}
