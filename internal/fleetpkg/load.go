// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package fleetpkg

import (
	"path/filepath"

	"github.com/elastic/elastic-package/internal/yamledit"
)

// Load will load a package from the given directory.
func Load(dir string) (*Package, error) {
	pkg := Package{
		sourceDir: dir,
	}

	// -------------------------------------------------------------------------
	// manifest.yml

	if _, err := yamledit.ParseDocumentFile(filepath.Join(dir, "manifest.yml"), &pkg.Manifest); err != nil {
		return nil, err
	}

	// -------------------------------------------------------------------------
	// Data Streams

	var dataStreamManifests []string
	if pkg.Manifest.Type == "input" {
		dataStreamManifests = []string{filepath.Join(dir, "manifest.yml")}
	} else {
		var err error
		pkg.DataStreams = map[string]*DataStream{}
		if dataStreamManifests, err = filepath.Glob(filepath.Join(dir, "data_stream/*/manifest.yml")); err != nil {
			return nil, err
		}
	}
	for _, manifestPath := range dataStreamManifests {
		ds := &DataStream{
			sourceDir: filepath.Dir(manifestPath),
		}
		if pkg.Manifest.Type == "input" {
			pkg.Input = ds
		} else {
			pkg.DataStreams[filepath.Base(ds.sourceDir)] = ds

			if _, err := yamledit.ParseDocumentFile(manifestPath, &ds.Manifest); err != nil {
				return nil, err
			}

			// -----------------------------------------------------------------
			// Pipelines

			pipelines, err := filepath.Glob(filepath.Join(ds.sourceDir, "elasticsearch/ingest_pipeline/*.yml"))
			if err != nil {
				return nil, err
			}

			if len(pipelines) > 0 {
				ds.Pipelines = map[string]*Pipeline{}
			}
			for _, pipelinePath := range pipelines {
				var pipeline Pipeline
				if _, err = yamledit.ParseDocumentFile(pipelinePath, &pipeline); err != nil {
					return nil, err
				}
				ds.Pipelines[filepath.Base(pipelinePath)] = &pipeline
			}
		}
	}

	return &pkg, nil
}
