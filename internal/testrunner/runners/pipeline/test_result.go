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
	"strings"

	"github.com/kylelemons/godebug/diff"
	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/common"
	"github.com/elastic/elastic-package/internal/testrunner"
)

const expectedTestResultSuffix = "-expected.json"

// diffContext is the number of context lines to show for diffs in test case
// mismatches. It is the equivalent of -U in a unified diff.
const diffContext = 3

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

	report := diffUlite(string(expected), string(actual), diffContext)
	if report != "" {
		return testrunner.ErrTestCaseFailed{
			Reason:  "Expected results are different from actual ones",
			Details: report,
		}
	}
	return nil
}

// diffUlite implements a unified diff-like rendering with u lines of context.
// It differs from a complete unified diff in that it does not provide the length
// of the chunk, so it cannot be used to generate patches, but can be used for
// human inspection.
func diffUlite(a, b string, u int) string {
	chunks := diff.DiffChunks(strings.Split(a, "\n"), strings.Split(b, "\n"))
	if len(chunks) == 0 {
		return ""
	}

	buf := new(bytes.Buffer)
	fmt.Fprintf(buf, "--- want\n+++ got\n")
	gotLine := 1
	wantLine := 1

	for i, c := range chunks {
		if i == 0 && (len(c.Added) != 0 || len(c.Deleted) != 0) {
			fmt.Fprintf(buf, "@@ -%d +%d @@\n", wantLine, gotLine)
		}
		var change bool
		for _, line := range c.Added {
			change = true
			fmt.Fprintf(buf, "+ %s\n", line)
		}
		gotLine += len(c.Added)

		for _, line := range c.Deleted {
			change = true
			fmt.Fprintf(buf, "- %s\n", line)
		}
		wantLine += len(c.Deleted)

		var used int
		if change {
			used = min(len(c.Equal), u)
			for _, line := range c.Equal[:used] {
				fmt.Fprintf(buf, "  %s\n", line)
			}
		}
		if i < len(chunks)-1 {
			next := chunks[i+1]
			if len(next.Added) != 0 || len(next.Deleted) != 0 {
				off := max(used, len(c.Equal)-u)
				if off < len(c.Equal) {
					if i == 0 || 2*u < len(c.Equal) {
						fmt.Fprintf(buf, "@@ -%d +%d @@\n", wantLine+off, gotLine+off)
					}
					for _, line := range c.Equal[off:] {
						fmt.Fprintf(buf, "  %s\n", line)
					}
				}
			}
		}
		gotLine += len(c.Equal)
		wantLine += len(c.Equal)
	}
	return strings.TrimRight(buf.String(), "\n")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
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
