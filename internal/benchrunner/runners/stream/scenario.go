// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package stream

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/elastic/go-ucfg/yaml"
)

const devPath = "_dev/benchmark/rally"

type scenario struct {
	Package     string     `config:"package" json:"package"`
	Description string     `config:"description" json:"description"`
	Version     string     `config:"version" json:"version"`
	DataStream  dataStream `config:"data_stream" json:"data_stream"`
	Corpora     corpora    `config:"corpora" json:"corpora"`
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
	Config      corporaAsset    `config:"config" json:"config"`
	Fields      corporaAsset    `config:"fields" json:"fields"`
}

type corporaAsset struct {
	Raw  map[string]interface{} `config:"raw" json:"raw"`
	Path string                 `config:"path" json:"path"`
}
type corporaTemplate struct {
	Raw  string `config:"raw" json:"raw"`
	Path string `config:"path" json:"path"`
	Type string `config:"type" json:"type"`
}

func defaultConfig() *scenario {
	return &scenario{}
}

func readConfig(path, scenarioName, packageName, packageVersion string) (*scenario, error) {
	configPath := filepath.Join(path, devPath, fmt.Sprintf("%s.yml", scenarioName))
	c := defaultConfig()
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
func readScenarios(path, scenarioName, packageName, packageVersion string) (map[string]*scenario, error) {
	scenarios := make(map[string]*scenario)
	if len(scenarioName) > 0 {
		scenario, err := readConfig(path, scenarioName, packageName, packageVersion)
		if err != nil {
			return nil, fmt.Errorf("error loading scenario: %w", err)
		}
		scenarios[scenarioName] = scenario
	} else {
		err := filepath.Walk(filepath.Join(path, devPath), func(_ string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if info.IsDir() {
				return nil
			}

			if strings.HasSuffix(info.Name(), "-benchmark.yml") {
				scenarioName = strings.TrimSuffix(info.Name(), ".yml")
				scenario, err := readConfig(path, scenarioName, packageName, packageVersion)
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
