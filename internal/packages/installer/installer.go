// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package installer

import (
	"path/filepath"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/kibana"
	"github.com/elastic/elastic-package/internal/packages"
)

type Installer struct {
	manifest packages.PackageManifest

	kibanaClient *kibana.Client
}

type InstalledPackage struct {
	Assets   []packages.Asset
	Manifest packages.PackageManifest
}

func CreateForPackage(packageRootPath string) (*Installer, error) {
	kibanaClient, err := kibana.NewClient()
	if err != nil {
		return nil, errors.Wrap(err, "could not create kibana client")
	}

	pkgManifest, err := packages.ReadPackageManifest(filepath.Join(packageRootPath, packages.PackageManifestFile))
	if err != nil {
		return nil, errors.Wrap(err, "reading package manifest failed")
	}

	return &Installer{
		manifest:     *pkgManifest,
		kibanaClient: kibanaClient,
	}, nil
}

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

func (i *Installer) Uninstall() error {
	_, err := i.kibanaClient.RemovePackage(i.manifest)
	if err != nil {
		return errors.Wrap(err, "can't remove the package")
	}
	return nil
}
