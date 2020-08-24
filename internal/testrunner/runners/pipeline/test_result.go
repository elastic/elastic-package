package pipeline

import (
	"encoding/json"
	"io/ioutil"

	"github.com/pkg/errors"
)

const expectedTestResultSuffix = "-expected.json"

type testResult struct {
	events []json.RawMessage
}

type testResultDefinition struct {
	Expected []json.RawMessage `json:"expected"`
}

func writeTestResult(testCaseFile string, result *testResult) error {
	var trd testResultDefinition
	trd.Expected = result.events
	body, err := json.Marshal(&trd)
	if err != nil {
		return errors.Wrap(err, "marshalling test result failed")
	}

	err = ioutil.WriteFile(testCaseFile, body, 0644)
	if err != nil {
		return errors.Wrap(err, "writing test result failed")
	}
	return nil
}
