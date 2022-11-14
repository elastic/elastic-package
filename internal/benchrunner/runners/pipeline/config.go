// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package pipeline

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/elastic/go-ucfg/yaml"
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
		return nil, fmt.Errorf("can't load common configuration: %s: %w", configPath, err)
	}

	if err == nil {
		if err := cfg.Unpack(c); err != nil {
			return nil, fmt.Errorf("can't unpack benchmark configuration: %s: %w", configPath, err)
		}
	}

	return c, nil
}
