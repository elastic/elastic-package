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
type Installer interface {
	Install() (*InstalledPackage, error)
	Uninstall() error
}

type manifestInstaller struct {
	name    string
	version string

	kibanaClient *kibana.Client
}

// InstalledPackage represents the installed package (including assets).
type InstalledPackage struct {
	Assets  []packages.Asset
	Name    string
	Version string
}

// CreateForManifest function creates a new instance of the installer.
func CreateForManifest(name, version string) (*manifestInstaller, error) {
	kibanaClient, err := kibana.NewClient()
	if err != nil {
		return nil, errors.Wrap(err, "could not create kibana client")
	}

	return &manifestInstaller{
		name:         name,
		version:      version,
		kibanaClient: kibanaClient,
	}, nil
}

// Install method installs the package using Kibana API.
func (i *manifestInstaller) Install() (*InstalledPackage, error) {
	assets, err := i.kibanaClient.InstallPackage(i.name, i.version)
	if err != nil {
		return nil, errors.Wrap(err, "can't install the package")
	}

	return &InstalledPackage{
		Name:    i.name,
		Version: i.version,
		Assets:  assets,
	}, nil
}

// Uninstall method uninstalls the package using Kibana API.
func (i *manifestInstaller) Uninstall() error {
	_, err := i.kibanaClient.RemovePackage(i.name, i.version)
	if err != nil {
		return errors.Wrap(err, "can't remove the package")
	}
	return nil
}
