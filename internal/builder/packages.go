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

// BuildPackage function builds the package.
func BuildPackage() (string, error) {
	packageRoot, err := packages.MustFindPackageRoot()
	if err != nil {
		return "", errors.Wrap(err, "locating package root failed")
	}

	target, err := buildPackage(packageRoot)
	if err != nil {
		return "", errors.Wrapf(err, "building package failed (root: %s)", packageRoot)
	}
	return target, nil
}

// FindBuildDirectory locates the target build directory.
func FindBuildDirectory() (string, bool, error) {
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

// FindBuildPackagesDirectory function locates the target build directory for packages.
func FindBuildPackagesDirectory() (string, bool, error) {
	buildDir, found, err := FindBuildDirectory()
	if err != nil {
		return "", false, err
	}

	if found {
		path := filepath.Join(buildDir, "integrations") // TODO add support for other package types
		fileInfo, err := os.Stat(path)
		if os.IsNotExist(err) {
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

func buildPackage(packageRoot string) (string, error) {
	buildDir, found, err := FindBuildPackagesDirectory()
	if err != nil {
		return "", errors.Wrap(err, "locating build directory failed")
	}
	if !found {
		buildDir, err = createBuildPackagesDirectory()
		if err != nil {
			return "", errors.Wrap(err, "creating new build directory failed")
		}
	}

	m, err := packages.ReadPackageManifestFromPackageRoot(packageRoot)
	if err != nil {
		return "", errors.Wrapf(err, "reading package manifest failed (path: %s)", packageRoot)
	}

	destinationDir := filepath.Join(buildDir, m.Name, m.Version)
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
	return destinationDir, nil
}

func createBuildPackagesDirectory() (string, error) {
	workDir, err := os.Getwd()
	if err != nil {
		return "", errors.Wrap(err, "locating working directory failed")
	}

	dir := workDir
	for dir != "." {
		path := filepath.Join(dir, ".git")
		fileInfo, err := os.Stat(path)
		if err == nil && fileInfo.IsDir() {
			buildDir := filepath.Join(dir, "build", "integrations") // TODO add support for other package types
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
