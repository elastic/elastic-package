// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package archetype

import (
	"bytes"
	"fmt"
	"html/template"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/packages"
)

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

	// Write package manifest
	err = renderResourceFile(packageManifestTemplate, &packageDescriptor, filepath.Join(baseDir, "manifest.yml"))
	if err != nil {
		return errors.Wrap(err, "can't render package manifest")
	}

	// Write package changelog
	err = renderResourceFile(packageChangelogTemplate, &packageDescriptor, filepath.Join(baseDir, "changelog.yml"))
	if err != nil {
		return errors.Wrap(err, "can't render package changelog")
	}

	// Write docs readme
	err = renderResourceFile(packageDocsReadme, &packageDescriptor, filepath.Join(baseDir, "docs", "README.md"))
	if err != nil {
		return errors.Wrap(err, "can't render package README")
	}

	// Write sample icon
	err = renderResourceFile(packageImgSampleIcon, &packageDescriptor, filepath.Join(baseDir, "img", "sample-logo.svg"))
	if err != nil {
		return errors.Wrap(err, "can't render sample icon")
	}

	fmt.Printf("New package has been created: %s\n", baseDir)
	return nil
}

func renderResourceFile(templateBody string, data interface{}, targetPath string) error {
	t := template.Must(template.New("template").Parse(templateBody))
	var rendered bytes.Buffer
	err := t.Execute(&rendered, data)
	if err != nil {
		return errors.Wrap(err, "can't render package resource")
	}

	err = os.MkdirAll(filepath.Dir(targetPath), 0755)
	if err != nil {
		return errors.Wrap(err, "can't create base directory")
	}

	packageManifestPath := targetPath
	err = ioutil.WriteFile(packageManifestPath, rendered.Bytes(), 0644)
	if err != nil {
		return errors.Wrapf(err, "can't write resource file (path: %s)", packageManifestPath)
	}
	return nil
}
