package pipeline

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/testrunner"
)

const (
	// TestType defining pipeline tests
	TestType testrunner.TestType = "pipeline"

	expectedTestResultSuffix = "-expected.json"
)

type runner struct {
	testFolderPath string
}

type testCase struct {
	name string
	inputEvents []byte
	expectedResults []byte
}

// Run runs the pipeline tests defined under the given folder
func Run(testFolderPath string) error {
	r := runner{testFolderPath}
	return r.run()
}

func (r *runner) run() error {
	testCases, err := r.prepareTestCases()
	if err != nil {
		return errors.Wrap(err, "prepare test cases")
	}



	return nil
}

func (r *runner) prepareTestCases() ([]testCase, error) {
	fis, err := ioutil.ReadDir(r.testFolderPath)
	if err != nil {
		return nil, errors.Wrapf(err, "reading pipeline tests failed (path: %s)", r.testFolderPath)
	}

	var testCases []testCase
	for _, fi := range fis {
		if strings.HasSuffix(fi.Name(), expectedTestResultSuffix) {
			continue
		}

		inputPath := filepath.Join(r.testFolderPath, fi.Name())
		inputData, err := ioutil.ReadFile(inputPath)
		if err != nil {
			return nil, errors.Wrapf(err, "reading input file failed (path: %s)", inputPath)
		}

		expectedPath := filepath.Join(r.testFolderPath, expectedTestResultFile(fi.Name()))
		expectedResults, err := ioutil.ReadFile(expectedPath)
		if err != nil {
			return nil, errors.Wrapf(err, "reading expected results failed (path: %s)", expectedPath)
		}
		testCases = append(testCases, testCase{
			name:            fi.Name(),
			inputEvents:     createInputEvents(inputData),
			expectedResults: expectedResults,
		})
	}
	return testCases, nil
}

func createInputEvents(inputData []byte) []byte {

}

func expectedTestResultFile(testFile string) string {
	return fmt.Sprintf("%s%s", testFile, expectedTestResultSuffix)
}

func init() {
	testrunner.RegisterRunner(TestType, Run)
}
