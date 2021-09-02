// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package packages

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

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

// String method returns a string representation of the asset
func (asset Asset) String() string {
	return fmt.Sprintf("%s (type: %s)", asset.ID, asset.Type)
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

		indexTemplateName := dsManifest.IndexTemplateName(pkgManifest.Name)

		asset := Asset{
			ID:         indexTemplateName,
			Type:       AssetTypeElasticsearchIndexTemplate,
			DataStream: dsManifest.Name,
		}
		assets = append(assets, asset)

		if dsManifest.Type == dataStreamTypeLogs || dsManifest.Type == dataStreamTypeTraces {
			elasticsearchDirPath := filepath.Join(filepath.Dir(dsManifestPath), "elasticsearch", "ingest_pipeline")

			var pipelineFiles []string
			for _, pattern := range []string{"*.json", "*.yml"} {
				files, err := filepath.Glob(filepath.Join(elasticsearchDirPath, pattern))
				if err != nil {
					return nil, errors.Wrapf(err, "listing '%s' in '%s'", pattern, elasticsearchDirPath)
				}
				pipelineFiles = append(pipelineFiles, files...)
			}

			if len(pipelineFiles) == 0 {
				continue // ingest pipeline is not defined
			}

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
	if err != nil && errors.Is(err, os.ErrNotExist) {
		// No assets folder defined; nothing to load
		return nil, nil
	}
	if err != nil {
		return nil, errors.Wrapf(err, "error finding kibana %s assets folder", assetType)
	}

	paths, err := filepath.Glob(filepath.Join(assetsFolderPath, "*.json"))
	if err != nil {
		return nil, errors.Wrapf(err, "could not read %s files", assetType)
	}

	var assets []Asset
	for _, assetPath := range paths {
		assetID, err := readAssetID(assetPath)
		if err != nil {
			return nil, errors.Wrapf(err, "can't read asset ID (path: %s)", assetPath)
		}

		asset := Asset{
			ID:   assetID,
			Type: assetType,
		}
		assets = append(assets, asset)
	}

	return assets, nil
}

func readAssetID(assetPath string) (string, error) {
	content, err := os.ReadFile(assetPath)
	if err != nil {
		return "", errors.Wrap(err, "can't read file body")
	}

	assetBody := struct {
		ID string `json:"id"`
	}{}

	err = json.Unmarshal(content, &assetBody)
	if err != nil {
		return "", errors.Wrap(err, "can't unmarshal asset")
	}

	if assetBody.ID == "" {
		return "", errors.New("empty asset ID")
	}
	return assetBody.ID, nil
}
