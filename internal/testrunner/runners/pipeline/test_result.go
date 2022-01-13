// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package pipeline

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/kylelemons/godebug/diff"
	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/common"
	"github.com/elastic/elastic-package/internal/testrunner"
)

const expectedTestResultSuffix = "-expected.json"

type testResult struct {
	events []json.RawMessage
}

type testResultDefinition struct {
	Expected []json.RawMessage `json:"expected"`
}

func writeTestResult(testCasePath string, result *testResult) error {
	testCaseDir := filepath.Dir(testCasePath)
	testCaseFile := filepath.Base(testCasePath)

	data, err := marshalTestResultDefinition(result)
	if err != nil {
		return errors.Wrap(err, "marshalling test result failed")
	}
	err = os.WriteFile(filepath.Join(testCaseDir, expectedTestResultFile(testCaseFile)), data, 0644)
	if err != nil {
		return errors.Wrap(err, "writing test result failed")
	}
	return nil
}

func compareResults(testCasePath string, config *testConfig, result *testResult) error {
	resultsWithoutDynamicFields, err := adjustTestResult(result, config)
	if err != nil {
		return errors.Wrap(err, "can't adjust test results")
	}

	actual, err := marshalTestResultDefinition(resultsWithoutDynamicFields)
	if err != nil {
		return errors.Wrap(err, "marshalling actual test results failed")
	}

	expectedResults, err := readExpectedTestResult(testCasePath, config)
	if err != nil {
		return errors.Wrap(err, "reading expected test result failed")
	}

	expected, err := marshalTestResultDefinition(expectedResults)
	if err != nil {
		return errors.Wrap(err, "marshalling expected test results failed")
	}

	report := diff.Diff(string(expected), string(actual))
	if report != "" {
		return testrunner.ErrTestCaseFailed{
			Reason:  "Expected results are different from actual ones",
			Details: report,
		}
	}
	return nil
}

func readExpectedTestResult(testCasePath string, config *testConfig) (*testResult, error) {
	testCaseDir := filepath.Dir(testCasePath)
	testCaseFile := filepath.Base(testCasePath)

	path := filepath.Join(testCaseDir, expectedTestResultFile(testCaseFile))
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.Wrap(err, "reading test result file failed")
	}

	u, err := unmarshalTestResult(data)
	if err != nil {
		return nil, errors.Wrap(err, "unmarshalling expected test result failed")
	}

	adjusted, err := adjustTestResult(u, config)
	if err != nil {
		return nil, errors.Wrap(err, "adjusting test result failed")
	}
	return adjusted, nil
}

func adjustTestResult(result *testResult, config *testConfig) (*testResult, error) {
	if config == nil || config.DynamicFields == nil {
		return result, nil
	}

	// Strip dynamic fields from test result
	var stripped testResult
	for _, event := range result.events {
		if event == nil {
			stripped.events = append(stripped.events, nil)
			continue
		}

		var m common.MapStr
		err := json.Unmarshal(event, &m)
		if err != nil {
			return nil, errors.Wrapf(err, "can't unmarshal event: %s", string(event))
		}

		for key := range config.DynamicFields {
			err := m.Delete(key)
			if err != nil && err != common.ErrKeyNotFound {
				return nil, errors.Wrap(err, "can't remove dynamic field")
			}
		}

		b, err := json.Marshal(&m)
		if err != nil {
			return nil, errors.Wrap(err, "can't marshal event")
		}

		stripped.events = append(stripped.events, b)
	}
	return &stripped, nil
}

func unmarshalTestResult(body []byte) (*testResult, error) {
	var trd testResultDefinition
	err := json.Unmarshal(body, &trd)
	if err != nil {
		return nil, errors.Wrap(err, "unmarshalling test result failed")
	}

	var tr testResult
	tr.events = append(tr.events, trd.Expected...)
	return &tr, nil
}

func marshalTestResultDefinition(result *testResult) ([]byte, error) {
	var trd testResultDefinition
	trd.Expected = result.events
	body, err := marshalNormalizedJSON(trd)
	if err != nil {
		return nil, errors.Wrap(err, "marshalling test result definition failed")
	}
	return body, nil
}

// marshalNormalizedJSON marshals test results ensuring that field
// order remains consistent independent of field order returned by
// ES to minimize diff noise during changes.
func marshalNormalizedJSON(v testResultDefinition) ([]byte, error) {
	msg, err := json.Marshal(v)
	if err != nil {
		return msg, err
	}
	var obj interface{}
	err = json.Unmarshal(msg, &obj)
	if err != nil {
		return msg, err
	}
	return json.MarshalIndent(obj, "", "    ")
}

func expectedTestResultFile(testFile string) string {
	return fmt.Sprintf("%s%s", testFile, expectedTestResultSuffix)
}
