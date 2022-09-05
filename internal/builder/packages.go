// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package builder

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/magefile/mage/sh"
	"github.com/pkg/errors"

	"github.com/elastic/package-spec/code/go/pkg/validator"

	"github.com/elastic/elastic-package/internal/files"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/packages"
)

const builtPackagesFolder = "packages"
const licenseTextFileName = "LICENSE.txt"

type BuildOptions struct {
	PackageRoot string

	CreateZip      bool
	SignPackage    bool
	SkipValidation bool
}

// BuildDirectory function locates the target build directory. If the directory doesn't exist, it will create it.
func BuildDirectory() (string, error) {
	buildDir, found, err := findBuildDirectory()
	if err != nil {
		return "", errors.Wrap(err, "can't locate build directory")
	}
	if !found {
		buildDir, err = createBuildDirectory()
		if err != nil {
			return "", errors.Wrap(err, "can't create new build directory")
		}
	}
	return buildDir, nil
}

func findBuildDirectory() (string, bool, error) {
	workDir, err := os.Getwd()
	if err != nil {
		return "", false, errors.Wrap(err, "can't locate build directory")
	}

	dir := workDir
	// required for multi platform support
	root := fmt.Sprintf("%s%c", filepath.VolumeName(dir), os.PathSeparator)
	for dir != "." {
		path := filepath.Join(dir, "build")
		fileInfo, err := os.Stat(path)
		if err == nil && fileInfo.IsDir() {
			return path, true, nil
		}

		if dir == root {
			break
		}
		dir = filepath.Dir(dir)
	}
	return "", false, nil
}

// BuildPackagesDirectory function locates the target build directory for the package.
func BuildPackagesDirectory(packageRoot string) (string, error) {
	buildDir, err := buildPackagesRootDirectory()
	if err != nil {
		return "", errors.Wrap(err, "can't locate build packages root directory")
	}
	m, err := packages.ReadPackageManifestFromPackageRoot(packageRoot)
	if err != nil {
		return "", errors.Wrapf(err, "reading package manifest failed (path: %s)", packageRoot)
	}
	return filepath.Join(buildDir, m.Name, m.Version), nil
}

// buildPackagesZipPath function locates the target zipped package path.
func buildPackagesZipPath(packageRoot string) (string, error) {
	buildDir, err := buildPackagesRootDirectory()
	if err != nil {
		return "", errors.Wrap(err, "can't locate build packages root directory")
	}
	m, err := packages.ReadPackageManifestFromPackageRoot(packageRoot)
	if err != nil {
		return "", errors.Wrapf(err, "reading package manifest failed (path: %s)", packageRoot)
	}
	return ZippedBuiltPackagePath(buildDir, *m), nil
}

// ZippedBuiltPackagePath function returns the path to zipped built package.
func ZippedBuiltPackagePath(buildDir string, m packages.PackageManifest) string {
	return filepath.Join(buildDir, fmt.Sprintf("%s-%s.zip", m.Name, m.Version))
}

func buildPackagesRootDirectory() (string, error) {
	buildDir, found, err := FindBuildPackagesDirectory()
	if err != nil {
		return "", errors.Wrap(err, "can't locate build directory")
	}
	if !found {
		buildDir, err = createBuildDirectory(builtPackagesFolder)
		if err != nil {
			return "", errors.Wrap(err, "can't create new build directory")
		}
	}
	return buildDir, nil
}

// FindBuildPackagesDirectory function locates the target build directory for packages.
func FindBuildPackagesDirectory() (string, bool, error) {
	buildDir, found, err := findBuildDirectory()
	if err != nil {
		return "", false, err
	}

	if found {
		path := filepath.Join(buildDir, builtPackagesFolder)
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
func BuildPackage(options BuildOptions) (string, error) {
	destinationDir, err := BuildPackagesDirectory(options.PackageRoot)
	if err != nil {
		return "", errors.Wrap(err, "can't locate build directory")
	}
	logger.Debugf("Build directory: %s\n", destinationDir)

	logger.Debugf("Clear target directory (path: %s)", destinationDir)
	err = files.ClearDir(destinationDir)
	if err != nil {
		return "", errors.Wrap(err, "clearing package contents failed")
	}

	logger.Debugf("Copy package content (source: %s)", options.PackageRoot)
	err = files.CopyWithoutDev(options.PackageRoot, destinationDir)
	if err != nil {
		return "", errors.Wrap(err, "copying package contents failed")
	}

	logger.Debug("Copy license file if needed")
	err = copyLicenseTextFile(filepath.Join(destinationDir, licenseTextFileName))
	if err != nil {
		return "", errors.Wrap(err, "copying license text file")
	}

	logger.Debug("Encode dashboards")
	err = encodeDashboards(destinationDir)
	if err != nil {
		return "", errors.Wrap(err, "encoding dashboards failed")
	}

	logger.Debug("Resolve external fields")
	err = resolveExternalFields(options.PackageRoot, destinationDir)
	if err != nil {
		return "", errors.Wrap(err, "resolving external fields failed")
	}

	if options.CreateZip {
		return buildZippedPackage(options, destinationDir)
	}

	if options.SkipValidation {
		logger.Debug("Skip validation of the built package")
		return destinationDir, nil
	}

	err = validator.ValidateFromPath(destinationDir)
	if err != nil {
		return "", errors.Wrap(err, "invalid content found in built package")
	}
	return destinationDir, nil
}

func buildZippedPackage(options BuildOptions, destinationDir string) (string, error) {
	logger.Debug("Build zipped package")
	zippedPackagePath, err := buildPackagesZipPath(options.PackageRoot)
	if err != nil {
		return "", errors.Wrap(err, "can't evaluate path for the zipped package")
	}

	err = files.Zip(destinationDir, zippedPackagePath)
	if err != nil {
		return "", errors.Wrapf(err, "can't compress the built package (compressed file path: %s)", zippedPackagePath)
	}

	if options.SkipValidation {
		logger.Debug("Skip validation of the built .zip package")
	} else {
		err = validator.ValidateFromZip(zippedPackagePath)
		if err != nil {
			return "", errors.Wrapf(err, "invalid content found in built zip package")
		}
	}

	if options.SignPackage {
		err := signZippedPackage(options, zippedPackagePath)
		if err != nil {
			return "", err
		}
	}

	return zippedPackagePath, nil
}

func signZippedPackage(options BuildOptions, zippedPackagePath string) error {
	logger.Debug("Sign the package")
	m, err := packages.ReadPackageManifestFromPackageRoot(options.PackageRoot)
	if err != nil {
		return errors.Wrapf(err, "reading package manifest failed (path: %s)", options.PackageRoot)
	}

	err = files.Sign(zippedPackagePath, files.SignOptions{
		PackageName:    m.Name,
		PackageVersion: m.Version,
	})
	if err != nil {
		return errors.Wrapf(err, "can't sign the zipped package (path: %s)", zippedPackagePath)
	}
	return nil
}

func copyLicenseTextFile(licensePath string) error {
	_, err := os.Stat(licensePath)
	if err == nil {
		logger.Debug("License file in the package will be used")
		return nil
	}

	sourceLicensePath, err := findRepositoryLicense()
	if errors.Is(err, os.ErrNotExist) {
		logger.Debug("No license text file is included in package")
		return nil
	}
	if err != nil {
		return errors.Wrap(err, "failure while looking for license in repository")
	}

	logger.Infof("License text found in %q will be included in package", sourceLicensePath)
	err = sh.Copy(licensePath, sourceLicensePath)
	if err != nil {
		return errors.Wrap(err, "can't copy license from repository")
	}

	return nil
}

func createBuildDirectory(dirs ...string) (string, error) {
	dir, err := findRepositoryRootDirectory()
	if errors.Is(err, os.ErrNotExist) {
		return "", errors.New("package can be only built inside of a Git repository (.git folder is used as reference point)")
	}
	if err != nil {
		return "", err
	}

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

func findRepositoryRootDirectory() (string, error) {
	workDir, err := os.Getwd()
	if err != nil {
		return "", errors.Wrap(err, "locating working directory failed")
	}

	dir := workDir
	for dir != "." {
		path := filepath.Join(dir, ".git")
		fileInfo, err := os.Stat(path)
		if err == nil && fileInfo.IsDir() {
			return dir, nil
		}

		if dir == "/" {
			break
		}
		dir = filepath.Dir(dir)
	}

	return "", os.ErrNotExist
}

func findRepositoryLicense() (string, error) {
	dir, err := findRepositoryRootDirectory()
	if err != nil {
		return "", err
	}

	sourceLicensePath := filepath.Join(dir, licenseTextFileName)
	_, err = os.Stat(sourceLicensePath)
	if err != nil {
		return "", err
	}

	return sourceLicensePath, nil
}
