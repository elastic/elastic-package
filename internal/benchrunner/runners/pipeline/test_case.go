// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package pipeline

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/elastic/elastic-package/internal/common"

	"github.com/pkg/errors"
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
	err := jsonUnmarshalUsingNumber(inputData, &tcd)
	if err != nil {
		return nil, errors.Wrap(err, "unmarshalling input data failed")
	}
	return tcd.Events, nil
}

func readBenchmarkEntriesForRawInput(inputData []byte) ([]json.RawMessage, error) {
	entries, err := readRawInputEntries(inputData)
	if err != nil {
		return nil, errors.Wrap(err, "reading raw input entries failed")
	}

	var events []json.RawMessage
	for _, entry := range entries {
		event := map[string]interface{}{}
		event["message"] = entry

		m, err := json.Marshal(&event)
		if err != nil {
			return nil, errors.Wrap(err, "marshalling mocked event failed")
		}
		events = append(events, m)
	}
	return events, nil
}

func createBenchmark(entries []json.RawMessage, config *config) (*benchmark, error) {
	var events []json.RawMessage
	for _, entry := range entries {
		var m common.MapStr
		err := jsonUnmarshalUsingNumber(entry, &m)
		if err != nil {
			return nil, errors.Wrap(err, "can't unmarshal benchmark entry")
		}

		event, err := json.Marshal(&m)
		if err != nil {
			return nil, errors.Wrap(err, "marshalling event failed")
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
		return nil, errors.Wrap(err, "reading raw input benchmark file failed")
	}

	lastEntry := builder.String()
	if len(lastEntry) > 0 {
		inputDataEntries = append(inputDataEntries, lastEntry)
	}
	return inputDataEntries, nil
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
