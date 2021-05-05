// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package archetype

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/formatter"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/packages"
)

// DataStreamDescriptor defines configurable properties of the data stream archetype
type DataStreamDescriptor struct {
	Manifest    packages.DataStreamManifest
	PackageRoot string
}

// CreateDataStream function bootstraps the new data stream based on the provided descriptor.
func CreateDataStream(dataStreamDescriptor DataStreamDescriptor) error {
	dataStreamDir := filepath.Join(dataStreamDescriptor.PackageRoot, "data_stream", dataStreamDescriptor.Manifest.Name)
	_, err := os.Stat(dataStreamDir)
	if err == nil {
		return fmt.Errorf(`data stream "%s" already exists`, dataStreamDescriptor.Manifest.Name)
	}

	logger.Debugf("Write data stream manifest")
	err = renderResourceFile(dataStreamManifestTemplate, &dataStreamDescriptor, filepath.Join(dataStreamDir, "manifest.yml"))
	if err != nil {
		return errors.Wrap(err, "can't render data stream manifest")
	}

	logger.Debugf("Write base fields")
	err = renderResourceFile(dataStreamFieldsBaseTemplate, &dataStreamDescriptor, filepath.Join(dataStreamDir, "fields", "base-fields.yml"))
	if err != nil {
		return errors.Wrap(err, "can't render base fields")
	}

	logger.Debugf("Write agent stream")
	err = renderResourceFile(dataStreamAgentStreamTemplate, &dataStreamDescriptor, filepath.Join(dataStreamDir, "agent", "stream", "stream.yml.hbs"))
	if err != nil {
		return errors.Wrap(err, "can't render base fields")
	}

	if dataStreamDescriptor.Manifest.Type == "logs" {
		logger.Debugf("Write ingest pipeline")
		err = renderResourceFile(dataStreamElasticsearchIngestPipelineTemplate, &dataStreamDescriptor, filepath.Join(dataStreamDir, "elasticsearch", "ingest_pipeline", "default.yml"))
		if err != nil {
			return errors.Wrap(err, "can't render ingest pipeline")
		}
	}

	logger.Debugf("Format the entire package")
	err = formatter.Format(dataStreamDescriptor.PackageRoot, false)
	if err != nil {
		return errors.Wrap(err, "can't format the new package")
	}

	fmt.Printf("New data stream has been created: %s\n", dataStreamDescriptor.Manifest.Name)
	return nil
}
