package pipeline

import (
	"encoding/json"

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

func createTestCaseEntriesForRawInput(inputData, configData, expectedResults []byte) []testCaseEntry {
	return nil // TODO
}
