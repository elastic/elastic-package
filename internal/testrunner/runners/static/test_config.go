// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package static

import (
	"io/ioutil"
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

func newConfig(staticTestFolderPath string) (*testConfig, error) {
	configFilePath := filepath.Join(staticTestFolderPath, "config.yml")

	// Test configuration file is optional for static loading tests. If it
	// doesn't exist, we can return early.
	if _, err := os.Stat(configFilePath); os.IsNotExist(err) {
		return nil, nil
	}

	data, err := ioutil.ReadFile(configFilePath)
	if err != nil {
		return nil, errors.Wrapf(err, "could not load static loading test configuration file: %s", configFilePath)
	}

	var c testConfig
	cfg, err := yaml.NewConfig(data, ucfg.PathSep("."))
	if err != nil {
		return nil, errors.Wrapf(err, "unable to load static loading test configuration file: %s", configFilePath)
	}
	if err := cfg.Unpack(&c); err != nil {
		return nil, errors.Wrapf(err, "unable to unpack static loading test configuration file: %s", configFilePath)
	}

	return &c, nil
}
