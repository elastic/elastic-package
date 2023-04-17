// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package system

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/elastic/go-ucfg/yaml"
)

const (
	devPath = "_dev/benchmark/system"
)

type scenario struct {
	Description      string                 `config:"description"`
	Version          string                 `config:"version"`
	PolicyTemplate   string                 `config:"policy_template"`
	Input            string                 `config:"input"`
	Vars             map[string]interface{} `config:"vars"`
	DataStream       dataStream             `config:"data_stream"`
	WarmupTimePeriod int                    `config:"warmup_time_period"`
	Corpora          corpora                `config:"corpora"`
}

type dataStream struct {
	Name string                 `config:"name"`
	Vars map[string]interface{} `config:"vars"`
}

type corpora struct {
	InputService string `config:"input_service"`
}

func defaultConfig() *scenario {
	return &scenario{}
}

func readConfig(path, scenario string) (*scenario, error) {
	configPath := filepath.Join(path, devPath, fmt.Sprintf("%s.yml", scenario))
	cfg, err := yaml.NewConfigWithFile(configPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			configPath = filepath.Join(path, devPath, fmt.Sprintf("%s.yaml", scenario))
			cfg, err = yaml.NewConfigWithFile(configPath)
		}
		if err != nil {
			return nil, fmt.Errorf("can't load scenario: %s: %w", configPath, err)
		}
	}

	c := defaultConfig()
	if err := cfg.Unpack(c); err != nil {
		return nil, fmt.Errorf("can't unpack scenario configuration: %s: %w", configPath, err)
	}
	return c, nil
}
