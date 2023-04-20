// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package system

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/aymerick/raymond"
	"github.com/elastic/elastic-package/internal/benchrunner/runners/system/servicedeployer"
	"github.com/elastic/go-ucfg"
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
	Generator    *generator    `config:"generator"`
	InputService *inputService `config:"input_service"`
}

type inputService struct {
	Name   string `config:"name"`
	Signal string `config:"signal"`
}

type generator struct {
	Size         string `config:"size"`
	TemplatePath string `config:"template_path"`
	ConfigPath   string `config:"config_path"`
	FieldsPath   string `config:"fields_path"`
}

func defaultConfig() *scenario {
	return &scenario{}
}

func readConfig(path, scenario string, ctxt servicedeployer.ServiceContext) (*scenario, error) {
	configPath := filepath.Join(path, devPath, fmt.Sprintf("%s.yml", scenario))
	data, err := os.ReadFile(configPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("unable to find system test configuration file: %s: %w", configPath, err)
		}
		return nil, fmt.Errorf("could not load system test configuration file: %s: %w", configPath, err)
	}

	data, err = applyContext(data, ctxt)
	if err != nil {
		return nil, fmt.Errorf("could not apply context to test configuration file: %s: %w", configPath)
	}

	cfg, err := yaml.NewConfig(data, ucfg.PathSep("."))
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

// applyContext takes the given system test configuration (data) and replaces any placeholder variables in
// it with values from the given context (ctxt). The context may be populated from various sources but usually the
// most interesting context values will be set by a ServiceDeployer in its SetUp method.
func applyContext(data []byte, ctxt servicedeployer.ServiceContext) ([]byte, error) {
	tmpl, err := raymond.Parse(string(data))
	if err != nil {
		return data, fmt.Errorf("parsing template body failed: %w", err)
	}
	tmpl.RegisterHelpers(ctxt.Aliases())

	result, err := tmpl.Exec(ctxt)
	if err != nil {
		return data, fmt.Errorf("could not render data with context: %w", err)
	}
	return []byte(result), nil
}
