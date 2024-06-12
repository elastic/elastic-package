package testrunner

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/elastic/go-ucfg"
	"github.com/elastic/go-ucfg/yaml"
)

type GlobalTestConfig struct {
	Asset    GlobalRunnerTestConfig `config:"asset"`
	Pipeline GlobalRunnerTestConfig `config:"pipeline"`
	Policy   GlobalRunnerTestConfig `config:"policy"`
	Static   GlobalRunnerTestConfig `config:"static"`
	System   GlobalRunnerTestConfig `config:"system"`
}

type GlobalRunnerTestConfig struct {
	Sequential      bool `config:"sequential"`
	SkippableConfig `config:",inline"`
}

func AGlobalTestConfig(packageRootPath string) (*GlobalTestConfig, error) {
	configFilePath := filepath.Join(packageRootPath, "_dev", "test", "config.yml")

	data, err := os.ReadFile(configFilePath)
	if errors.Is(err, os.ErrNotExist) {
		return &GlobalTestConfig{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", configFilePath, err)
	}

	var c GlobalTestConfig
	cfg, err := yaml.NewConfig(data, ucfg.PathSep("."))
	if err != nil {
		return nil, fmt.Errorf("unable to load global test configuration file: %s: %w", configFilePath, err)
	}
	if err := cfg.Unpack(&c); err != nil {
		return nil, fmt.Errorf("unable to unpack global test configuration file: %s: %w", configFilePath, err)
	}

	return &c, nil
}
