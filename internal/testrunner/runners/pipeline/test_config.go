// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package pipeline

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/elastic/go-ucfg/yaml"
	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/testrunner"
)

const (
	configTestSuffixYAML = "-config.yml"
	commonTestConfigYAML = "test-common-config.yml"
)

type testConfig struct {
	testrunner.SkippableConfig `config:",inline"`

	Multiline     *multiline             `config:"multiline"`
	Fields        map[string]interface{} `config:"fields"`
	DynamicFields map[string]string      `config:"dynamic_fields"`

	// NumericKeywordFields holds a list of fields that have keyword
	// type but can be ingested as numeric type.
	NumericKeywordFields []string `config:"numeric_keyword_fields"`
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
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, errors.Wrapf(err, "can't load common configuration: %s", commonConfigPath)
	}

	if err == nil {
		if err := cfg.Unpack(&c); err != nil {
			return nil, errors.Wrapf(err, "can't unpack test configuration: %s", commonConfigPath)
		}
	}

	configPath := filepath.Join(testCaseDir, expectedTestConfigFile(testCaseFile, configTestSuffixYAML))
	cfg, err = yaml.NewConfigWithFile(configPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, errors.Wrapf(err, "can't load test configuration: %s", configPath)
	}

	if err == nil {
		if err := cfg.Unpack(&c); err != nil {
			return nil, errors.Wrapf(err, "can't unpack test configuration: %s", configPath)
		}
	}
	return &c, nil
}

func expectedTestConfigFile(testFile, configTestSuffix string) string {
	return fmt.Sprintf("%s%s", testFile, configTestSuffix)
}
