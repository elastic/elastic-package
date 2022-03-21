// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package pipeline

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/nsf/jsondiff"
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

	report, err := diffJson(expected, actual)
	if err != nil {
		return errors.Wrap(err, "comparing expected test result")
	}
	if report != "" {
		return testrunner.ErrTestCaseFailed{
			Reason:  "Expected results are different from actual ones",
			Details: report,
		}
	}

	return nil
}

func compareJsonNumbers(a, b json.Number) bool {
	if a == b {
		// Equal literals, so they are the same.
		return true
	}
	if inta, err := a.Int64(); err == nil {
		if intb, err := b.Int64(); err == nil {
			return inta == intb
		}
		if floatb, err := b.Float64(); err == nil {
			return float64(inta) == floatb
		}
	} else if floata, err := a.Float64(); err == nil {
		if intb, err := b.Int64(); err == nil {
			return floata == float64(intb)
		}
		if floatb, err := b.Float64(); err == nil {
			return floata == floatb
		}
	}
	return false
}

func diffJson(want, got []byte) (string, error) {
	opts := jsondiff.DefaultConsoleOptions()

	// Remove colored output.
	opts.Added = jsondiff.Tag{}
	opts.Removed = jsondiff.Tag{}
	opts.Changed = jsondiff.Tag{}
	opts.Skipped = jsondiff.Tag{}

	// Configure diff.
	opts.SkipMatches = true
	opts.CompareNumbers = compareJsonNumbers

	result, diff := jsondiff.Compare(want, got, &opts)
	switch result {
	case jsondiff.FirstArgIsInvalidJson, jsondiff.SecondArgIsInvalidJson, jsondiff.BothArgsAreInvalidJson:
		return "", errors.New("invalid json")
	}
	return diff, nil
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
		err := jsonUnmarshalUsingNumber(event, &m)
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
	err := jsonUnmarshalUsingNumber(body, &trd)
	if err != nil {
		return nil, errors.Wrap(err, "unmarshalling test result failed")
	}

	var tr testResult
	tr.events = append(tr.events, trd.Expected...)
	return &tr, nil
}

// jsonUnmarshalUsingNumber is a drop-in replacement for json.Unmarshal that
// does not default to unmarshaling numeric values to float64 in order to
// prevent low bit truncation of values greater than 1<<53.
// See https://golang.org/cl/6202068 for details.
func jsonUnmarshalUsingNumber(data []byte, v interface{}) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	err := dec.Decode(v)
	if err != nil {
		if err == io.EOF {
			return errors.New("unexpected end of JSON input")
		}
		return err
	}
	// Make sure there is no more data after the message
	// to approximate json.Unmarshal's behaviour.
	if dec.More() {
		return fmt.Errorf("more data after top-level value")
	}
	return nil
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
	err = jsonUnmarshalUsingNumber(msg, &obj)
	if err != nil {
		return msg, err
	}
	return json.MarshalIndent(obj, "", "    ")
}

func expectedTestResultFile(testFile string) string {
	return fmt.Sprintf("%s%s", testFile, expectedTestResultSuffix)
}
