package installer

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"

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
func CreateForZip(zipPath string) (Installer, error) {
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
		return nil, fmt.Errorf("invalid kibana version")
	}
	if v.LessThan(semver8_7_0) {
		return nil, fmt.Errorf("not supported uploading zip packages in %s", kibanaVersion.Number)
	}

	tempDir, err := os.MkdirTemp("", "elastic-package-")
	if err != nil {
		return nil, errors.Wrap(err, "can't prepare a temporary directory")
	}
	defer os.RemoveAll(tempDir)

	err = uncompressManifestZipPackage(zipPath, tempDir)
	if err != nil {
		return nil, errors.Wrap(err, "extracting manifest from zip failed")
	}

	m, err := packages.ReadPackageManifestFromPackageRoot(tempDir)
	if err != nil {
		return nil, errors.Wrapf(err, "reading package manifest from zip failed (path: %s)", zipPath)
	}

	return &zipInstaller{
		zipPath:      zipPath,
		kibanaClient: kibanaClient,
		manifest:     *m,
	}, nil
}

func uncompressManifestZipPackage(zipPath, target string) error {
	zipReader, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer zipReader.Close()
	targetPath := filepath.Join(target, packages.PackageManifestFile)
	outputFile, err := os.OpenFile(
		targetPath,
		os.O_WRONLY|os.O_CREATE|os.O_TRUNC,
		0644,
	)
	if err != nil {
		return err
	}
	defer outputFile.Close()

	for _, f := range zipReader.File {
		if f.Name != packages.PackageManifestFile {
			continue
		}
		zippedFile, err := f.Open()
		if err != nil {
			return err
		}
		defer zippedFile.Close()

		_, err = io.Copy(outputFile, zippedFile)
		if err != nil {
			return err
		}
		return nil
	}
	return errors.Errorf("not found package %s in %s", packages.PackageManifestFile, zipPath)
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
