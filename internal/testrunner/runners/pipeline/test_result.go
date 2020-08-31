// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package pipeline

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"

	"github.com/kylelemons/godebug/diff"
	"github.com/pkg/errors"
)

const expectedTestResultSuffix = "-expected.json"

var errTestCaseFailed = errors.New("test case failed")

type testResult struct {
	events []json.RawMessage
}

type testResultDefinition struct {
	Expected []json.RawMessage `json:"expected"`
}

func writeTestResult(testCasePath string, result *testResult) error {
	testCaseDir := filepath.Dir(testCasePath)
	testCaseFile := filepath.Base(testCasePath)

	data, err := marshalTestResult(result)
	if err != nil {
		return errors.Wrap(err, "marshalling test result failed")
	}
	err = ioutil.WriteFile(filepath.Join(testCaseDir, expectedTestResultFile(testCaseFile)), data, 0644)
	if err != nil {
		return errors.Wrap(err, "writing test result failed")
	}
	return nil
}

func compareResults(testCasePath string, result *testResult) error {
	current, err := marshalTestResult(result)
	if err != nil {
		return errors.Wrap(err, "marshalling test result failed")
	}

	expected, err := readExpectedTestResult(testCasePath)
	if err != nil {
		return errors.Wrap(err, "reading expected test result failed")
	}

	report := diff.Diff(string(expected), string(current))
	if report != "" {
		fmt.Println("Expected results are different from current ones:")
		fmt.Println(report)
		return errTestCaseFailed
	}
	return nil
}

func marshalTestResult(result *testResult) ([]byte, error) {
	var trd testResultDefinition
	trd.Expected = result.events
	body, err := json.MarshalIndent(&trd, "", "    ")
	if err != nil {
		return nil, errors.Wrap(err, "marshalling test result failed")
	}
	return body, nil
}

func readExpectedTestResult(testCasePath string) ([]byte, error) {
	testCaseDir := filepath.Dir(testCasePath)
	testCaseFile := filepath.Base(testCasePath)

	path := filepath.Join(testCaseDir, expectedTestResultFile(testCaseFile))
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, errors.Wrap(err, "reading test result file failed")
	}
	return data, nil
}

func expectedTestResultFile(testFile string) string {
	return fmt.Sprintf("%s%s", testFile, expectedTestResultSuffix)
}
