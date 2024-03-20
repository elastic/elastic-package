// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package pipeline

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/elastic/elastic-package/internal/common"
	"github.com/elastic/elastic-package/internal/formatter"
)

type benchmark struct {
	events []json.RawMessage
	config *config
}

type benchmarkDefinition struct {
	Events []json.RawMessage `json:"events"`
}

func readBenchmarkEntriesForEvents(inputData []byte) ([]json.RawMessage, error) {
	var tcd benchmarkDefinition
	err := formatter.JSONUnmarshalUsingNumber(inputData, &tcd)
	if err != nil {
		return nil, fmt.Errorf("unmarshalling input data failed: %w", err)
	}
	return tcd.Events, nil
}

func readBenchmarkEntriesForRawInput(inputData []byte) ([]json.RawMessage, error) {
	entries, err := readRawInputEntries(inputData)
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

func createBenchmark(entries []json.RawMessage, config *config) (*benchmark, error) {
	var events []json.RawMessage
	for _, entry := range entries {
		var m common.MapStr
		err := formatter.JSONUnmarshalUsingNumber(entry, &m)
		if err != nil {
			return nil, fmt.Errorf("can't unmarshal benchmark entry: %w", err)
		}

		event, err := json.Marshal(&m)
		if err != nil {
			return nil, fmt.Errorf("marshalling event failed: %w", err)
		}
		events = append(events, event)
	}
	return &benchmark{
		events: events,
		config: config,
	}, nil
}

func readRawInputEntries(inputData []byte) ([]string, error) {
	var inputDataEntries []string

	var builder strings.Builder
	scanner := bufio.NewScanner(bytes.NewReader(inputData))
	for scanner.Scan() {
		line := scanner.Text()
		inputDataEntries = append(inputDataEntries, line)
	}
	err := scanner.Err()
	if err != nil {
		return nil, fmt.Errorf("reading raw input benchmark file failed: %w", err)
	}

	lastEntry := builder.String()
	if len(lastEntry) > 0 {
		inputDataEntries = append(inputDataEntries, lastEntry)
	}
	return inputDataEntries, nil
}
