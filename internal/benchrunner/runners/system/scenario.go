// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package system

import (
	"errors"
	"fmt"
	"github.com/elastic/elastic-package/internal/benchrunner/runners/common"
	"os"
	"path/filepath"
	"time"

	"github.com/aymerick/raymond"
	"github.com/elastic/go-ucfg"
	"github.com/elastic/go-ucfg/yaml"

	"github.com/elastic/elastic-package/internal/servicedeployer"
)

type scenario struct {
	Package             string                 `config:"package" json:"package"`
	Description         string                 `config:"description" json:"description"`
	Version             string                 `config:"version" json:"version"`
	PolicyTemplate      string                 `config:"policy_template" json:"policy_template"`
	Input               string                 `config:"input" json:"input"`
	Vars                map[string]interface{} `config:"vars" json:"vars"`
	DataStream          dataStream             `config:"data_stream" json:"data_stream"`
	WarmupTimePeriod    time.Duration          `config:"warmup_time_period" json:"warmup_time_period"`
	BenchmarkTimePeriod time.Duration          `config:"benchmark_time_period" json:"benchmark_time_period"`
	WaitForDataTimeout  *time.Duration         `config:"wait_for_data_timeout" json:"wait_for_data_timeout"`
	Corpora             corpora                `config:"corpora" json:"corpora"`
}

// TODO: Why is this and next one slightly different from the common fields?
type dataStream struct {
	Name string                 `config:"name" json:"name"`
	Vars map[string]interface{} `config:"vars" json:"vars"`
}

type corpora struct {
	Generator    *common.Generator `config:"generator" json:"generator"`
	InputService *inputService     `config:"input_service" json:"input_service"`
}

type inputService struct {
	Name   string `config:"name" json:"name"`
	Signal string `config:"signal" json:"signal"`
}

// TODO: This seems to be identical to CorporaAsset
type corporaConfig struct {
	Raw  map[string]interface{} `config:"raw" json:"raw"`
	Path string                 `config:"path" json:"path"`
}

// TODO: This seems to be identical to CorporaAsset
type corporaFields struct {
	Raw  map[string]interface{} `config:"raw" json:"raw"`
	Path string                 `config:"path" json:"path"`
}

func defaultConfig() *scenario {
	timeout := 10 * time.Minute
	return &scenario{
		WaitForDataTimeout: &timeout,
	}
}

func readConfig(benchPath string, scenario string, ctxt servicedeployer.ServiceContext) (*scenario, error) {
	configPath := filepath.Clean(filepath.Join(benchPath, fmt.Sprintf("%s.yml", scenario)))
	data, err := os.ReadFile(configPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("unable to find system benchmark configuration file: %s: %w", configPath, err)
		}
		return nil, fmt.Errorf("could not load system benchmark configuration file: %s: %w", configPath, err)
	}

	data, err = applyContext(data, ctxt)
	if err != nil {
		return nil, fmt.Errorf("could not apply context to benchmark configuration file: %s: %w", configPath, err)
	}

	cfg, err := yaml.NewConfig(data, ucfg.PathSep("."))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
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

// applyContext takes the given system benchmark configuration (data) and replaces any placeholder variables in
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
