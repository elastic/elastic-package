// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package pipeline

import (
	"os"
	"path/filepath"

	"github.com/elastic/go-ucfg/yaml"
	"github.com/pkg/errors"
)

const (
	configYAML = "config.yml"
)

type config struct {
	NumDocs int `config:"num_docs"`
}

func defaultConfig() *config {
	return &config{
		NumDocs: 1000,
	}
}

func readConfig(path string) (*config, error) {
	configPath := filepath.Join(path, configYAML)
	c := defaultConfig()
	cfg, err := yaml.NewConfigWithFile(configPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, errors.Wrapf(err, "can't load common configuration: %s", configPath)
	}

	if err == nil {
		if err := cfg.Unpack(c); err != nil {
			return nil, errors.Wrapf(err, "can't unpack benchmark configuration: %s", configPath)
		}
	}

	return c, nil
}
