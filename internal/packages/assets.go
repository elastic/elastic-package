package packages

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
)

type AssetType string

const (
	AssetTypeElasticsearchIndexTemplate  AssetType = "index_template"
	AssetTypeElasticsearchIngestPipeline AssetType = "ingest_pipeline"

	AssetTypeKibanaSavedSearch   AssetType = "search"
	AssetTypeKibanaVisualization AssetType = "visualization"
	AssetTypeKibanaDashboard     AssetType = "dashboard"
	AssetTypeKibanaMap           AssetType = "map"
)

type Asset struct {
	ID   string    `json:"id"`
	Type AssetType `json:"type"`
}

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

	return a, nil
}

func loadKibanaAssets(pkgRootPath string) ([]Asset, error) {
	kibanaAssetsFolderPath := filepath.Join(pkgRootPath, "kibana")

	assets, err := loadFileBasedAssets(kibanaAssetsFolderPath, AssetTypeKibanaDashboard)
	if err != nil {
		return nil, errors.Wrap(err, "could not load kibana dashboard assets")
	}

	a, err := loadFileBasedAssets(kibanaAssetsFolderPath, AssetTypeKibanaVisualization)
	if err != nil {
		return nil, errors.Wrap(err, "could not load kibana visualization assets")
	}
	assets = append(assets, a...)

	a, err = loadFileBasedAssets(kibanaAssetsFolderPath, AssetTypeKibanaSavedSearch)
	if err != nil {
		return nil, errors.Wrap(err, "could not load kibana saved search assets")
	}
	assets = append(assets, a...)

	a, err = loadFileBasedAssets(kibanaAssetsFolderPath, AssetTypeKibanaMap)
	if err != nil {
		return nil, errors.Wrap(err, "could not load kibana map assets")
	}
	assets = append(assets, a...)

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

	assets := make([]Asset, 0)
	for _, dsManifestPath := range dataStreamManifestPaths {
		dsManifest, err := ReadDataStreamManifest(dsManifestPath)
		if err != nil {
			return nil, errors.Wrap(err, "reading data stream manifest failed")
		}

		indexTemplateName := fmt.Sprintf("%s-%s.%s", dsManifest.Type, pkgManifest.Name, dsManifest.Name)
		asset := Asset{
			ID:   indexTemplateName,
			Type: AssetTypeElasticsearchIndexTemplate,
		}
		assets = append(assets, asset)

		if dsManifest.Type == "log" {
			ingestPipelineName := dsManifest.GetPipelineNameOrDefault()
			if ingestPipelineName == defaultPipelineName {
				ingestPipelineName = fmt.Sprintf("%s-%s.%s-%s", dsManifest.Type, pkgManifest.Name, dsManifest.Name, pkgManifest.Version)
			}
			asset = Asset{
				ID:   ingestPipelineName,
				Type: AssetTypeElasticsearchIngestPipeline,
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

	assets := make([]Asset, 0)
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
