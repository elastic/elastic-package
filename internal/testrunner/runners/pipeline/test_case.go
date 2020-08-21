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
	name    string
	entries []testCaseEntry
}

type testCaseEntry struct {
	event    json.RawMessage
	expected json.RawMessage
}

type events struct {
	Events []json.RawMessage `json:"events"`
}

type results struct {
	Expected []json.RawMessage `json:"expected"`
}

type testConfig struct {
	Multiline *multiline             `json:"multiline"`
	Fields    map[string]interface{} `json:"fields"`
}

type multiline struct {
	FirstLinePattern string `json:"first_line_pattern"`
}

func createTestCaseEntriesForEvents(inputData, expectedResultsData []byte) ([]testCaseEntry, error) {
	var inputEvents events
	err := json.Unmarshal(inputData, &inputEvents)
	if err != nil {
		return nil, errors.Wrap(err, "unmarshalling input data failed")
	}

	var expectedResults results
	err = json.Unmarshal(expectedResultsData, &expectedResults)
	if err != nil {
		return nil, errors.Wrap(err, "unmarshalling expected results failed")
	}

	if len(inputEvents.Events) != len(expectedResults.Expected) {
		return nil, errors.New("number of input events and expected results is not equal")
	}

	var entries []testCaseEntry
	for i := range inputEvents.Events {
		entries = append(entries, testCaseEntry{
			event:    inputEvents.Events[i],
			expected: expectedResults.Expected[i],
		})
	}
	return entries, nil
}

func createTestCaseEntriesForRawInput(inputData, configData, expectedResultsData []byte) ([]testCaseEntry, error) {
	var c testConfig
	if configData != nil {
		err := json.Unmarshal(configData, &c)
		if err != nil {
			return nil, errors.Wrap(err, "unmarshalling test config failed")
		}
	}

	inputEntries, err := readRawInputEntries(inputData, c)
	if err != nil {
		return nil, errors.Wrap(err, "reading raw input entries failed")
	}

	var expectedResults results
	err = json.Unmarshal(expectedResultsData, &expectedResults)
	if err != nil {
		return nil, errors.Wrap(err, "unmarshalling expected results failed")
	}

	if len(inputEntries) != len(expectedResults.Expected) {
		return nil, errors.New("number of input events and expected results is not equal")
	}

	var inputEvents events
	for _, entry := range inputEntries {
		event := map[string]interface{}{}
		event["message"] = entry

		for k, v := range c.Fields {
			event[k] = v
		}

		m, err := json.Marshal(&event)
		if err != nil {
			return nil, errors.Wrap(err, "marshalling mocked event failed")
		}
		inputEvents.Events = append(inputEvents.Events, m)
	}

	var entries []testCaseEntry
	for i := range inputEvents.Events {
		entries = append(entries, testCaseEntry{
			event:    inputEvents.Events[i],
			expected: expectedResults.Expected[i],
		})
	}
	return entries, nil
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
	inputDataEntries = append(inputDataEntries, builder.String())

	return inputDataEntries, nil
}
