// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package pipeline

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/testrunner"
)

const (
	configTestSuffixJSON = "-config.json"
	configTestSuffixYAML = "-config.yml"
)

type testConfig struct {
	testrunner.SkippableConfig `config:",inline"`

	Multiline     *multiline             `json:"multiline" yaml:"multiline"`
	Fields        map[string]interface{} `json:"fields" yaml:"fields"`
	DynamicFields map[string]string      `json:"dynamic_fields" yaml:"dynamic_fields"`

	// NumericKeywordFields holds a list of fields that have keyword
	// type but can be ingested as numeric type.
	NumericKeywordFields []string `json:"numeric_keyword_fields" yaml:"numeric_keyword_fields"`
}

type multiline struct {
	FirstLinePattern string `json:"first_line_pattern" yaml:"first_line_pattern"`
}

func readConfigForTestCase(testCasePath string) (testConfig, error) {
	testCaseDir := filepath.Dir(testCasePath)
	testCaseFile := filepath.Base(testCasePath)

	var c testConfig
	configData, err := os.ReadFile(filepath.Join(testCaseDir, expectedTestConfigFile(testCaseFile, configTestSuffixYAML)))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return c, errors.Wrapf(err, "reading YAML-formatted test config file failed (path: %s)", testCasePath)
	}

	if configData != nil {
		if err := yaml.Unmarshal(configData, &c); err != nil {
			return c, errors.Wrap(err, "unmarshalling YAML-formatted test config failed")
		}

		return c, nil
	}

	configData, err = os.ReadFile(filepath.Join(testCaseDir, expectedTestConfigFile(testCaseFile, configTestSuffixJSON)))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return c, errors.Wrapf(err, "reading JSON-formatted test config file failed (path: %s)", testCasePath)
	}

	if configData != nil {
		if err := json.Unmarshal(configData, &c); err != nil {
			return c, errors.Wrap(err, "unmarshalling JSON-formatted test config failed")
		}

		return c, nil
	}

	return c, nil
}

func expectedTestConfigFile(testFile, configTestSuffix string) string {
	return fmt.Sprintf("%s%s", testFile, configTestSuffix)
}
