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

	"github.com/elastic/elastic-package/internal/packages"
)

type globalTestConfig struct {
	Requires []packages.RequiresOverride `config:"requires,omitempty"`

	Asset    GlobalRunnerTestConfig `config:"asset"`
	Pipeline GlobalRunnerTestConfig `config:"pipeline"`
	Policy   GlobalRunnerTestConfig `config:"policy"`
	Static   GlobalRunnerTestConfig `config:"static"`
	System   GlobalRunnerTestConfig `config:"system"`
}

// RequiresOverrides returns a merged map of requires overrides for the given
// test type. Per-type entries override global entries by package name.
func (c *globalTestConfig) RequiresOverrides(testType TestType) map[string]packages.RequiresOverride {
	merged := make(map[string]packages.RequiresOverride, len(c.Requires))
	for _, r := range c.Requires {
		merged[r.Package] = r
	}

	var typeRequires []packages.RequiresOverride
	switch testType {
	case "asset":
		typeRequires = c.Asset.Requires
	case "pipeline":
		typeRequires = c.Pipeline.Requires
	case "policy":
		typeRequires = c.Policy.Requires
	case "static":
		typeRequires = c.Static.Requires
	case "system":
		typeRequires = c.System.Requires
	}
	for _, r := range typeRequires {
		merged[r.Package] = r
	}

	if len(merged) == 0 {
		return nil
	}
	return merged
}

type GlobalRunnerTestConfig struct {
	Parallel        bool                        `config:"parallel"`
	Requires        []packages.RequiresOverride `config:"requires,omitempty"`
	SkippableConfig `config:",inline"`

	// MergedRequiresOverrides is populated by the caller after reading
	// the config. It holds the result of merging top-level requires with
	// this test type's requires (type takes precedence).
	MergedRequiresOverrides map[string]packages.RequiresOverride `config:"-"`
}

// prepare populates MergedRequiresOverrides on each per-type config by
// merging the top-level requires with the type-specific requires.
func (c *globalTestConfig) prepare() {
	types := map[TestType]*GlobalRunnerTestConfig{
		"asset":    &c.Asset,
		"pipeline": &c.Pipeline,
		"policy":   &c.Policy,
		"static":   &c.Static,
		"system":   &c.System,
	}
	for tt, cfg := range types {
		cfg.MergedRequiresOverrides = c.RequiresOverrides(tt)
	}
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

	c.prepare()
	return &c, nil
}
