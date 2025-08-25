// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package archetype

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

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

	logger.Debugf("Populate input variables")
	err = populateInputs(&dataStreamDescriptor)
	if err != nil {
		return fmt.Errorf("can't populate input variables: %w", err)
	}

	logger.Debugf("Write data stream manifest")
	err = renderResourceFile(dataStreamManifestTemplate, &dataStreamDescriptor, filepath.Join(dataStreamDir, "manifest.yml"))
	if err != nil {
		return fmt.Errorf("can't render data stream manifest: %w", err)
	}

	logger.Debugf("Write base fields")
	err = renderResourceFile(fieldsBaseTemplate, &dataStreamDescriptor, filepath.Join(dataStreamDir, "fields", "base-fields.yml"))
	if err != nil {
		return fmt.Errorf("can't render base fields: %w", err)
	}

	logger.Debugf("Write agent stream")
	if dataStreamDescriptor.Manifest.Type == "logs" {

		if len(dataStreamDescriptor.Manifest.Streams) == 0 {
			err = renderResourceFile(dataStreamAgentStreamTemplate, &dataStreamDescriptor, filepath.Join(dataStreamDir, "agent", "stream", "stream.yml.hbs"))
			if err != nil {
				return fmt.Errorf("can't render agent stream: %w", err)
			}
		}

		for _, stream := range dataStreamDescriptor.Manifest.Streams {
			agentTemplate, err := loadRawAgentTemplate(stream.Input)
			if err != nil {
				return fmt.Errorf("can't find agent definition for input %q: %w", stream.Input, err)
			}

			fileName := fmt.Sprintf("%s.yml.hbs", strings.ReplaceAll(stream.Input, "-", "_"))
			err = writeRawResourceFile([]byte(agentTemplate), filepath.Join(dataStreamDir, "agent", "stream", fileName))
			if err != nil {
				return fmt.Errorf("can't write agent stream file for input %q: %w", stream.Input, err)
			}
		}
	} else {
		err = renderResourceFile(dataStreamAgentStreamTemplate, &dataStreamDescriptor, filepath.Join(dataStreamDir, "agent", "stream", "stream.yml.hbs"))
		if err != nil {
			return fmt.Errorf("can't render agent stream: %w", err)
		}

	}

	if dataStreamDescriptor.Manifest.Type == "logs" {
		logger.Debugf("Write ingest pipeline")
		err = renderResourceFile(dataStreamElasticsearchIngestPipelineTemplate, &dataStreamDescriptor, filepath.Join(dataStreamDir, "elasticsearch", "ingest_pipeline", "default.yml"))
		if err != nil {
			return fmt.Errorf("can't render ingest pipeline: %w", err)
		}
	}

	logger.Debugf("Format the entire package")
	err = formatter.Format(dataStreamDescriptor.PackageRoot, false)
	if err != nil {
		return fmt.Errorf("can't format the new data stream: %w", err)
	}

	fmt.Printf("New data stream has been created: %s\n", dataStreamDescriptor.Manifest.Name)
	return nil
}
