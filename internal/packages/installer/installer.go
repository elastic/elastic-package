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

type InstalledPackage struct {
	Manifest packages.PackageManifest
	Assets   []packages.Asset

	kibanaClient *kibana.Client
}

func Install(packageRootPath string) (*InstalledPackage, error) {
	pkgManifest, err := packages.ReadPackageManifest(filepath.Join(packageRootPath, packages.PackageManifestFile))
	if err != nil {
		return nil, errors.Wrap(err, "reading package manifest failed")
	}

	kibanaClient, err := kibana.NewClient()
	if err != nil {
		return nil, errors.Wrap(err, "could not create kibana client")
	}

	assets, err := kibanaClient.InstallPackage(*pkgManifest)
	if err != nil {
		return nil, errors.Wrap(err, "could not install package")
	}

	return &InstalledPackage{
		Assets:       assets,
		Manifest:     *pkgManifest,
		kibanaClient: kibanaClient,
	}, nil
}

func (p *InstalledPackage) Uninstall() error {
	_, err := p.kibanaClient.RemovePackage(p.Manifest)
	if err != nil {
		return errors.Wrap(err, "can't remove the package")
	}
	return nil
}
