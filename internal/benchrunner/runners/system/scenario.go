// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package system

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/aymerick/raymond"
	"github.com/elastic/go-ucfg"
	"github.com/elastic/go-ucfg/yaml"

	"github.com/elastic/elastic-package/internal/servicedeployer"
)

var allowedDeployerNames = []string{"docker", "k8s", "tf"}

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
	Deployer            string                 `config:"deployer" json:"deployer"` // Name of the service deployer to use for this scenario.
}

type dataStream struct {
	Name string                 `config:"name" json:"name"`
	Vars map[string]interface{} `config:"vars" json:"vars"`
}

type corpora struct {
	Generator    *generator    `config:"generator" json:"generator"`
	InputService *inputService `config:"input_service" json:"input_service"`
}

type inputService struct {
	Name   string `config:"name" json:"name"`
	Signal string `config:"signal" json:"signal"`
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
	timeout := 10 * time.Minute
	return &scenario{
		WaitForDataTimeout: &timeout,
	}
}

// readRawConfig reads the configuration without applying any template
func readRawConfig(benchPath string, scenario string) (*scenario, error) {
	return readConfig(benchPath, scenario, nil)
}

func readConfig(benchPath string, scenario string, svcInfo *servicedeployer.ServiceInfo) (*scenario, error) {
	configPath := filepath.Clean(filepath.Join(benchPath, fmt.Sprintf("%s.yml", scenario)))
	data, err := os.ReadFile(configPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("unable to find system benchmark configuration file: %s: %w", configPath, err)
		}
		return nil, fmt.Errorf("could not load system benchmark configuration file: %s: %w", configPath, err)
	}

	if svcInfo != nil {
		data, err = applyServiceInfo(data, *svcInfo)
		if err != nil {
			return nil, fmt.Errorf("could not apply context to benchmark configuration file: %s: %w", configPath, err)
		}
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
	if c.Deployer != "" && !slices.Contains(allowedDeployerNames, c.Deployer) {
		return nil, fmt.Errorf("invalid deployer name %q in system benchmark configuration file %q, allowed values are: %s",
			c.Deployer, configPath, strings.Join(allowedDeployerNames, ", "))
	}
	return c, nil
}

// applyServiceInfo takes the given system benchmark configuration (data) and replaces any placeholder variables in
// it with values from the given service information. The context may be populated from various sources but usually the
// most interesting context values will be set by a ServiceDeployer in its SetUp method.
func applyServiceInfo(data []byte, svcInfo servicedeployer.ServiceInfo) ([]byte, error) {
	tmpl, err := raymond.Parse(string(data))
	if err != nil {
		return data, fmt.Errorf("parsing template body failed: %w", err)
	}
	tmpl.RegisterHelpers(svcInfo.Aliases())

	result, err := tmpl.Exec(svcInfo)
	if err != nil {
		return data, fmt.Errorf("could not render data with context: %w", err)
	}
	return []byte(result), nil
}
