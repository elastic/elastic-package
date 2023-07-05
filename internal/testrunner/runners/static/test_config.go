// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package static

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/elastic/go-ucfg"
	"github.com/elastic/go-ucfg/yaml"

	"github.com/elastic/elastic-package/internal/testrunner"
)

type testConfig struct {
	testrunner.SkippableConfig `config:",inline"`
}

func newConfig(staticTestFolderPath string) (*testConfig, error) {
	configFilePath := filepath.Join(staticTestFolderPath, "config.yml")

	// Test configuration file is optional for static loading tests. If it
	// doesn't exist, we can return early.
	if _, err := os.Stat(configFilePath); errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}

	data, err := os.ReadFile(configFilePath)
	if err != nil {
		return nil, fmt.Errorf("could not load static test configuration file: %s: %w", configFilePath, err)
	}

	cfg, err := yaml.NewConfig(data, ucfg.PathSep("."))
	if err != nil {
		return nil, fmt.Errorf("unable to load static test configuration file: %s: %w", configFilePath, err)
	}

	var c testConfig
	if err := cfg.Unpack(&c); err != nil {
		return nil, fmt.Errorf("unable to unpack static test configuration file: %s: %w", configFilePath, err)
	}

	return &c, nil
}
