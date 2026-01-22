// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package testrunner

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/elastic/go-ucfg"
	"github.com/elastic/go-ucfg/yaml"
)

type globalTestConfig struct {
	Asset    GlobalRunnerTestConfig `config:"asset"`
	Pipeline GlobalRunnerTestConfig `config:"pipeline"`
	Policy   GlobalRunnerTestConfig `config:"policy"`
	Static   GlobalRunnerTestConfig `config:"static"`
	System   GlobalRunnerTestConfig `config:"system"`
}

type GlobalRunnerTestConfig struct {
	Parallel        bool `config:"parallel"`
	SkippableConfig `config:",inline"`
}

func ReadGlobalTestConfig(packageRoot string) (*globalTestConfig, error) {
	configFilePath := filepath.Join(packageRoot, "_dev", "test", "config.yml")

	data, err := os.ReadFile(configFilePath)
	if errors.Is(err, os.ErrNotExist) {
		return &globalTestConfig{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", configFilePath, err)
	}

	var c globalTestConfig
	cfg, err := yaml.NewConfig(data, ucfg.PathSep("."))
	if err != nil {
		return nil, fmt.Errorf("unable to load global test configuration file: %s: %w", configFilePath, err)
	}
	if err := cfg.Unpack(&c); err != nil {
		return nil, fmt.Errorf("unable to unpack global test configuration file: %s: %w", configFilePath, err)
	}

	return &c, nil
}
