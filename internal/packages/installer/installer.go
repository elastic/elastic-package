// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package installer

import (
	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/kibana"
	"github.com/elastic/elastic-package/internal/packages"
)

// Installer is responsible for installation/uninstallation of the package.
type Installer struct {
	manifest packages.PackageManifest

	kibanaClient *kibana.Client
}

// InstalledPackage represents the installed package (including assets).
type InstalledPackage struct {
	Assets   []packages.Asset
	Manifest packages.PackageManifest
}

// CreateForManifest function creates a new instance of the installer.
func CreateForManifest(manifest packages.PackageManifest) (*Installer, error) {
	kibanaClient, err := kibana.NewClient()
	if err != nil {
		return nil, errors.Wrap(err, "could not create kibana client")
	}

	return &Installer{
		manifest:     manifest,
		kibanaClient: kibanaClient,
	}, nil
}

// Install method installs the package using Kibana API.
func (i *Installer) Install() (*InstalledPackage, error) {
	assets, err := i.kibanaClient.InstallPackage(i.manifest)
	if err != nil {
		return nil, errors.Wrap(err, "can't install the package")
	}

	return &InstalledPackage{
		Manifest: i.manifest,
		Assets:   assets,
	}, nil
}

// Uninstall method uninstalls the package using Kibana API.
func (i *Installer) Uninstall() error {
	_, err := i.kibanaClient.RemovePackage(i.manifest)
	if err != nil {
		return errors.Wrap(err, "can't remove the package")
	}
	return nil
}
