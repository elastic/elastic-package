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

	"github.com/elastic/elastic-package/internal/common"

	"github.com/pkg/errors"
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
	err := json.Unmarshal(inputData, &tcd)
	if err != nil {
		return nil, errors.Wrap(err, "unmarshalling input data failed")
	}
	return tcd.Events, nil
}

func readTestCaseEntriesForRawInput(inputData []byte, config *testConfig) ([]json.RawMessage, error) {
	entries, err := readRawInputEntries(inputData, config)
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

func createTestCase(filename string, entries []json.RawMessage, config *testConfig) (*testCase, error) {
	var events []json.RawMessage
	for _, entry := range entries {
		var m common.MapStr
		err := json.Unmarshal(entry, &m)
		if err != nil {
			return nil, errors.Wrap(err, "can't unmarshal test case entry")
		}

		for k, v := range config.Fields {
			_, err = m.Put(k, v)
			if err != nil {
				return nil, errors.Wrap(err, "can't set custom field")
			}
		}

		event, err := json.Marshal(&m)
		if err != nil {
			return nil, errors.Wrap(err, "marshalling event failed")
		}
		events = append(events, event)
	}
	return &testCase{
		name:   filename,
		config: config,
		events: events,
	}, nil
}

type lineReader interface {
	Scan() bool
	Text() string
	Err() error
}

func readRawInputEntries(inputData []byte, c *testConfig) ([]string, error) {
	var err error
	var scanner lineReader = bufio.NewScanner(bytes.NewReader(inputData))

	// Setup multiline
	if c.Multiline != nil && c.Multiline.FirstLinePattern != "" {
		scanner, err = newMultilineReader(scanner, c.Multiline)
		if err != nil {
			return nil, errors.Wrap(err, "failed to read multiline")
		}
	}

	// Setup exclude lines
	if len(c.ExcludeLines) > 0 {
		scanner, err = newExcludePatternsReader(scanner, c.ExcludeLines)
		if err != nil {
			return nil, errors.Wrap(err, "invalid expression in exclude_lines")
		}
	}

	var inputDataEntries []string
	for scanner.Scan() {
		inputDataEntries = append(inputDataEntries, scanner.Text())
	}
	err = scanner.Err()
	if err != nil {
		return nil, errors.Wrap(err, "reading raw input test file failed")
	}

	return inputDataEntries, nil
}

type multilineReader struct {
	reader           lineReader
	firstLinePattern *regexp.Regexp

	current strings.Builder
	next    strings.Builder
}

func newMultilineReader(reader lineReader, config *multiline) (*multilineReader, error) {
	firstLinePattern, err := regexp.Compile(config.FirstLinePattern)
	if err != nil {
		return nil, err
	}
	return &multilineReader{
		reader:           reader,
		firstLinePattern: firstLinePattern,
	}, nil
}

func (r *multilineReader) Scan() (scanned bool) {
	r.current.Reset()
	if r.next.Len() > 0 {
		scanned = true
		r.current.WriteString(r.next.String())
		r.next.Reset()
	}

	for r.reader.Scan() {
		scanned = true
		text := r.reader.Text()
		if r.firstLinePattern.MatchString(text) && r.current.Len() > 0 {
			r.next.WriteString(text)
			break
		}

		if r.current.Len() > 0 {
			r.current.WriteByte('\n')
		}

		r.current.WriteString(text)
	}

	return
}

func (r *multilineReader) Text() (body string) {
	return r.current.String()
}

func (r *multilineReader) Err() error {
	return r.reader.Err()
}

type excludePatternsReader struct {
	reader   lineReader
	patterns []*regexp.Regexp

	text string
}

func newExcludePatternsReader(reader lineReader, patterns []string) (*excludePatternsReader, error) {
	compiled, err := compilePatterns(patterns)
	if err != nil {
		return nil, err
	}
	return &excludePatternsReader{
		reader:   reader,
		patterns: compiled,
	}, nil
}

func (r *excludePatternsReader) Scan() (scanned bool) {
	r.text = ""
	for r.reader.Scan() {
		text := r.reader.Text()
		if anyPatternMatch(r.patterns, text) {
			continue
		}

		r.text = text
		return true
	}
	return false
}

func (r *excludePatternsReader) Text() (body string) {
	return r.text
}

func (r *excludePatternsReader) Err() error {
	return r.reader.Err()
}

func anyPatternMatch(patterns []*regexp.Regexp, text string) bool {
	for _, pattern := range patterns {
		if pattern.MatchString(text) {
			return true
		}
	}
	return false
}

func compilePatterns(patterns []string) (regexps []*regexp.Regexp, err error) {
	for _, pattern := range patterns {
		r, err := regexp.Compile(pattern)
		if err != nil {
			return nil, err
		}
		regexps = append(regexps, r)
	}
	return
}
