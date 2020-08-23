package packages

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

const (
	// PackageManifestFile is the name of the package's main manifest file.
	PackageManifestFile = "manifest.yml"

	// DatasetManifestFile is the name of the dataset's manifest file.
	DatasetManifestFile = "manifest.yml"
)

// PackageManifest represents the basic structure of a package's manifest
type PackageManifest struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Version string `json:"version"`
}

// DatasetManifest represents the structure of a dataset's manifest
type DatasetManifest struct {
	Title         string `json:"title"`
	Type          string `json:"type"`
	Elasticsearch *struct {
		IngestPipelineName string `json:"ingest_pipeline.name"`
	} `json:"elasticsearch"`
}

// FindPackageRoot finds and returns the path to the root folder of a package.
func FindPackageRoot() (string, bool, error) {
	workDir, err := os.Getwd()
	if err != nil {
		return "", false, errors.Wrap(err, "locating working directory failed")
	}

	dir := workDir
	for dir != "." {
		path := filepath.Join(dir, PackageManifestFile)
		fileInfo, err := os.Stat(path)
		if err == nil && !fileInfo.IsDir() {
			ok, err := isPackageManifest(path)
			if err != nil {
				return "", false, errors.Wrapf(err, "verifying manifest file failed (path: %s)", path)
			}
			if ok {
				return dir, true, nil
			}
		}

		if dir == "/" {
			break
		}
		dir = filepath.Dir(dir)
	}
	return "", false, nil
}

// FindDatasetRootForPath finds and returns the path to the root folder of a dataset.
func FindDatasetRootForPath(workDir string) (string, bool, error) {
	dir := workDir
	for dir != "." {
		path := filepath.Join(dir, DatasetManifestFile)
		fileInfo, err := os.Stat(path)
		if err == nil && !fileInfo.IsDir() {
			ok, err := isDatasetManifest(path)
			if err != nil {
				return "", false, errors.Wrapf(err, "verifying manifest file failed (path: %s)", path)
			}
			if ok {
				return dir, true, nil
			}
		}

		if dir == "/" {
			break
		}
		dir = filepath.Dir(dir)
	}
	return "", false, nil
}

// ReadPackageManifest reads and parses the given package manifest file.
func ReadPackageManifest(path string) (*PackageManifest, error) {
	content, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, errors.Wrapf(err, "reading file body failed (path: %s)", path)
	}

	var m PackageManifest
	err = yaml.Unmarshal(content, &m)
	if err != nil {
		return nil, errors.Wrapf(err, "unmarshalling package manifest failed (path: %s)", path)
	}
	return &m, nil
}

// ReadDatasetManifest reads and parses the given dataset manifest file.
func ReadDatasetManifest(path string) (*DatasetManifest, error) {
	content, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, errors.Wrapf(err, "reading file body failed (path: %s)", path)
	}

	var m DatasetManifest
	err = yaml.Unmarshal(content, &m)
	if err != nil {
		return nil, errors.Wrapf(err, "unmarshalling dataset manifest failed (path: %s)", path)
	}
	return &m, nil
}

func isPackageManifest(path string) (bool, error) {
	m, err := ReadPackageManifest(path)
	if err != nil {
		return false, errors.Wrapf(err, "reading package manifest failed (path: %s)", path)
	}
	return m.Type == "integration" && m.Version != "", nil // TODO add support for other package types
}

func isDatasetManifest(path string) (bool, error) {
	m, err := ReadDatasetManifest(path)
	if err != nil {
		return false, errors.Wrapf(err, "reading package manifest failed (path: %s)", path)
	}
	return m.Title != "" && (m.Type == "logs" || m.Type == "metrics"), nil
}
