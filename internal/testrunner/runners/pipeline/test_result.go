package pipeline

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"

	"github.com/pkg/errors"
)

const expectedTestResultSuffix = "-expected.json"

type testResult struct {
	events []json.RawMessage
}

type testResultDefinition struct {
	Expected []json.RawMessage `json:"expected"`
}

func writeTestResult(testCasePath string, result *testResult) error {
	testCaseDir := filepath.Dir(testCasePath)
	testCaseFile := filepath.Base(testCasePath)

	var trd testResultDefinition
	trd.Expected = result.events
	body, err := json.MarshalIndent(&trd, "", "    ")
	if err != nil {
		return errors.Wrap(err, "marshalling test result failed")
	}

	err = ioutil.WriteFile(filepath.Join(testCaseDir, expectedTestResultFile(testCaseFile)), body, 0644)
	if err != nil {
		return errors.Wrap(err, "writing test result failed")
	}
	return nil
}

func expectedTestResultFile(testFile string) string {
	return fmt.Sprintf("%s%s", testFile, expectedTestResultSuffix)
}
