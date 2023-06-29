// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package status

import (
	"fmt"

	"github.com/Masterminds/semver/v3"

	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/packages/changelog"
	"github.com/elastic/elastic-package/internal/registry"
)

// PackageStatus holds version and deployment information about a package
type PackageStatus struct {
	Name           string
	Changelog      []changelog.Revision
	PendingChanges *changelog.Revision
	Local          *packages.PackageManifest
	Production     []packages.PackageManifest
}

// LocalPackage returns the status of a given package including local development information
func LocalPackage(packageRootPath string, options registry.SearchOptions) (*PackageStatus, error) {
	manifest, err := packages.ReadPackageManifestFromPackageRoot(packageRootPath)
	if err != nil {
		return nil, fmt.Errorf("reading package manifest failed: %w", err)
	}
	changelog, err := changelog.ReadChangelogFromPackageRoot(packageRootPath)
	if err != nil {
		return nil, fmt.Errorf("reading package changelog failed: %w", err)
	}
	status, err := RemotePackage(manifest.Name, options)
	if err != nil {
		return nil, err
	}
	status.Changelog = changelog
	status.Local = manifest

	if len(changelog) == 0 {
		return status, nil
	}
	lastChangelogEntry := changelog[0]
	pendingVersion, err := semver.NewVersion(lastChangelogEntry.Version)
	if err != nil {
		return nil, fmt.Errorf("reading changelog semver failed: %w", err)
	}
	currentVersion, err := semver.NewVersion(manifest.Version)
	if err != nil {
		return nil, fmt.Errorf("reading manifest semver failed: %w", err)
	}
	if currentVersion.LessThan(pendingVersion) {
		status.PendingChanges = &lastChangelogEntry
	}
	return status, nil
}

// RemotePackage returns the status of a given package
func RemotePackage(packageName string, options registry.SearchOptions) (*PackageStatus, error) {
	productionManifests, err := registry.Production.Revisions(packageName, options)
	if err != nil {
		return nil, fmt.Errorf("retrieving production deployment failed: %w", err)
	}
	return &PackageStatus{
		Name:       packageName,
		Production: productionManifests,
	}, nil
}
