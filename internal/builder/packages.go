// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package builder

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/magefile/mage/sh"

	"github.com/elastic/elastic-package/internal/environment"
	"github.com/elastic/elastic-package/internal/files"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/validation"
)

const builtPackagesFolder = "packages"
const licenseTextFileName = "LICENSE.txt"

var repositoryLicenseEnv = environment.WithElasticPackagePrefix("REPOSITORY_LICENSE")

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
func BuildPackagesDirectory(packageRoot string) (string, error) {
	buildDir, err := buildPackagesRootDirectory()
	if err != nil {
		return "", fmt.Errorf("can't locate build packages root directory: %w", err)
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
		buildDir, err = createBuildDirectory(builtPackagesFolder)
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
func (b *Builder) BuildPackage() (string, error) {
	destinationDir, err := BuildPackagesDirectory(b.packageRoot)
	if err != nil {
		return "", fmt.Errorf("can't locate build directory: %w", err)
	}
	b.logger.Debug("Build directory", slog.String("path", destinationDir))

	b.logger.Debug("Clear target directory", slog.String("path", destinationDir))
	err = files.ClearDir(destinationDir)
	if err != nil {
		return "", fmt.Errorf("clearing package contents failed: %w", err)
	}

	b.logger.Debug("Copy package content")
	err = files.CopyWithoutDev(b.packageRoot, destinationDir)
	if err != nil {
		return "", fmt.Errorf("copying package contents failed: %w", err)
	}

	b.logger.Debug("Copy license file if needed")
	err = b.copyLicenseTextFile(filepath.Join(destinationDir, licenseTextFileName))
	if err != nil {
		return "", fmt.Errorf("copying license text file: %w", err)
	}

	b.logger.Debug("Encode dashboards")
	err = encodeDashboards(destinationDir)
	if err != nil {
		return "", fmt.Errorf("encoding dashboards failed: %w", err)
	}

	b.logger.Debug("Resolve external fields")
	err = b.resolveExternalFields(destinationDir)
	if err != nil {
		return "", fmt.Errorf("resolving external fields failed: %w", err)
	}

	err = b.addDynamicMappings(b.packageRoot, destinationDir)
	if err != nil {
		return "", fmt.Errorf("adding dynamic mappings: %w", err)
	}

	if b.createZip {
		return b.buildZippedPackage(destinationDir)
	}

	if b.skipValidation {
		b.logger.Debug("Skip validation of the built package")
		return destinationDir, nil
	}

	b.logger.Debug("Validating built package", slog.String("path", destinationDir))
	errs, skipped := validation.ValidateAndFilterFromPath(destinationDir)
	if skipped != nil {
		b.logger.Info("Skipped errors", slog.Any("skipped.errors", skipped))
	}
	if errs != nil {
		return "", fmt.Errorf("invalid content found in built package: %w", errs)
	}
	return destinationDir, nil
}

func (b *Builder) buildZippedPackage(destinationDir string) (string, error) {
	b.logger.Debug("Build zipped package")
	zippedPackagePath, err := buildPackagesZipPath(b.packageRoot)
	if err != nil {
		return "", fmt.Errorf("can't evaluate path for the zipped package: %w", err)
	}

	err = files.Zip(destinationDir, zippedPackagePath, b.logger)
	if err != nil {
		return "", fmt.Errorf("can't compress the built package (compressed file path: %s): %w", zippedPackagePath, err)
	}

	if b.skipValidation {
		b.logger.Debug("Skip validation of the built .zip package")
	} else {
		b.logger.Debug("Validating built .zip package)", slog.String("zip.path", zippedPackagePath))
		errs, skipped := validation.ValidateAndFilterFromZip(zippedPackagePath)
		if skipped != nil {
			logger.Info("Skipped errors", slog.Any("skipped.errors", skipped))
		}
		if errs != nil {
			return "", fmt.Errorf("invalid content found in built zip package: %w", errs)
		}
	}

	if b.signPackage {
		err := b.signZippedPackage(zippedPackagePath)
		if err != nil {
			return "", err
		}
	}

	return zippedPackagePath, nil
}

func (b *Builder) signZippedPackage(zippedPackagePath string) error {
	b.logger.Debug("Sign the package")
	m, err := packages.ReadPackageManifestFromPackageRoot(b.packageRoot)
	if err != nil {
		return fmt.Errorf("reading package manifest failed (path: %s): %w", b.packageRoot, err)
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

func (b *Builder) copyLicenseTextFile(licensePath string) error {
	_, err := os.Stat(licensePath)
	if err == nil {
		b.logger.Debug("License file in the package will be used")
		return nil
	}

	repositoryLicenseTextFileName, userDefined := os.LookupEnv(repositoryLicenseEnv)
	if !userDefined {
		repositoryLicenseTextFileName = licenseTextFileName
	}

	sourceLicensePath, err := findRepositoryLicense(repositoryLicenseTextFileName)
	if !userDefined && errors.Is(err, os.ErrNotExist) {
		b.logger.Debug("No license text file is included in package")
		return nil
	}
	if err != nil {
		return fmt.Errorf("failure while looking for license %q in repository: %w", repositoryLicenseTextFileName, err)
	}

	b.logger.Info("License text found, it will be included in package", slog.String("license.path", sourceLicensePath))
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
