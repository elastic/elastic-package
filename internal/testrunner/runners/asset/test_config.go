// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package asset

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

func newConfig(assetTestFolderPath string) (*testConfig, error) {
	configFilePath := filepath.Join(assetTestFolderPath, "config.yml")

	// Test configuration file is optional for asset loading tests. If it
	// doesn't exist, we can return early.
	if _, err := os.Stat(configFilePath); errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}

	data, err := os.ReadFile(configFilePath)
	if err != nil {
		return nil, fmt.Errorf("could not load asset loading test configuration file: %s: %w", configFilePath, err)
	}

	var c testConfig
	cfg, err := yaml.NewConfig(data, ucfg.PathSep("."))
	if err != nil {
		return nil, fmt.Errorf("unable to load asset loading test configuration file: %s: %w", configFilePath, err)
	}
	if err := cfg.Unpack(&c); err != nil {
		return nil, fmt.Errorf("unable to unpack asset loading test configuration file: %s: %w", configFilePath, err)
	}

	return &c, nil
}
