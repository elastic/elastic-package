package pipeline

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/testrunner"
)

const (
	// TestType defining pipeline tests
	TestType testrunner.TestType = "pipeline"

	configTestSuffix         = "-config.json"
	expectedTestResultSuffix = "-expected.json"
)

type runner struct {
	testFolderPath string
}

// Run runs the pipeline tests defined under the given folder
func Run(testFolderPath string) error {
	r := runner{testFolderPath}
	return r.run()
}

func (r *runner) run() error {
	testCases, err := r.prepareTestCases()
	if err != nil {
		return errors.Wrap(err, "preparing test cases failed")
	}

	// TODO install pipeline

	for _, tc := range testCases {
		fmt.Printf("Run test case: %s\n", tc.name)

		for i := range tc.entries {
			fmt.Printf("Process input data %d/%d\n", i+1, len(tc.entries))
		}
	}

	// TODO uninstall pipeline
	return nil
}

func (r *runner) prepareTestCases() ([]testCase, error) {
	fis, err := ioutil.ReadDir(r.testFolderPath)
	if err != nil {
		return nil, errors.Wrapf(err, "reading pipeline tests failed (path: %s)", r.testFolderPath)
	}

	var testCases []testCase
	for _, fi := range fis {
		if strings.HasSuffix(fi.Name(), expectedTestResultSuffix) || strings.HasSuffix(fi.Name(), configTestSuffix) {
			continue
		}

		inputPath := filepath.Join(r.testFolderPath, fi.Name())
		inputData, err := ioutil.ReadFile(inputPath)
		if err != nil {
			return nil, errors.Wrapf(err, "reading input file failed (path: %s)", inputPath)
		}

		expectedResultsPath := filepath.Join(r.testFolderPath, expectedTestResultFile(fi.Name()))
		expectedResults, err := ioutil.ReadFile(expectedResultsPath)
		if err != nil {
			return nil, errors.Wrapf(err, "reading expected results failed (path: %s)", expectedResultsPath)
		}

		var entries []testCaseEntry
		ext := filepath.Ext(fi.Name())
		switch ext {
		case ".json":
			entries, err = createTestCaseEntriesForEvents(inputData, expectedResults)
			if err != nil {
				return nil, errors.Wrapf(err, "creating test case entries for events failed (inputPath: %s, expectedResultsPath: %s)",
					inputPath, expectedResultsPath)
			}
		case ".log":
			configPath := filepath.Join(r.testFolderPath, expectedTestConfigFile(fi.Name()))
			configData, err := ioutil.ReadFile(configPath)
			if err != nil && !os.IsNotExist(err) {
				return nil, errors.Wrapf(err, "reading test config file failed (path: %s)", configPath)
			}
			entries, err = createTestCaseEntriesForRawInput(inputData, configData, expectedResults)
			if err != nil {
				return nil, errors.Wrapf(err, "creating test case entries for raw input failed (inputPath: %s, expectedResultsPath: %s)",
					inputPath, expectedResultsPath)
			}
		default:
			continue // unsupported test file extension
		}

		testCases = append(testCases, testCase{
			name:    fi.Name(),
			entries: entries,
		})
	}
	return testCases, nil
}

func expectedTestConfigFile(testFile string) string {
	return fmt.Sprintf("%s%s", testFile, configTestSuffix)
}

func expectedTestResultFile(testFile string) string {
	return fmt.Sprintf("%s%s", testFile, expectedTestResultSuffix)
}

func init() {
	testrunner.RegisterRunner(TestType, Run)
}
