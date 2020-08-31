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

// BuildPackage method builds the package.
func BuildPackage() error {
	packageRoot, found, err := packages.FindPackageRoot()
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

// FindBuildPackagesDirectory method locates the target build directory for packages.
func FindBuildPackagesDirectory() (string, bool, error) {
	workDir, err := os.Getwd()
	if err != nil {
		return "", false, errors.Wrap(err, "locating working directory failed")
	}

	dir := workDir
	for dir != "." {
		path := filepath.Join(dir, "build", "integrations") // TODO add support for other package types
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

func buildPackage(sourcePath string) error {
	buildDir, found, err := FindBuildPackagesDirectory()
	if err != nil {
		return errors.Wrap(err, "locating build directory failed")
	}
	if !found {
		buildDir, err = createBuildPackagesDirectory()
		if err != nil {
			return errors.Wrap(err, "creating new build directory failed")
		}
	}

	m, err := packages.ReadPackageManifest(filepath.Join(sourcePath, packages.PackageManifestFile))
	if err != nil {
		return errors.Wrapf(err, "reading package manifest failed (path: %s)", sourcePath)
	}

	destinationDir := filepath.Join(buildDir, m.Name, m.Version)
	logger.Debugf("Build directory: %s\n", destinationDir)

	logger.Debugf("Clear target directory (path: %s)", destinationDir)
	err = files.ClearDir(destinationDir)
	if err != nil {
		return errors.Wrap(err, "clearing package contents failed")
	}

	logger.Debugf("Copy package content (source: %s)", sourcePath)
	err = files.CopyWithoutDev(sourcePath, destinationDir)
	if err != nil {
		return errors.Wrap(err, "copying package contents failed")
	}

	logger.Debug("Encode dashboards")
	err = encodeDashboards(destinationDir)
	if err != nil {
		return errors.Wrap(err, "encoding dashboards failed")
	}
	return nil
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
