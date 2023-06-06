// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package installer

import (
	"fmt"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/kibana"
	"github.com/elastic/elastic-package/internal/packages"
)

type zipInstaller struct {
	zipPath  string
	manifest *packages.PackageManifest

	kibanaClient *kibana.Client
}

// CreateForZip function creates a new instance of the installer.
func CreateForZip(kibanaClient *kibana.Client, zipPath string) (*zipInstaller, error) {
	manifest, err := packages.ReadPackageManifestFromZipPackage(zipPath)
	if err != nil {
		return nil, fmt.Errorf("could not read manifest: %w", err)
	}
	return &zipInstaller{
		zipPath:      zipPath,
		kibanaClient: kibanaClient,
		manifest:     manifest,
	}, nil
}

// Install method installs the package using Kibana API.
func (i *zipInstaller) Install() (*InstalledPackage, error) {
	assets, err := i.kibanaClient.InstallZipPackage(i.zipPath)
	if err != nil {
		return nil, errors.Wrap(err, "can't install the package")
	}

	return &InstalledPackage{
		Name:    i.manifest.Name,
		Version: i.manifest.Version,
		Assets:  assets,
	}, nil
}

// Uninstall method uninstalls the package using Kibana API.
func (i *zipInstaller) Uninstall() error {
	_, err := i.kibanaClient.RemovePackage(i.manifest.Name, i.manifest.Version)
	if err != nil {
		return errors.Wrap(err, "can't remove the package")
	}
	return nil
}

// Manifest method returns the package manifest.
func (i *zipInstaller) Manifest() (*packages.PackageManifest, error) {
	return i.manifest, nil
}
