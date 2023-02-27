// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package installer

import (
	"fmt"

	"github.com/Masterminds/semver/v3"
	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/kibana"
	"github.com/elastic/elastic-package/internal/packages"
)

var semver8_7_0 = semver.MustParse("8.7.0")

type zipInstaller struct {
	zipPath  string
	manifest packages.PackageManifest

	kibanaClient *kibana.Client
}

// CreateForZip function creates a new instance of the installer.
func CreateForZip(zipPath string, m packages.PackageManifest) (Installer, error) {
	kibanaClient, err := kibana.NewClient()
	if err != nil {
		return nil, errors.Wrap(err, "could not create kibana client")
	}
	kibanaVersion, err := kibanaClient.Version()
	if err != nil {
		return nil, err
	}
	v, err := semver.NewVersion(kibanaVersion.Number)
	if err != nil {
		return nil, fmt.Errorf("invalid Kibana version")
	}
	if v.LessThan(semver8_7_0) {
		return nil, fmt.Errorf("not supported uploading zip packages in Kibana %s", kibanaVersion.Number)
	}

	return &zipInstaller{
		zipPath:      zipPath,
		kibanaClient: kibanaClient,
		manifest:     m,
	}, nil
}

// Install method installs the package using Kibana API.
func (i *zipInstaller) Install() (*InstalledPackage, error) {
	assets, err := i.kibanaClient.InstallZipPackage(i.zipPath)
	if err != nil {
		return nil, errors.Wrap(err, "can't install the package")
	}

	return &InstalledPackage{
		Manifest: i.manifest,
		Assets:   assets,
	}, nil
}

// Uninstall method uninstalls the package using Kibana API.
func (i *zipInstaller) Uninstall() error {
	return errors.Errorf("not implemented")
}
