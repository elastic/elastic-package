// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package builder

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/magefile/mage/sh"

	"github.com/elastic/elastic-package/internal/environment"
	"github.com/elastic/elastic-package/internal/files"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/validation"
)

const builtPackagesDir = "packages"
const licenseTextFileName = "LICENSE.txt"

var repositoryLicenseEnv = environment.WithElasticPackagePrefix("REPOSITORY_LICENSE")

type BuildOptions struct {
	PackageRoot string
	BuildDir    string

	CreateZip      bool
	SignPackage    bool
	SkipValidation bool
}

// BuildDirectory function locates the target build directory. If the directory doesn't exist, it will create it.
func BuildDirectory() (string, error) {
	buildDir, found, err := findBuildDirectory()
	if err != nil {
		return "", fmt.Errorf("can't locate build directory: %w", err)
	}
	if !found {
		buildDir, err = createBuildDirectory()
		if err != nil {
			return "", fmt.Errorf("can't create new build directory: %w", err)
		}
	}
	return buildDir, nil
}

func findBuildDirectory() (string, bool, error) {
	workDir, err := os.Getwd()
	if err != nil {
		return "", false, fmt.Errorf("can't locate build directory: %w", err)
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
// It is in the form <buildDir>/packages/<package name>/<package version>.
func BuildPackagesDirectory(packageRoot string, buildDir string) (string, error) {
	if buildDir == "" {
		d, err := buildPackagesRootDirectory()
		if err != nil {
			return "", fmt.Errorf("can't locate build packages root directory: %w", err)
		}
		buildDir = d
	} else {
		info, err := os.Stat(buildDir)
		if err != nil {
			return "", fmt.Errorf("can't check build directory: %w", err)
		}
		if !info.IsDir() {
			return "", fmt.Errorf("build path (%s) expected to be a directory", err)
		}
		d := filepath.Join(buildDir, builtPackagesDir)
		err = os.MkdirAll(d, 0755)
		if err != nil {
			return "", err
		}
		buildDir = d
	}
	m, err := packages.ReadPackageManifestFromPackageRoot(packageRoot)
	if err != nil {
		return "", fmt.Errorf("reading package manifest failed (path: %s): %w", packageRoot, err)
	}
	return filepath.Join(buildDir, m.Name, m.Version), nil
}

// buildPackagesZipPath function locates the target zipped package path.
func buildPackagesZipPath(packageRoot string) (string, error) {
	buildDir, err := buildPackagesRootDirectory()
	if err != nil {
		return "", fmt.Errorf("can't locate build packages root directory: %w", err)
	}
	m, err := packages.ReadPackageManifestFromPackageRoot(packageRoot)
	if err != nil {
		return "", fmt.Errorf("reading package manifest failed (path: %s): %w", packageRoot, err)
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
		return "", fmt.Errorf("can't locate build directory: %w", err)
	}
	if !found {
		buildDir, err = createBuildDirectory(builtPackagesDir)
		if err != nil {
			return "", fmt.Errorf("can't create new build directory: %w", err)
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
		path := filepath.Join(buildDir, builtPackagesDir)
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
func BuildPackage(ctx context.Context, options BuildOptions) (string, error) {
	destinationDir, err := BuildPackagesDirectory(options.PackageRoot, options.BuildDir)
	if err != nil {
		return "", fmt.Errorf("can't locate build directory: %w", err)
	}
	logger.Debugf("Build directory: %s\n", destinationDir)

	logger.Debugf("Clear target directory (path: %s)", destinationDir)
	err = files.ClearDir(destinationDir)
	if err != nil {
		return "", fmt.Errorf("clearing package contents failed: %w", err)
	}

	logger.Debugf("Copy package content (source: %s)", options.PackageRoot)
	err = files.CopyWithoutDev(options.PackageRoot, destinationDir)
	if err != nil {
		return "", fmt.Errorf("copying package contents failed: %w", err)
	}

	logger.Debug("Copy license file if needed")
	err = copyLicenseTextFile(filepath.Join(destinationDir, licenseTextFileName))
	if err != nil {
		return "", fmt.Errorf("copying license text file: %w", err)
	}

	logger.Debug("Encode dashboards")
	err = encodeDashboards(destinationDir)
	if err != nil {
		return "", fmt.Errorf("encoding dashboards failed: %w", err)
	}

	logger.Debug("Resolve external fields")
	err = resolveExternalFields(options.PackageRoot, destinationDir)
	if err != nil {
		return "", fmt.Errorf("resolving external fields failed: %w", err)
	}

	err = addDynamicMappings(options.PackageRoot, destinationDir)
	if err != nil {
		return "", fmt.Errorf("adding dynamic mappings: %w", err)
	}

	logger.Debug("Include linked files")
	linksFS, err := files.CreateLinksFSFromPath(options.PackageRoot)
	if err != nil {
		return "", fmt.Errorf("creating links filesystem failed: %w", err)
	}

	links, err := linksFS.IncludeLinkedFiles(destinationDir)
	if err != nil {
		return "", fmt.Errorf("including linked files failed: %w", err)
	}
	for _, l := range links {
		logger.Debugf("Linked file included (path: %s)", l.TargetFilePath(destinationDir))
	}

	err = resolveTransformDefinitions(destinationDir)
	if err != nil {
		return "", fmt.Errorf("resolving transform manifests failed: %w", err)
	}

	if options.CreateZip {
		return buildZippedPackage(ctx, options, destinationDir)
	}

	if options.SkipValidation {
		logger.Debug("Skip validation of the built package")
		return destinationDir, nil
	}

	logger.Debugf("Validating built package (path: %s)", destinationDir)
	errs, skipped := validation.ValidateAndFilterFromPath(destinationDir)
	if skipped != nil {
		logger.Infof("Skipped errors: %v", skipped)
	}
	if errs != nil {
		return "", fmt.Errorf("invalid content found in built package: %w", errs)
	}
	return destinationDir, nil
}

func buildZippedPackage(ctx context.Context, options BuildOptions, destinationDir string) (string, error) {
	logger.Debug("Build zipped package")
	zippedPackagePath, err := buildPackagesZipPath(options.PackageRoot)
	if err != nil {
		return "", fmt.Errorf("can't evaluate path for the zipped package: %w", err)
	}

	err = files.Zip(ctx, destinationDir, zippedPackagePath)
	if err != nil {
		return "", fmt.Errorf("can't compress the built package (compressed file path: %s): %w", zippedPackagePath, err)
	}

	if options.SkipValidation {
		logger.Debug("Skip validation of the built .zip package")
	} else {
		logger.Debugf("Validating built .zip package (path: %s)", zippedPackagePath)
		errs, skipped := validation.ValidateAndFilterFromZip(zippedPackagePath)
		if skipped != nil {
			logger.Infof("Skipped errors: %v", skipped)
		}
		if errs != nil {
			return "", fmt.Errorf("invalid content found in built zip package: %w", errs)
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
		return fmt.Errorf("reading package manifest failed (path: %s): %w", options.PackageRoot, err)
	}

	err = files.Sign(zippedPackagePath, files.SignOptions{
		PackageName:    m.Name,
		PackageVersion: m.Version,
	})
	if err != nil {
		return fmt.Errorf("can't sign the zipped package (path: %s): %w", zippedPackagePath, err)
	}
	return nil
}

func copyLicenseTextFile(licensePath string) error {
	_, err := os.Stat(licensePath)
	if err == nil {
		logger.Debug("License file in the package will be used")
		return nil
	}

	repositoryLicenseTextFileName, userDefined := os.LookupEnv(repositoryLicenseEnv)
	if !userDefined {
		repositoryLicenseTextFileName = licenseTextFileName
	}

	sourceLicensePath, err := findRepositoryLicense(repositoryLicenseTextFileName)
	if !userDefined && errors.Is(err, os.ErrNotExist) {
		logger.Debug("No license text file is included in package")
		return nil
	}
	if err != nil {
		return fmt.Errorf("failure while looking for license %q in repository: %w", repositoryLicenseTextFileName, err)
	}

	logger.Infof("License text found in %q will be included in package", sourceLicensePath)
	err = sh.Copy(licensePath, sourceLicensePath)
	if err != nil {
		return fmt.Errorf("can't copy license from repository: %w", err)
	}

	return nil
}

func createBuildDirectory(dirs ...string) (string, error) {
	dir, err := files.FindRepositoryRootDirectory()
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
		return "", fmt.Errorf("mkdir failed (path: %s): %w", buildDir, err)
	}
	return buildDir, nil
}

func findRepositoryLicense(licenseTextFileName string) (string, error) {
	dir, err := files.FindRepositoryRootDirectory()
	if err != nil {
		return "", err
	}

	sourceFileName := filepath.Join(dir, licenseTextFileName)
	_, err = os.Stat(sourceFileName)
	if err != nil {
		return "", fmt.Errorf("failed to find repository license: %w", err)
	}

	return sourceFileName, nil
}
