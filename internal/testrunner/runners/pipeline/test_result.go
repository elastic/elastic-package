// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package pipeline

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/Masterminds/semver/v3"
	"github.com/google/go-cmp/cmp"
	"github.com/pmezard/go-difflib/difflib"

	"github.com/elastic/elastic-package/internal/common"
	"github.com/elastic/elastic-package/internal/formatter"
	"github.com/elastic/elastic-package/internal/testrunner"
)

const expectedTestResultSuffix = "-expected.json"

var geoIPKeys = []string{
	"as",
	"geo",
	"client.as",
	"client.geo",
	"destination.as",
	"destination.geo",
	"host.geo",     // not defined host.as in ECS
	"observer.geo", // not defined observer.as in ECS
	"server.as",
	"server.geo",
	"source.as",
	"source.geo",
	"threat.enrichments.indicateor.as",
	"threat.enrichments.indicateor.geo",
	"threat.indicateor.as",
	"threat.indicateor.geo",
}

type testResult struct {
	events []json.RawMessage
}

type testResultDefinition struct {
	Expected []json.RawMessage `json:"expected"`
}

func writeTestResult(testCasePath string, result *testResult, specVersion semver.Version) error {
	testCaseDir := filepath.Dir(testCasePath)
	testCaseFile := filepath.Base(testCasePath)

	data, err := marshalTestResultDefinition(result, specVersion)
	if err != nil {
		return fmt.Errorf("marshalling test result failed: %w", err)
	}
	err = os.WriteFile(filepath.Join(testCaseDir, expectedTestResultFile(testCaseFile)), data, 0644)
	if err != nil {
		return fmt.Errorf("writing test result failed: %w", err)
	}
	return nil
}

func compareResults(testCasePath string, config *testConfig, result *testResult, skipGeoip bool, specVersion semver.Version) error {
	resultsWithoutDynamicFields, err := adjustTestResult(result, config, skipGeoip)
	if err != nil {
		return fmt.Errorf("can't adjust test results: %w", err)
	}

	actual, err := marshalTestResultDefinition(resultsWithoutDynamicFields, specVersion)
	if err != nil {
		return fmt.Errorf("marshalling actual test results failed: %w", err)
	}

	expectedResults, err := readExpectedTestResult(testCasePath, config, skipGeoip)
	if err != nil {
		return fmt.Errorf("reading expected test result failed: %w", err)
	}

	expected, err := marshalTestResultDefinition(expectedResults, specVersion)
	if err != nil {
		return fmt.Errorf("marshalling expected test results failed: %w", err)
	}

	report, err := diffJson(expected, actual, specVersion)
	if err != nil {
		return fmt.Errorf("comparing expected test result: %w", err)
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

func diffJson(want, got []byte, specVersion semver.Version) (string, error) {
	var gotVal, wantVal interface{}
	err := jsonUnmarshalUsingNumber(want, &wantVal)
	if err != nil {
		return "", fmt.Errorf("invalid want value: %w", err)
	}
	err = jsonUnmarshalUsingNumber(got, &gotVal)
	if err != nil {
		return "", fmt.Errorf("invalid got value: %w", err)
	}
	if cmp.Equal(gotVal, wantVal, cmp.Comparer(compareJsonNumbers)) {
		return "", nil
	}

	got, err = marshalNormalizedJSON(gotVal, specVersion)
	if err != nil {
		return "", err
	}
	want, err = marshalNormalizedJSON(wantVal, specVersion)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	err = difflib.WriteUnifiedDiff(&buf, difflib.UnifiedDiff{
		A:        difflib.SplitLines(string(want)),
		B:        difflib.SplitLines(string(got)),
		FromFile: "want",
		ToFile:   "got",
		Context:  3,
	})
	return buf.String(), err
}

func readExpectedTestResult(testCasePath string, config *testConfig, skipGeoIP bool) (*testResult, error) {
	testCaseDir := filepath.Dir(testCasePath)
	testCaseFile := filepath.Base(testCasePath)

	path := filepath.Join(testCaseDir, expectedTestResultFile(testCaseFile))
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading test result file failed: %w", err)
	}

	u, err := unmarshalTestResult(data)
	if err != nil {
		return nil, fmt.Errorf("unmarshalling expected test result failed: %w", err)
	}

	adjusted, err := adjustTestResult(u, config, skipGeoIP)
	if err != nil {
		return nil, fmt.Errorf("adjusting test result failed: %w", err)
	}
	return adjusted, nil
}

func adjustTestResult(result *testResult, config *testConfig, skipGeoIP bool) (*testResult, error) {
	if !skipGeoIP && (config == nil || config.DynamicFields == nil) {
		return result, nil
	}
	var stripped testResult
	for _, event := range result.events {
		if event == nil {
			stripped.events = append(stripped.events, nil)
			continue
		}

		var m common.MapStr
		err := jsonUnmarshalUsingNumber(event, &m)
		if err != nil {
			return nil, fmt.Errorf("can't unmarshal event: %s: %w", string(event), err)
		}

		if config != nil && config.DynamicFields != nil {
			// Strip dynamic fields from test result
			for key := range config.DynamicFields {
				err := m.Delete(key)
				if err != nil && err != common.ErrKeyNotFound {
					return nil, fmt.Errorf("can't remove dynamic field: %w", err)
				}
			}
		}

		if skipGeoIP {
			for _, key := range geoIPKeys {
				err := m.Delete(key)
				if err != nil && err != common.ErrKeyNotFound {
					return nil, fmt.Errorf("can't remove geoIP field: %w", err)
				}
			}
		}

		b, err := json.Marshal(&m)
		if err != nil {
			return nil, fmt.Errorf("can't marshal event: %w", err)
		}

		stripped.events = append(stripped.events, b)
	}
	return &stripped, nil
}

func unmarshalTestResult(body []byte) (*testResult, error) {
	var trd testResultDefinition
	err := jsonUnmarshalUsingNumber(body, &trd)
	if err != nil {
		return nil, fmt.Errorf("unmarshalling test result failed: %w", err)
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

func marshalTestResultDefinition(result *testResult, specVersion semver.Version) ([]byte, error) {
	var trd testResultDefinition
	trd.Expected = result.events
	body, err := marshalNormalizedJSON(trd, specVersion)
	if err != nil {
		return nil, fmt.Errorf("marshalling test result definition failed: %w", err)
	}
	return body, nil
}

// marshalNormalizedJSON marshals test results ensuring that field
// order remains consistent independent of field order returned by
// ES to minimize diff noise during changes.
func marshalNormalizedJSON(v interface{}, specVersion semver.Version) ([]byte, error) {
	jsonFormatter := formatter.JSONFormatterBuilder(specVersion)
	msg, err := jsonFormatter.Encode(v)
	if err != nil {
		return msg, err
	}

	var obj interface{}
	err = jsonUnmarshalUsingNumber(msg, &obj)
	if err != nil {
		return msg, err
	}

	return jsonFormatter.Encode(obj)
}

func expectedTestResultFile(testFile string) string {
	return fmt.Sprintf("%s%s", testFile, expectedTestResultSuffix)
}
