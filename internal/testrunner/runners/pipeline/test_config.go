// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package pipeline

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
)

const configTestSuffix = "-config.json"

type testConfig struct {
	Multiline *multiline             `json:"multiline"`
	Fields    map[string]interface{} `json:"fields"`
}

func readConfigForTestCase(testCasePath string) (testConfig, error) {
	testCaseDir := filepath.Dir(testCasePath)
	testCaseFile := filepath.Base(testCasePath)

	var c testConfig
	configData, err := ioutil.ReadFile(filepath.Join(testCaseDir, expectedTestConfigFile(testCaseFile)))
	if err != nil && !os.IsNotExist(err) {
		return c, errors.Wrapf(err, "reading test config file failed (path: %s)", testCasePath)
	}

	if configData == nil {
		return c, nil
	}

	err = json.Unmarshal(configData, &c)
	if err != nil {
		return c, errors.Wrap(err, "unmarshalling test config failed")
	}
	return c, nil
}

func expectedTestConfigFile(testFile string) string {
	return fmt.Sprintf("%s%s", testFile, configTestSuffix)
}
