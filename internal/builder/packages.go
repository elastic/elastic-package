// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package builder

import (
	"os"
	"path/filepath"

	"github.com/elastic/elastic-package/internal/files"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/packages"

	"github.com/pkg/errors"
)

// BuildDirectory function locates the target build directory. If the directory doesn't exist, it will create it.
func BuildDirectory() (string, error) {
	buildDir, found, err := findBuildDirectory()
	if err != nil {
		return "", errors.Wrap(err, "locating build directory failed")
	}
	if !found {
		buildDir, err = createBuildDirectory()
		if err != nil {
			return "", errors.Wrap(err, "creating new build directory failed")
		}
	}
	return buildDir, nil
}

func findBuildDirectory() (string, bool, error) {
	workDir, err := os.Getwd()
	if err != nil {
		return "", false, errors.Wrap(err, "locating working directory failed")
	}

	dir := workDir
	for dir != "." {
		path := filepath.Join(dir, "build")
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

// BuildPackagesDirectory function locates the target build directory for packages.
// If the directories path doesn't exist, it will create it.
func BuildPackagesDirectory(packageRoot string) (string, error) {
	buildDir, found, err := FindBuildPackagesDirectory()
	if err != nil {
		return "", errors.Wrap(err, "locating build directory failed")
	}
	if !found {
		buildDir, err = createBuildDirectory("integrations") // TODO add support for other package types
		if err != nil {
			return "", errors.Wrap(err, "creating new build directory failed")
		}
	}

	m, err := packages.ReadPackageManifestFromPackageRoot(packageRoot)
	if err != nil {
		return "", errors.Wrapf(err, "reading package manifest failed (path: %s)", packageRoot)
	}
	return filepath.Join(buildDir, m.Name, m.Version), nil
}

// FindBuildPackagesDirectory function locates the target build directory for packages.
func FindBuildPackagesDirectory() (string, bool, error) {
	buildDir, found, err := findBuildDirectory()
	if err != nil {
		return "", false, err
	}

	if found {
		path := filepath.Join(buildDir, "integrations") // TODO add support for other package types
		fileInfo, err := os.Stat(path)
		if errors.Is(err, os.ErrNotExist) {
			return "", false, nil
		}
		if err != nil {
			return "", false, err
		}

		if fileInfo.IsDir() {
			return path, true, nil
		}
	}

	return "", false, nil
}

// BuildPackage function builds the package.
func BuildPackage(packageRoot string) (string, error) {
	destinationDir, err := BuildPackagesDirectory(packageRoot)
	if err != nil {
		return "", errors.Wrap(err, "locating build directory for package failed")
	}
	logger.Debugf("Build directory: %s\n", destinationDir)

	logger.Debugf("Clear target directory (path: %s)", destinationDir)
	err = files.ClearDir(destinationDir)
	if err != nil {
		return "", errors.Wrap(err, "clearing package contents failed")
	}

	logger.Debugf("Copy package content (source: %s)", packageRoot)
	err = files.CopyWithoutDev(packageRoot, destinationDir)
	if err != nil {
		return "", errors.Wrap(err, "copying package contents failed")
	}

	logger.Debug("Encode dashboards")
	err = encodeDashboards(destinationDir)
	if err != nil {
		return "", errors.Wrap(err, "encoding dashboards failed")
	}

	logger.Debug("Resolve external fields")
	err = resolveExternalFields(packageRoot, destinationDir)
	if err != nil {
		return "", errors.Wrap(err, "resolving external fields failed")
	}
	return destinationDir, nil
}

func createBuildDirectory(dirs ...string) (string, error) {
	workDir, err := os.Getwd()
	if err != nil {
		return "", errors.Wrap(err, "locating working directory failed")
	}

	dir := workDir
	for dir != "." {
		path := filepath.Join(dir, ".git")
		fileInfo, err := os.Stat(path)
		if err == nil && fileInfo.IsDir() {
			p := []string{dir, "build"}
			if len(dirs) > 0 {
				p = append(p, dirs...)
			}
			buildDir := filepath.Join(p...)
			err = os.MkdirAll(buildDir, 0755)
			if err != nil {
				return "", errors.Wrapf(err, "mkdir failed (path: %s)", buildDir)
			}
			return buildDir, nil
		}

		if dir == "/" {
			break
		}
		dir = filepath.Dir(dir)
	}
	return "", errors.New("locating place for build directory failed")
}
