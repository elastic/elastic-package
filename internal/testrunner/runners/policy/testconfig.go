// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package policy

import (
	"fmt"
	"os"

	"github.com/elastic/go-ucfg"
	"github.com/elastic/go-ucfg/yaml"

	"github.com/elastic/elastic-package/internal/testrunner"
)

type testConfig struct {
	testrunner.SkippableConfig `config:",inline"`

	Input      string         `config:"input,omitempty" yaml:"input,omitempty"`
	Vars       map[string]any `config:"vars,omitempty" yaml:"vars,omitempty"`
	DataStream struct {
		Vars map[string]any `config:"vars,omitempty" yaml:"vars,omitempty"`
	} `config:"data_stream" yaml:"data_stream"`
}

func readTestConfig(testPath string) (*testConfig, error) {
	d, err := os.ReadFile(testPath)
	if err != nil {
		return nil, err
	}

	var config testConfig
	cfg, err := yaml.NewConfig(d, ucfg.PathSep("."))
	if err != nil {
		return nil, fmt.Errorf("unable to load system test configuration file: %s: %w", testPath, err)
	}
	if err := cfg.Unpack(&config); err != nil {
		return nil, fmt.Errorf("unable to unpack system test configuration file: %s: %w", testPath, err)
	}

	return &config, nil
}
