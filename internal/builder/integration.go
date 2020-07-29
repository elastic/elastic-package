package builder

import (
	"fmt"
	"github.com/elastic/elastic-package/internal/files"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
)

const packageManifestFile = "manifest.yml"

type packageManifest struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Version string `json:"version"`
}

// BuildIntegration method builds the integration package.
func BuildIntegration() error {
	packageRoot, found, err := findPackageRoot()
	if !found {
		return errors.New("package root not found")
	}
	if err != nil {
		return errors.Wrap(err, "locating package root failed")
	}

	err = buildPackage(packageRoot)
	if err != nil {
		return errors.Wrapf(err, "building package failed (root: %s)", packageRoot)
	}
	return nil
}

// FindBuildPackagesDirectory method locates the target build directory for integrations.
func FindBuildPackagesDirectory() (string, bool, error) {
	workDir, err := os.Getwd()
	if err != nil {
		return "", false, errors.Wrap(err, "locating working directory failed")
	}

	dir := workDir
	for dir != "." {
		path := filepath.Join(dir, "build", "integrations") // TODO add support for other repositories
		fileInfo, err := os.Stat(path)
		if err == nil && fileInfo.IsDir() {
			return path, true, nil
		}

		if dir == "/" {
			break
		}
		dir = filepath.Dir(dir)
	}
	return "", false, nil
}

func findPackageRoot() (string, bool, error) {
	workDir, err := os.Getwd()
	if err != nil {
		return "", false, errors.Wrap(err, "locating working directory failed")
	}

	dir := workDir
	for dir != "." {
		path := filepath.Join(dir, packageManifestFile)
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

func isPackageManifest(path string) (bool, error) {
	m, err := readPackageManifest(path)
	if err != nil {
		return false, errors.Wrapf(err, "reading package manifest failed (path: %s)", path)
	}
	return m.Type == "integration" && m.Version != "", nil
}

func buildPackage(sourcePath string) error {
	fmt.Printf("Building integration: %s\n", sourcePath)

	buildDir, found, err := FindBuildPackagesDirectory()
	if err != nil {
		return errors.Wrap(err, "locating build directory failed")
	}
	if !found {
		return errors.New("build directory not found")
	}

	m, err := readPackageManifest(filepath.Join(sourcePath, packageManifestFile))
	if err != nil {
		return errors.Wrapf(err, "reading package manifest failed (path: %s)", sourcePath)
	}

	destinationDir := filepath.Join(buildDir, m.Name, m.Version)
	fmt.Printf("Build directory: %s\n", destinationDir)

	fmt.Printf("Clear target directory (path: %s)\n", destinationDir)
	err = files.ClearDir(destinationDir)
	if err != nil {
		return errors.Wrap(err, "clearing package contents failed")
	}

	fmt.Printf("Copy package content (source: %s)\n", sourcePath)
	err = files.CopyAll(sourcePath, destinationDir)
	if err != nil {
		return errors.Wrap(err, "copying package contents failed")
	}

	fmt.Println("Encode dashboards")
	err = encodeDashboards(destinationDir)
	if err != nil {
		return errors.Wrap(err, "encoding dashboards failed")
	}

	fmt.Println("Done.")
	return nil
}

func readPackageManifest(path string) (*packageManifest, error) {
	content, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, errors.Wrapf(err, "reading file body failed (path: %s)", path)
	}

	var m packageManifest
	err = yaml.Unmarshal(content, &m)
	if err != nil {
		return nil, errors.Wrapf(err, "unmarshalling package manifest failed (path: %s)", path)
	}
	return &m, nil
}
