// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package externalfields

import (
	"os"
	"path/filepath"

	"github.com/elastic/go-ucfg"
	"github.com/elastic/go-ucfg/yaml"
	"github.com/pkg/errors"
)

type buildManifest struct {
	Dependencies dependencies `config:"dependencies"`
}

type dependencies struct {
	ECS ecsDependency `config:"ecs"`
}

type ecsDependency struct {
	Reference string `config:"reference"`
}

func (bm *buildManifest) hasDependencies() bool {
	return bm.Dependencies.ECS.Reference != ""
}

func readBuildManifest(packageRoot string) (*buildManifest, bool, error) {
	path := filepath.Join(packageRoot, "_dev", "build", "build.yml")
	cfg, err := yaml.NewConfigWithFile(path, ucfg.PathSep("."))
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil // ignore not found errors
	}
	if err != nil {
		return nil, false, errors.Wrapf(err, "reading file failed (path: %s)", path)
	}

	var bm buildManifest
	err = cfg.Unpack(&bm)
	if err != nil {
		return nil, true, errors.Wrapf(err, "unpacking build manifest failed (path: %s)", path)
	}
	return &bm, true, nil
}
