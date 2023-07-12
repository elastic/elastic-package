// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package pipeline

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/elastic/elastic-package/internal/common"
)

type testCase struct {
	name   string
	config *testConfig
	events []json.RawMessage
}

type testCaseDefinition struct {
	Events []json.RawMessage `json:"events"`
}

func readTestCaseEntriesForEvents(inputData []byte) ([]json.RawMessage, error) {
	var tcd testCaseDefinition
	err := jsonUnmarshalUsingNumber(inputData, &tcd)
	if err != nil {
		return nil, fmt.Errorf("unmarshalling input data failed: %w", err)
	}
	return tcd.Events, nil
}

func readTestCaseEntriesForRawInput(inputData []byte, config *testConfig) ([]json.RawMessage, error) {
	entries, err := readRawInputEntries(inputData, config)
	if err != nil {
		return nil, fmt.Errorf("reading raw input entries failed: %w", err)
	}

	var events []json.RawMessage
	for _, entry := range entries {
		event := map[string]interface{}{}
		event["message"] = entry

		m, err := json.Marshal(&event)
		if err != nil {
			return nil, fmt.Errorf("marshalling mocked event failed: %w", err)
		}
		events = append(events, m)
	}
	return events, nil
}

func createTestCase(filename string, entries []json.RawMessage, config *testConfig) (*testCase, error) {
	var events []json.RawMessage
	for _, entry := range entries {
		var m common.MapStr
		err := jsonUnmarshalUsingNumber(entry, &m)
		if err != nil {
			return nil, fmt.Errorf("can't unmarshal test case entry: %w", err)
		}

		for k, v := range config.Fields {
			_, err = m.Put(k, v)
			if err != nil {
				return nil, fmt.Errorf("can't set custom field: %w", err)
			}
		}

		event, err := json.Marshal(&m)
		if err != nil {
			return nil, fmt.Errorf("marshalling event failed: %w", err)
		}
		events = append(events, event)
	}
	return &testCase{
		name:   filename,
		config: config,
		events: events,
	}, nil
}

func readRawInputEntries(inputData []byte, c *testConfig) ([]string, error) {
	var inputDataEntries []string

	var builder strings.Builder
	scanner := bufio.NewScanner(bytes.NewReader(inputData))
	for scanner.Scan() {
		line := scanner.Text()

		var body string
		if c.Multiline != nil && c.Multiline.FirstLinePattern != "" {
			matched, err := regexp.MatchString(c.Multiline.FirstLinePattern, line)
			if err != nil {
				return nil, fmt.Errorf("regexp matching failed (pattern: %s): %w", c.Multiline.FirstLinePattern, err)
			}

			if matched {
				body = builder.String()
				builder.Reset()
			}
			if builder.Len() > 0 {
				builder.WriteByte('\n')
			}
			builder.WriteString(line)
			if !matched || body == "" {
				continue
			}
		} else {
			body = line
		}

		inputDataEntries = append(inputDataEntries, body)
	}
	err := scanner.Err()
	if err != nil {
		return nil, fmt.Errorf("reading raw input test file failed: %w", err)
	}

	lastEntry := builder.String()
	if len(lastEntry) > 0 {
		inputDataEntries = append(inputDataEntries, lastEntry)
	}
	return inputDataEntries, nil
}
