// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package testrunner

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/elastic/go-ucfg"
	"github.com/elastic/go-ucfg/yaml"
)

type globalTestConfig struct {
	Asset    GlobalRunnerTestConfig `config:"asset"`
	Pipeline GlobalRunnerTestConfig `config:"pipeline"`
	Policy   GlobalRunnerTestConfig `config:"policy"`
	Static   GlobalRunnerTestConfig `config:"static"`
	System   GlobalRunnerTestConfig `config:"system"`
	Requires []RequiresTestOverride `config:"requires"`
}

// RequiresTestOverride maps a required input package name to a local source path.
// The source path may be relative (resolved against the package root) or absolute.
// It may point to a directory or a zip file.
type RequiresTestOverride struct {
	Package string `config:"package"`
	Source  string `config:"source"`
}

// RequiresSourceOverrides returns a map of package name to absolute local source path
// for each entry in the Requires section. Relative source paths are resolved against
// packageRoot. Returns nil if no overrides are configured.
func (c *globalTestConfig) RequiresSourceOverrides(packageRoot string) map[string]string {
	if len(c.Requires) == 0 {
		return nil
	}
	overrides := make(map[string]string, len(c.Requires))
	for _, r := range c.Requires {
		sourcePath := r.Source
		if !filepath.IsAbs(sourcePath) {
			sourcePath = filepath.Join(packageRoot, sourcePath)
		}
		overrides[r.Package] = sourcePath
	}
	return overrides
}

type GlobalRunnerTestConfig struct {
	Parallel        bool `config:"parallel"`
	SkippableConfig `config:",inline"`
}

func ReadGlobalTestConfig(packageRoot string) (*globalTestConfig, error) {
	configFilePath := filepath.Join(packageRoot, "_dev", "test", "config.yml")

	data, err := os.ReadFile(configFilePath)
	if errors.Is(err, os.ErrNotExist) {
		return &globalTestConfig{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", configFilePath, err)
	}

	var c globalTestConfig
	cfg, err := yaml.NewConfig(data, ucfg.PathSep("."))
	if err != nil {
		return nil, fmt.Errorf("unable to load global test configuration file: %s: %w", configFilePath, err)
	}
	if err := cfg.Unpack(&c); err != nil {
		return nil, fmt.Errorf("unable to unpack global test configuration file: %s: %w", configFilePath, err)
	}

	return &c, nil
}
