package common

import (
	"errors"
	"fmt"
	"github.com/elastic/go-ucfg/yaml"
	"os"
	"path/filepath"
	"strings"
)

// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

const DevPath = "_dev/benchmark/rally"

type Scenario struct {
	Package     string     `config:"package" json:"package"`
	Description string     `config:"description" json:"description"`
	Version     string     `config:"version" json:"version"`
	DataStream  DataStream `config:"data_stream" json:"data_stream"`
	Corpora     Corpora    `config:"corpora" json:"corpora"`
}

type DataStream struct {
	Name string `config:"name" json:"name"`
}

type Corpora struct {
	Generator *Generator `config:"generator" json:"generator"`
}

type Generator struct {
	TotalEvents uint64          `config:"total_events" json:"total_events"`
	Template    CorporaTemplate `config:"template" json:"template"`
	Config      CorporaAsset    `config:"config" json:"config"`
	Fields      CorporaAsset    `config:"fields" json:"fields"`
}

type CorporaAsset struct {
	Raw  map[string]interface{} `config:"raw" json:"raw"`
	Path string                 `config:"path" json:"path"`
}
type CorporaTemplate struct {
	Raw  string `config:"raw" json:"raw"`
	Path string `config:"path" json:"path"`
	Type string `config:"type" json:"type"`
}

func DefaultConfig() *Scenario {
	return &Scenario{}
}

func ReadConfig(path, scenario, packageName, packageVersion string) (*Scenario, error) {
	configPath := filepath.Join(path, DevPath, fmt.Sprintf("%s.yml", scenario))
	c := DefaultConfig()
	cfg, err := yaml.NewConfigWithFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("can't load benchmark configuration: %s: %w", configPath, err)
	}

	if err == nil {
		if err := cfg.Unpack(c); err != nil {
			return nil, fmt.Errorf("can't unpack benchmark configuration: %s: %w", configPath, err)
		}
	}

	c.Package = packageName
	c.Version = packageVersion

	if c.DataStream.Name == "" {
		return nil, errors.New("can't read data stream name from benchmark configuration: empty")
	}

	return c, nil
}

func ReadScenarios(path, scenarioName, packageName, packageVersion string) (map[string]*Scenario, error) {
	scenarios := make(map[string]*Scenario)
	if len(scenarioName) > 0 {
		scenario, err := ReadConfig(path, scenarioName, packageName, packageVersion)
		if err != nil {
			return nil, fmt.Errorf("error loading scenario: %w", err)
		}
		scenarios[scenarioName] = scenario
	} else {
		err := filepath.Walk(filepath.Join(path, DevPath), func(_ string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if info.IsDir() {
				return nil
			}

			if strings.HasSuffix(info.Name(), "-benchmark.yml") {
				scenarioName = strings.TrimSuffix(info.Name(), ".yml")
				scenario, err := ReadConfig(path, scenarioName, packageName, packageVersion)
				if err != nil {
					return err
				}
				scenarios[scenarioName] = scenario
			}

			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("error loading scenario: %w", err)
		}
	}

	return scenarios, nil
}
