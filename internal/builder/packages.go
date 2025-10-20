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
	PackageRootPath string // path to the package source content
	BuildDir        string // directory where all the built packages are placed and zipped packages are stored
	RepositoryRoot  *os.Root

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

// buildPackagesZipPath function returns the path to zipped built package.
func buildPackagesZipPath(packageRoot, buildDir string) (string, error) {
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
	// builtPackageDir is the directory where the built package content is placed
	// eg. <buildDir>/packages/<package name>/<package version>
	builtPackageDir, err := BuildPackagesDirectory(options.PackageRootPath, options.BuildDir)
	if err != nil {
		return "", fmt.Errorf("can't locate build directory: %w", err)
	}
	logger.Debugf("Build directory: %s\n", builtPackageDir)

	logger.Debugf("Clear target directory (path: %s)", builtPackageDir)
	err = files.ClearDir(builtPackageDir)
	if err != nil {
		return "", fmt.Errorf("clearing package contents failed: %w", err)
	}

	logger.Debugf("Copy package content (source: %s)", options.PackageRootPath)
	err = files.CopyWithoutDev(options.PackageRootPath, builtPackageDir)
	if err != nil {
		return "", fmt.Errorf("copying package contents failed: %w", err)
	}

	logger.Debug("Copy license file if needed")
	destinationLicenseFilePath := filepath.Join(builtPackageDir, licenseTextFileName)
	err = copyLicenseTextFile(options.RepositoryRoot, destinationLicenseFilePath)
	if err != nil {
		return "", fmt.Errorf("copying license text file: %w", err)
	}

	logger.Debug("Encode dashboards")
	err = encodeDashboards(builtPackageDir)
	if err != nil {
		return "", fmt.Errorf("encoding dashboards failed: %w", err)
	}

	logger.Debug("Resolve external fields")
	err = resolveExternalFields(options.PackageRootPath, builtPackageDir)
	if err != nil {
		return "", fmt.Errorf("resolving external fields failed: %w", err)
	}

	err = addDynamicMappings(options.PackageRootPath, builtPackageDir)
	if err != nil {
		return "", fmt.Errorf("adding dynamic mappings: %w", err)
	}

	logger.Debug("Include linked files")
	linksFS, err := files.CreateLinksFSFromPath(options.RepositoryRoot, options.PackageRootPath)
	if err != nil {
		return "", fmt.Errorf("creating links filesystem failed: %w", err)
	}

	links, err := linksFS.IncludeLinkedFiles(builtPackageDir)
	if err != nil {
		return "", fmt.Errorf("including linked files failed: %w", err)
	}
	for _, l := range links {
		logger.Debugf("Linked file included (path: %s)", l.TargetRelPath)
	}

	err = resolveTransformDefinitions(builtPackageDir)
	if err != nil {
		return "", fmt.Errorf("resolving transform manifests failed: %w", err)
	}

	if options.CreateZip {
		return buildZippedPackage(ctx, options, builtPackageDir, options.BuildDir)
	}

	if options.SkipValidation {
		logger.Debug("Skip validation of the built package")
		return builtPackageDir, nil
	}

	logger.Debugf("Validating built package (path: %s)", builtPackageDir)
	errs, skipped := validation.ValidateAndFilterFromPath(builtPackageDir)
	if skipped != nil {
		logger.Infof("Skipped errors: %v", skipped)
	}
	if errs != nil {
		return "", fmt.Errorf("invalid content found in built package: %w", errs)
	}
	return builtPackageDir, nil
}

// buildZippedPackage function builds the zipped package from the sourcePackageDir and stores it in buildDir.
func buildZippedPackage(ctx context.Context, options BuildOptions, sourcePackageDir, buildDir string) (string, error) {
	logger.Debug("Build zipped package")
	zippedPackagePath, err := buildPackagesZipPath(options.PackageRootPath, buildDir)
	if err != nil {
		return "", fmt.Errorf("can't evaluate path for the zipped package: %w", err)
	}

	err = files.Zip(ctx, sourcePackageDir, zippedPackagePath)
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
	m, err := packages.ReadPackageManifestFromPackageRoot(options.PackageRootPath)
	if err != nil {
		return fmt.Errorf("reading package manifest failed (path: %s): %w", options.PackageRootPath, err)
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

// copyLicenseTextFile checks the targetLicencePath and copies the license file from the repository root if needed.
// If the targetLicensePath already exists, it will skip copying.
// If the targetLicensePath does not exist, it will look for a source license file in the repository root and copy it to the targetLicensePath.
// The source license file name can be overridden by setting the REPOSITORY_LICENSE environment variable.
func copyLicenseTextFile(repositoryRoot *os.Root, targetLicensePath string) error {
	if !filepath.IsAbs(targetLicensePath) {
		return fmt.Errorf("target license path (%s) is not an absolute path", targetLicensePath)
	}

	// if the given path exists, skip copying
	info, err := os.Stat(targetLicensePath)
	if err == nil && !info.IsDir() {
		logger.Debug("License file in the package will be used")
		return nil
	}
	// if the given path does not exist, continue
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("can't check license path (%s): %w", targetLicensePath, err)
	}
	// if the given path exists, but is a directory, return an error
	if info != nil && info.IsDir() {
		return fmt.Errorf("license path (%s) is a directory", targetLicensePath)
	}

	// lookup for the license file in the repository
	// default license name can be overridden by the user
	repositoryLicenseTextFileName, userDefined := os.LookupEnv(repositoryLicenseEnv)
	if !userDefined {
		repositoryLicenseTextFileName = licenseTextFileName
	}

	// sourceLicensePath is an absolute path to the repositoryLicenseTextFileName in the repository root
	sourceLicensePath, err := findRepositoryLicensePath(repositoryRoot, repositoryLicenseTextFileName)
	if !userDefined && errors.Is(err, os.ErrNotExist) {
		logger.Debug("No license text file is included in package")
		return nil
	}
	if err != nil {
		return fmt.Errorf("failure while looking for license %q in repository: %w", repositoryLicenseTextFileName, err)
	}

	logger.Infof("License text found in %q will be included in package", sourceLicensePath)
	err = sh.Copy(targetLicensePath, sourceLicensePath)
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

// findRepositoryLicensePath checks if a license file exists at the specified path and its not empty.
// If the file exists, it returns the path; otherwise, it returns an error indicating
// that the repository license could not be found.
//
// Parameters:
//
//	repositoryLicenseTextFileName - the relative path to the license file from the repository root.
//
// Returns:
//
//	string - the license file absolute path if found.
//	error  - an error if the license file does not exist.
func findRepositoryLicensePath(repositoryRoot *os.Root, repositoryLicenseTextFileName string) (string, error) {
	// root.ReadFile is supported after go1.25,
	// https://go.dev/doc/go1.25
	bytes, err := os.ReadFile(filepath.Join(repositoryRoot.Name(), repositoryLicenseTextFileName))
	if err != nil {
		return "", fmt.Errorf("failed to read repository license: %w", err)
	}
	if len(bytes) == 0 {
		return "", fmt.Errorf("repository license file is empty")
	}
	path := filepath.Join(repositoryRoot.Name(), repositoryLicenseTextFileName)
	return path, nil
}
