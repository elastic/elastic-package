// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package buildmanifest

import (
	"os"
	"path/filepath"

	"github.com/elastic/go-ucfg"
	"github.com/elastic/go-ucfg/yaml"
	"github.com/pkg/errors"
)

// BuildManifest defines the manifest defining the building procedure.
type BuildManifest struct {
	Dependencies Dependencies `config:"dependencies"`
}

// Dependencies define external package dependencies.
type Dependencies struct {
	ECS ECSDependency `config:"ecs"`
}

// ECSDependency defines a dependency on ECS fields.
type ECSDependency struct {
	Reference string `config:"reference"`
}

// HasDependencies function checks if there are any dependencies defined.
func (bm *BuildManifest) HasDependencies() bool {
	return bm.Dependencies.ECS.Reference != ""
}

// ReadBuildManifest function reads the package build manifest.
func ReadBuildManifest(packageRoot string) (*BuildManifest, bool, error) {
	path := buildManifestPath(packageRoot)
	cfg, err := yaml.NewConfigWithFile(path, ucfg.PathSep("."))
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil // ignore not found errors
	}
	if err != nil {
		return nil, false, errors.Wrapf(err, "reading file failed (path: %s)", path)
	}

	var bm BuildManifest
	err = cfg.Unpack(&bm)
	if err != nil {
		return nil, true, errors.Wrapf(err, "unpacking build manifest failed (path: %s)", path)
	}
	return &bm, true, nil
}

func buildManifestPath(packageRoot string) string {
	return filepath.Join(packageRoot, "_dev", "build", "build.yml")
}
