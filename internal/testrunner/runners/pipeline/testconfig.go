// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package pipeline

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/elastic/go-ucfg/yaml"

	"github.com/elastic/elastic-package/internal/testrunner"
)

const (
	configTestSuffixYAML = "-config.yml"
	commonTestConfigYAML = "test-common-config.yml"
)

type TestExpectedOutput struct {
	// Name is the name of the file that contains the output which the test is expected to match.
	Name string `config:"name"`

	// VersionConstraints holds version constraints of running services, namely "elasticsearch",
	// "kibana", etc. that need to match in order for this output to be considered as the
	// expected one for the test.
	VersionConstraints map[string]string `config:"version_constraints"`
}

type testConfig struct {
	testrunner.SkippableConfig `config:",inline"`

	Multiline     *multiline             `config:"multiline"`
	Fields        map[string]interface{} `config:"fields"`
	DynamicFields map[string]string      `config:"dynamic_fields"`

	// NumericKeywordFields holds a list of fields that have keyword
	// type but can be ingested as numeric type.
	NumericKeywordFields []string `config:"numeric_keyword_fields"`

	// StringNumberFields holds a list of fields that have numeric
	// types but can be ingested as strings.
	StringNumberFields []string `config:"string_number_fields"`

	// ExpectedOutputs holds the expected outputs of the test case.
	ExpectedOutputs []TestExpectedOutput `config:"expected_outputs"`
}

type multiline struct {
	FirstLinePattern string `config:"first_line_pattern"`
}

func readConfigForTestCase(testCasePath string) (*testConfig, error) {
	testCaseDir := filepath.Dir(testCasePath)
	testCaseFile := filepath.Base(testCasePath)

	commonConfigPath := filepath.Join(testCaseDir, commonTestConfigYAML)
	var c testConfig
	cfg, err := yaml.NewConfigWithFile(commonConfigPath)
	if err == nil {
		if len(c.ExpectedOutputs) > 0 {
			return nil, fmt.Errorf("expected_outputs is not supported in common configuration: %s", commonConfigPath)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("can't load common configuration: %s: %w", commonConfigPath, err)
	}

	configPath := filepath.Join(testCaseDir, expectedTestConfigFile(testCaseFile, configTestSuffixYAML))
	cfgTestCase, err := yaml.NewConfigWithFile(configPath)
	if err == nil {
		if cfg == nil {
			cfg = cfgTestCase
		} else {
			if err := cfg.Merge(cfgTestCase); err != nil {
				return nil, fmt.Errorf("can't merge common configuration %s with test case configuration %s: %w", commonConfigPath, configPath, err)
			}
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("can't load test configuration: %s: %w", configPath, err)
	}

	if cfg != nil {
		if err := cfg.Unpack(&c); err != nil {
			return nil, fmt.Errorf("can't unpack test configuration: %s: %w", commonConfigPath, err)
		}
	}
	return &c, nil
}

func expectedTestConfigFile(testFile, configTestSuffix string) string {
	return fmt.Sprintf("%s%s", testFile, configTestSuffix)
}
