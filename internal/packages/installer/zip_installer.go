// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package installer

import (
	"context"
	"fmt"

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
func (i *zipInstaller) Install(ctx context.Context) (*InstalledPackage, error) {
	assets, err := i.kibanaClient.InstallZipPackage(ctx, i.zipPath)
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
func (i *zipInstaller) Uninstall(ctx context.Context) error {
	_, err := i.kibanaClient.RemovePackage(ctx, i.manifest.Name, i.manifest.Version, false)
	if err != nil {
		return fmt.Errorf("can't remove the package: %w", err)
	}
	return nil
}

// Manifest method returns the package manifest.
func (i *zipInstaller) Manifest(context.Context) (*packages.PackageManifest, error) {
	return i.manifest, nil
}
