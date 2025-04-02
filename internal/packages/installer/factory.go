// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package installer

import (
	"context"
	"errors"
	"fmt"

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
	Kibana *kibana.Client

	StackSubscription string

	RootPath       string
	ZipPath        string
	SkipValidation bool
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
	if options.RootPath == "" && options.ZipPath == "" {
		return nil, errors.New("missing package root path or pre-built zip package")
	}
	if options.StackSubscription == "" {
		return nil, errors.New("missing stack subscription")
	}

	version, err := kibanaVersion(options.Kibana)
	if err != nil {
		return nil, fmt.Errorf("failed to get kibana version: %w", err)
	}

	logger.Debugf("Subscription stack: %s", options.StackSubscription)
	supportsUploadZip := supportedUploadZip(options.StackSubscription, version)
	if options.ZipPath != "" {
		if !supportsUploadZip {
			if version.LessThan(semver8_7_0) {
				return nil, fmt.Errorf("not supported uploading zip packages in Kibana %s (%s required)", version, semver8_7_0)
			}
			if version.LessThan(semver8_8_2) {
				return nil, fmt.Errorf("not supported uploading zip packages in Kibana %s using subscription %s (%s required)", version, options.StackSubscription, semver8_8_2)
			}
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
		PackageRoot:    options.RootPath,
		CreateZip:      supportsUploadZip,
		SignPackage:    false,
		SkipValidation: options.SkipValidation,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to build package: %v", err)
	}

	if supportsUploadZip {
		return CreateForZip(options.Kibana, target)
	}
	return CreateForManifest(options.Kibana, target)
}

func supportedUploadZip(pkgSubscription string, kibanaVersion *semver.Version) bool {
	if kibanaVersion.LessThan(semver8_7_0) {
		return false
	}

	if kibanaVersion.LessThan(semver8_8_2) && pkgSubscription == "basic" {
		return false
	}

	return true
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
