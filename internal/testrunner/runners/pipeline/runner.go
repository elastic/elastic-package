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
	testCaseFiles, err := r.listTestCaseFiles()
	if err != nil {
		return errors.Wrap(err, "listing test case definitions failed")
	}

	// TODO Find default pipeline
	// TODO Find all pipelines
	// TODO Render templates
	// TODO Convert yaml to json
	// TODO Install pipeline

	for _, file := range testCaseFiles {
		tc, err := r.loadTestCaseFile(file)
		if err != nil {
			return errors.Wrap(err, "loading test case failed")
		}

		fmt.Printf("Test case: %s\n", tc.name)

		for i := range tc.events {
			fmt.Printf("Event %d/%d\n", i+1, len(tc.events))
		}

		// TODO call Simulate API

		// TODO Check "generate" flag

		// TODO compare results
	}

	// TODO uninstall pipeline
	return nil
}

func (r *runner) listTestCaseFiles() ([]string, error) {
	fis, err := ioutil.ReadDir(r.testFolderPath)
	if err != nil {
		return nil, errors.Wrapf(err, "reading pipeline tests failed (path: %s)", r.testFolderPath)
	}

	var files []string
	for _, fi := range fis {
		if strings.HasSuffix(fi.Name(), expectedTestResultSuffix) || strings.HasSuffix(fi.Name(), configTestSuffix) {
			continue
		}
		files = append(files, fi.Name())
	}
	return files, nil
}

func (r *runner) loadTestCaseFile(filename string) (*testCase, error) {
	inputPath := filepath.Join(r.testFolderPath, filename)
	inputData, err := ioutil.ReadFile(inputPath)
	if err != nil {
		return nil, errors.Wrapf(err, "reading input file failed (path: %s)", inputPath)
	}

	var tc *testCase
	ext := filepath.Ext(filename)
	switch ext {
	case ".json":
		tc, err = createTestCaseForEvents(filename, inputData)
		if err != nil {
			return nil, errors.Wrapf(err, "creating test case for events failed (path: %s)", inputPath)
		}
	case ".log":
		configPath := filepath.Join(r.testFolderPath, expectedTestConfigFile(filename))
		configData, err := ioutil.ReadFile(configPath)
		if err != nil && !os.IsNotExist(err) {
			return nil, errors.Wrapf(err, "reading test config file failed (path: %s)", configPath)
		}

		tc, err = createTestCaseForRawInput(filename, inputData, configData)
		if err != nil {
			return nil, errors.Wrapf(err, "creating test case for events failed (path: %s)", inputPath)
		}
	default:
		return nil, fmt.Errorf("unsupported extension for test case file (ext: %s)", ext)
	}
	return tc, nil
}

func expectedTestConfigFile(testFile string) string {
	return fmt.Sprintf("%s%s", testFile, configTestSuffix)
}

func init() {
	testrunner.RegisterRunner(TestType, Run)
}
