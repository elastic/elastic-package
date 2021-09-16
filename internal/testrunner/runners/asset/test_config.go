// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package asset

import (
	"os"
	"path/filepath"

	"github.com/elastic/go-ucfg"
	"github.com/elastic/go-ucfg/yaml"
	"github.com/pkg/errors"

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
		return nil, errors.Wrapf(err, "could not load asset loading test configuration file: %s", configFilePath)
	}

	var c testConfig
	cfg, err := yaml.NewConfig(data, ucfg.PathSep("."))
	if err != nil {
		return nil, errors.Wrapf(err, "unable to load asset loading test configuration file: %s", configFilePath)
	}
	if err := cfg.Unpack(&c); err != nil {
		return nil, errors.Wrapf(err, "unable to unpack asset loading test configuration file: %s", configFilePath)
	}

	return &c, nil
}
