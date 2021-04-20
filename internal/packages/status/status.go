// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package status

import (
	"github.com/Masterminds/semver"
	"github.com/pkg/errors"

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
	Staging        []packages.PackageManifest
	Snapshot       []packages.PackageManifest
}

// LocalPackage returns the status of a given package including local development information
func LocalPackage(packageRootPath string, options registry.SearchOptions) (*PackageStatus, error) {
	manifest, err := packages.ReadPackageManifestFromPackageRoot(packageRootPath)
	if err != nil {
		return nil, errors.Wrap(err, "reading package manifest failed")
	}
	changelog, err := changelog.ReadChangelogFromPackageRoot(packageRootPath)
	if err != nil {
		return nil, errors.Wrap(err, "reading package changelog failed")
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
		return nil, errors.Wrap(err, "reading changelog semver failed")
	}
	currentVersion, err := semver.NewVersion(manifest.Version)
	if err != nil {
		return nil, errors.Wrap(err, "reading manifest semver failed")
	}
	if currentVersion.LessThan(pendingVersion) {
		status.PendingChanges = &lastChangelogEntry
	}
	return status, nil
}

// RemotePackage returns the status of a given package
func RemotePackage(packageName string, options registry.SearchOptions) (*PackageStatus, error) {
	snapshotManifests, err := registry.Snapshot.Revisions(packageName, options)
	if err != nil {
		return nil, errors.Wrap(err, "retrieving snapshot deployment failed")
	}
	stagingManifests, err := registry.Staging.Revisions(packageName, options)
	if err != nil {
		return nil, errors.Wrap(err, "retrieving staging deployment failed")
	}
	productionManifests, err := registry.Production.Revisions(packageName, options)
	if err != nil {
		return nil, errors.Wrap(err, "retrieving production deployment failed")
	}
	return &PackageStatus{
		Name:       packageName,
		Snapshot:   snapshotManifests,
		Staging:    stagingManifests,
		Production: productionManifests,
	}, nil
}
