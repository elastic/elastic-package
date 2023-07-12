// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package archetype

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/elastic/elastic-package/internal/formatter"
	"github.com/elastic/elastic-package/internal/licenses"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/packages"
)

// PackageDescriptor defines configurable properties of the package archetype
type PackageDescriptor struct {
	Manifest packages.PackageManifest
}

// CreatePackage function bootstraps the new package based on the provided descriptor.
func CreatePackage(packageDescriptor PackageDescriptor) error {
	baseDir := packageDescriptor.Manifest.Name
	_, err := os.Stat(baseDir)
	if err == nil {
		return fmt.Errorf(`package "%s" already exists`, baseDir)
	}

	logger.Debugf("Write package manifest")
	err = renderResourceFile(packageManifestTemplate, &packageDescriptor, filepath.Join(baseDir, "manifest.yml"))
	if err != nil {
		return fmt.Errorf("can't render package manifest: %w", err)
	}

	logger.Debugf("Write package changelog")
	err = renderResourceFile(packageChangelogTemplate, &packageDescriptor, filepath.Join(baseDir, "changelog.yml"))
	if err != nil {
		return fmt.Errorf("can't render package changelog: %w", err)
	}

	logger.Debugf("Write docs readme")
	err = renderResourceFile(packageDocsReadme, &packageDescriptor, filepath.Join(baseDir, "docs", "README.md"))
	if err != nil {
		return fmt.Errorf("can't render package README: %w", err)
	}

	if license := packageDescriptor.Manifest.Source.License; license != "" {
		logger.Debugf("Write license file")
		err = licenses.WriteTextToFile(license, filepath.Join(baseDir, "LICENSE.txt"))
		if err != nil {
			return fmt.Errorf("can't write license file: %w", err)
		}
	}

	logger.Debugf("Write sample icon")
	err = writeRawResourceFile(packageImgSampleIcon, filepath.Join(baseDir, "img", "sample-logo.svg"))
	if err != nil {
		return fmt.Errorf("can't render sample icon: %w", err)
	}

	logger.Debugf("Write sample screenshot")
	decodedSampleScreenshot, err := decodeBase64Resource(packageImgSampleScreenshot)
	if err != nil {
		return fmt.Errorf("can't decode sample screenshot: %w", err)
	}
	err = writeRawResourceFile(decodedSampleScreenshot, filepath.Join(baseDir, "img", "sample-screenshot.png"))
	if err != nil {
		return fmt.Errorf("can't render sample screenshot: %w", err)
	}

	logger.Debugf("Format the entire package")
	err = formatter.Format(baseDir, false)
	if err != nil {
		return fmt.Errorf("can't format the new package: %w", err)
	}

	fmt.Printf("New package has been created: %s\n", baseDir)
	return nil
}
