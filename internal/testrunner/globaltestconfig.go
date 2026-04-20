// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package testrunner

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/elastic/go-ucfg"
	"github.com/elastic/go-ucfg/yaml"

	"github.com/elastic/elastic-package/internal/packages"
)

type globalTestConfig struct {
	Asset    GlobalRunnerTestConfig `config:"asset"`
	Pipeline GlobalRunnerTestConfig `config:"pipeline"`
	Policy   GlobalRunnerTestConfig `config:"policy"`
	Static   GlobalRunnerTestConfig `config:"static"`
	System   GlobalRunnerTestConfig `config:"system"`
}

// PackageTestRequirement matches package-spec package_requirement under each
// test runner's requires list: either {package, version} or {source} only.
type PackageTestRequirement struct {
	Package string `config:"package"`
	Version string `config:"version"`
	Source  string `config:"source"`
}

// RequiresSourceOverrides returns a map of required input package name to
// absolute local path for each requires entry that uses source (package-spec
// local override). Package names are read from manifest.yml at each source path.
func (c GlobalRunnerTestConfig) RequiresSourceOverrides(packageRoot string) (map[string]string, error) {
	return sourceOverridesFromRequirements(packageRoot, c.Requires)
}

// MergedRequiresSourceOverrides unions source overrides from all runner blocks.
// It returns an error if the same package name maps to different absolute paths
// in different blocks (no runner context to disambiguate).
func (c *globalTestConfig) MergedRequiresSourceOverrides(packageRoot string) (map[string]string, error) {
	runners := []struct {
		label string
		cfg   GlobalRunnerTestConfig
	}{
		{"system", c.System},
		{"policy", c.Policy},
		{"pipeline", c.Pipeline},
		{"static", c.Static},
		{"asset", c.Asset},
	}
	merged := make(map[string]string)
	for _, rn := range runners {
		part, err := sourceOverridesFromRequirements(packageRoot, rn.cfg.Requires)
		if err != nil {
			return nil, fmt.Errorf("%s.requires: %w", rn.label, err)
		}
		for k, v := range part {
			vClean := filepath.Clean(v)
			if prev, ok := merged[k]; ok {
				prevClean := filepath.Clean(prev)
				if prevClean != vClean {
					return nil, fmt.Errorf("conflicting requires source for package %q across test runners: %q vs %q", k, prev, v)
				}
				continue
			}
			merged[k] = v
		}
	}
	if len(merged) == 0 {
		return nil, nil
	}
	return merged, nil
}

type GlobalRunnerTestConfig struct {
	Parallel        bool                     `config:"parallel"`
	Requires        []PackageTestRequirement `config:"requires"`
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

	if err := validateGlobalTestConfigRequires(&c); err != nil {
		return nil, fmt.Errorf("invalid requires in global test configuration file %s: %w", configFilePath, err)
	}

	return &c, nil
}

func validateGlobalTestConfigRequires(c *globalTestConfig) error {
	runners := []struct {
		label string
		reqs  []PackageTestRequirement
	}{
		{"system", c.System.Requires},
		{"policy", c.Policy.Requires},
		{"pipeline", c.Pipeline.Requires},
		{"static", c.Static.Requires},
		{"asset", c.Asset.Requires},
	}
	for _, rn := range runners {
		if err := validatePackageTestRequirements(rn.label, rn.reqs); err != nil {
			return err
		}
	}
	return nil
}

func validatePackageTestRequirements(runnerLabel string, reqs []PackageTestRequirement) error {
	for i, r := range reqs {
		hasSource := strings.TrimSpace(r.Source) != ""
		hasPkg := strings.TrimSpace(r.Package) != ""
		hasVer := strings.TrimSpace(r.Version) != ""

		switch {
		case !hasSource && !hasPkg && !hasVer:
			return fmt.Errorf("%s.requires[%d]: empty entry (package-spec requires either {package, version} or {source})", runnerLabel, i)
		case hasSource && (hasPkg || hasVer):
			return fmt.Errorf("%s.requires[%d]: invalid entry: source must be used alone (package-spec package_requirement)", runnerLabel, i)
		case hasSource:
			continue
		case hasPkg && hasVer:
			continue
		case hasPkg && !hasVer:
			return fmt.Errorf("%s.requires[%d]: package %q requires version when not using source", runnerLabel, i, r.Package)
		case hasVer && !hasPkg:
			return fmt.Errorf("%s.requires[%d]: version %q requires package name when not using source", runnerLabel, i, r.Version)
		default:
			return fmt.Errorf("%s.requires[%d]: invalid package_requirement", runnerLabel, i)
		}
	}
	return nil
}

func sourceOverridesFromRequirements(packageRoot string, reqs []PackageTestRequirement) (map[string]string, error) {
	if len(reqs) == 0 {
		return nil, nil
	}
	out := make(map[string]string)
	for _, r := range reqs {
		if strings.TrimSpace(r.Source) == "" {
			continue
		}
		name, path, err := packageNameAndPathFromSource(packageRoot, r.Source)
		if err != nil {
			return nil, err
		}
		pathClean := filepath.Clean(path)
		if prev, ok := out[name]; ok {
			if filepath.Clean(prev) != pathClean {
				return nil, fmt.Errorf("duplicate requires source for package %q: %q vs %q", name, prev, path)
			}
			continue
		}
		out[name] = path
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

func absoluteSourcePath(packageRoot, source string) (string, error) {
	source = strings.TrimSpace(source)
	if source == "" {
		return "", fmt.Errorf("empty source path")
	}
	if filepath.IsAbs(source) {
		return filepath.Clean(source), nil
	}
	return filepath.Clean(filepath.Join(packageRoot, source)), nil
}

func packageNameAndPathFromSource(packageRoot, source string) (name, absPath string, err error) {
	absPath, err = absoluteSourcePath(packageRoot, source)
	if err != nil {
		return "", "", err
	}
	fi, err := os.Stat(absPath)
	if err != nil {
		return "", "", fmt.Errorf("requires source %q: %w", source, err)
	}
	if fi.IsDir() {
		m, err := packages.ReadPackageManifestFromPackageRoot(absPath)
		if err != nil {
			return "", "", fmt.Errorf("requires source %q: %w", source, err)
		}
		if m.Name == "" {
			return "", "", fmt.Errorf("requires source %q: package manifest has no name", source)
		}
		return m.Name, absPath, nil
	}
	if strings.EqualFold(filepath.Ext(absPath), ".zip") {
		m, err := packages.ReadPackageManifestFromZipPackage(absPath)
		if err != nil {
			return "", "", fmt.Errorf("requires source %q: %w", source, err)
		}
		if m.Name == "" {
			return "", "", fmt.Errorf("requires source %q: package manifest in zip has no name", source)
		}
		return m.Name, absPath, nil
	}
	return "", "", fmt.Errorf("requires source %q: not a directory or .zip package", source)
}
