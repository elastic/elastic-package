// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package rally

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/elastic/go-ucfg/yaml"
)

const devPath = "_dev/benchmark/rally"

type scenario struct {
	Package          string        `config:"package" json:"package"`
	Description      string        `config:"description" json:"description"`
	Version          string        `config:"version" json:"version"`
	DataStream       dataStream    `config:"data_stream" json:"data_stream"`
	WarmupTimePeriod time.Duration `config:"warmup_time_period" json:"warmup_time_period"`
	Corpora          corpora       `config:"corpora" json:"corpora"`
}

type dataStream struct {
	Name string `config:"name" json:"name"`
}

type corpora struct {
	Generator *generator `config:"generator" json:"generator"`
}

type generator struct {
	TotalEvents uint64          `config:"total_events" json:"total_events"`
	Template    corporaTemplate `config:"template" json:"template"`
	Config      corporaConfig   `config:"config" json:"config"`
	Fields      corporaFields   `config:"fields" json:"fields"`
}

type corporaTemplate struct {
	Raw  string `config:"raw" json:"raw"`
	Path string `config:"path" json:"path"`
	Type string `config:"type" json:"type"`
}

type corporaConfig struct {
	Raw  map[string]interface{} `config:"raw" json:"raw"`
	Path string                 `config:"path" json:"path"`
}

type corporaFields struct {
	Raw  map[string]interface{} `config:"raw" json:"raw"`
	Path string                 `config:"path" json:"path"`
}

func defaultConfig() *scenario {
	return &scenario{}
}

func readConfig(path, scenario, packageName, packageVersion string) (*scenario, error) {
	configPath := filepath.Join(path, devPath, fmt.Sprintf("%s.yml", scenario))
	c := defaultConfig()
	cfg, err := yaml.NewConfigWithFile(configPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("can't load benchmark configuration: %s: %w", configPath, err)
	}

	if err == nil {
		if err := cfg.Unpack(c); err != nil {
			return nil, fmt.Errorf("can't unpack benchmark configuration: %s: %w", configPath, err)
		}
	}

	c.Package = packageName
	c.Version = packageVersion

	return c, nil
}
