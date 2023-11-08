// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package installer

import (
	"fmt"

	"github.com/elastic/elastic-package/internal/kibana"
	"github.com/elastic/elastic-package/pkg/packages"
)

type manifestInstaller struct {
	manifest     *packages.PackageManifest
	kibanaClient *kibana.Client
}

// InstalledPackage represents the installed package (including assets).
type InstalledPackage struct {
	Assets  []packages.Asset
	Name    string
	Version string
}

// CreateForManifest function creates a new instance of the installer.
func CreateForManifest(kibanaClient *kibana.Client, packageRoot string) (*manifestInstaller, error) {
	manifest, err := packages.ReadPackageManifestFromPackageRoot(packageRoot)
	if err != nil {
		return nil, fmt.Errorf("could not read manifest: %w", err)
	}
	return &manifestInstaller{
		manifest:     manifest,
		kibanaClient: kibanaClient,
	}, nil
}

// Install method installs the package using Kibana API.
func (i *manifestInstaller) Install() (*InstalledPackage, error) {
	assets, err := i.kibanaClient.InstallPackage(i.manifest.Name, i.manifest.Version)
	if err != nil {
		return nil, fmt.Errorf("can't install the package: %w", err)
	}

	return &InstalledPackage{
		Name:    i.manifest.Name,
		Version: i.manifest.Version,
		Assets:  assets,
	}, nil
}

// Uninstall method uninstalls the package using Kibana API.
func (i *manifestInstaller) Uninstall() error {
	_, err := i.kibanaClient.RemovePackage(i.manifest.Name, i.manifest.Version)
	if err != nil {
		return fmt.Errorf("can't remove the package: %w", err)
	}
	return nil
}

// Manifest method returns the package manifest.
func (i *manifestInstaller) Manifest() (*packages.PackageManifest, error) {
	return i.manifest, nil
}
