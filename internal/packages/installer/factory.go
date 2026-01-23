// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package installer

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/Masterminds/semver/v3"

	"github.com/elastic/elastic-package/internal/builder"
	"github.com/elastic/elastic-package/internal/kibana"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/validation"
)

var (
	semver8_7_0 = semver.MustParse("8.7.0")
	semver8_8_2 = semver.MustParse("8.8.2")
)

// Installer is responsible for installation/uninstallation of the package.
type Installer interface {
	Install(context.Context) (*InstalledPackage, error)
	Uninstall(context.Context) error

	Manifest(context.Context) (*packages.PackageManifest, error)
}

// Options are the parameters used to build an installer.
type Options struct {
	Kibana          *kibana.Client
	WorkDir         string
	PackageRootPath string // Root path of the package to be installed.
	ZipPath         string
	SkipValidation  bool
	RepositoryRoot  *os.Root // Root of the repository where package source code is located.
}

// NewForPackage creates a new installer for a package, given its root path, or its prebuilt zip.
// If a zip path is given, this package is validated and installed as is. This fails on versions
// of Kibana lower than 8.7.0.
// When no zip is given, package is built as zip and installed if version is at least 8.7.0,
// or from the package registry otherwise.
func NewForPackage(options Options) (Installer, error) {
	if options.Kibana == nil {
		return nil, errors.New("missing kibana client")
	}
	if options.PackageRootPath == "" && options.ZipPath == "" {
		return nil, errors.New("missing package root path or pre-built zip package")
	}
	if options.RepositoryRoot == nil {
		return nil, errors.New("missing repository root")
	}

	version, err := kibanaVersion(options.Kibana)
	if err != nil {
		return nil, fmt.Errorf("failed to get kibana version: %w", err)
	}

	supportsUploadZip, reason, err := isAllowedInstallationViaApi(context.TODO(), options.Kibana, version)
	if err != nil {
		return nil, fmt.Errorf("failed to validate whether or not it can be used upload API: %w", err)
	}
	if options.ZipPath != "" {
		if !supportsUploadZip {
			return nil, errors.New(reason)
		}

		if !options.SkipValidation {
			logger.Debugf("Validating built .zip package (path: %s)", options.ZipPath)
			errs, skipped := validation.ValidateAndFilterFromZip(options.ZipPath)
			if skipped != nil {
				logger.Infof("Skipped errors: %v", skipped)
			}
			if errs != nil {
				return nil, fmt.Errorf("invalid content found in built zip package: %w", errs)
			}
		}
		logger.Debug("Skip validation of the built .zip package")
		return CreateForZip(options.Kibana, options.ZipPath)
	}

	target, err := builder.BuildPackage(builder.BuildOptions{
		WorkDir:         options.WorkDir,
		PackageRootPath: options.PackageRootPath,
		CreateZip:       supportsUploadZip,
		SignPackage:     false,
		SkipValidation:  options.SkipValidation,
		RepositoryRoot:  options.RepositoryRoot,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to build package: %v", err)
	}

	if supportsUploadZip {
		return CreateForZip(options.Kibana, target)
	}
	return CreateForManifest(options.Kibana, target)
}

func isAllowedInstallationViaApi(ctx context.Context, kbnClient *kibana.Client, kibanaVersion *semver.Version) (bool, string, error) {
	reason := ""
	if kibanaVersion.LessThan(semver8_7_0) {
		reason = fmt.Sprintf("not supported uploading zip packages in Kibana %s (%s required)", kibanaVersion, semver8_7_0)
		return false, reason, nil
	}

	if kibanaVersion.LessThan(semver8_8_2) {
		err := kbnClient.EnsureZipPackageCanBeInstalled(ctx)
		if errors.Is(err, kibana.ErrNotSupported) {
			reason = fmt.Sprintf("not supported uploading zip packages in Kibana %s (%s required or Enteprise license)", kibanaVersion, semver8_8_2)
			return false, reason, nil
		}
		if err != nil {
			return false, "", err
		}
	}

	return true, "", nil
}

func kibanaVersion(kibana *kibana.Client) (*semver.Version, error) {
	version, err := kibana.Version()
	if err != nil {
		return nil, err
	}
	sv, err := semver.NewVersion(version.Number)
	if err != nil {
		return nil, fmt.Errorf("invalid kibana version: %w", err)
	}
	return sv, nil
}
