// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package policy

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type testConfig struct {
	Input      string         `yaml:"input,omitempty"`
	Vars       map[string]any `yaml:"vars,omitempty"`
	DataStream struct {
		Vars map[string]any `yaml:"vars,omitempty"`
	} `yaml:"data_stream"`
}

func readTestConfig(testPath string) (*testConfig, error) {
	d, err := os.ReadFile(testPath)
	if err != nil {
		return nil, err
	}

	var config testConfig
	err = yaml.Unmarshal(d, &config)
	if err != nil {
		return nil, fmt.Errorf("failed to decode config: %w", err)
	}

	return &config, nil
}
