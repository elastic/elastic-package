// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package pipeline

import (
	"bufio"
	"bytes"
	"encoding/json"
	"regexp"
	"strings"

	"github.com/pkg/errors"
)

type testCase struct {
	name   string
	events []json.RawMessage
}

type testCaseDefinition struct {
	Events []json.RawMessage `json:"events"`
}

type multiline struct {
	FirstLinePattern string `json:"first_line_pattern"`
}

func createTestCaseForEvents(filename string, inputData []byte) (*testCase, error) {
	var tcd testCaseDefinition
	err := json.Unmarshal(inputData, &tcd)
	if err != nil {
		return nil, errors.Wrap(err, "unmarshalling input data failed")
	}
	return &testCase{
		name:   filename,
		events: tcd.Events,
	}, nil
}

func createTestCaseForRawInput(filename string, inputData []byte, config testConfig) (*testCase, error) {
	entries, err := readRawInputEntries(inputData, config)
	if err != nil {
		return nil, errors.Wrap(err, "reading raw input entries failed")
	}

	var events []json.RawMessage
	for _, entry := range entries {
		event := map[string]interface{}{}
		event["message"] = entry

		for k, v := range config.Fields {
			event[k] = v
		}

		m, err := json.Marshal(&event)
		if err != nil {
			return nil, errors.Wrap(err, "marshalling mocked event failed")
		}
		events = append(events, m)
	}
	return &testCase{
		name:   filename,
		events: events,
	}, nil
}

func readRawInputEntries(inputData []byte, c testConfig) ([]string, error) {
	var inputDataEntries []string

	var builder strings.Builder
	scanner := bufio.NewScanner(bytes.NewReader(inputData))
	for scanner.Scan() {
		line := scanner.Text()

		var body string
		if c.Multiline != nil && c.Multiline.FirstLinePattern != "" {
			matched, err := regexp.MatchString(c.Multiline.FirstLinePattern, line)
			if err != nil {
				return nil, errors.Wrapf(err, "regexp matching failed (pattern: %s)", c.Multiline.FirstLinePattern)
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
		return nil, errors.Wrap(err, "reading raw input test file failed")
	}

	lastEntry := builder.String()
	if len(lastEntry) > 0 {
		inputDataEntries = append(inputDataEntries, lastEntry)
	}
	return inputDataEntries, nil
}
