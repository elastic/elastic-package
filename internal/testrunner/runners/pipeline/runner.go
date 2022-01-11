// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package pipeline

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/common"
	"github.com/elastic/elastic-package/internal/elasticsearch/ingest"
	"github.com/elastic/elastic-package/internal/fields"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/multierror"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/testrunner"
)

const (
	// TestType defining pipeline tests
	TestType testrunner.TestType = "pipeline"
)

type runner struct {
	options   testrunner.TestOptions
	pipelines []ingest.Pipeline
}

func (r *runner) TestFolderRequired() bool {
	return true
}

// Type returns the type of test that can be run by this test runner.
func (r *runner) Type() testrunner.TestType {
	return TestType
}

// String returns the human-friendly name of the test runner.
func (r *runner) String() string {
	return "pipeline"
}

// Run runs the pipeline tests defined under the given folder
func (r *runner) Run(options testrunner.TestOptions) ([]testrunner.TestResult, error) {
	r.options = options
	return r.run()
}

// TearDown shuts down the pipeline test runner.
func (r *runner) TearDown() error {
	if r.options.DeferCleanup > 0 {
		logger.Debugf("Waiting for %s before cleanup...", r.options.DeferCleanup)
		time.Sleep(r.options.DeferCleanup)
	}

	err := uninstallIngestPipelines(r.options.API, r.pipelines)
	if err != nil {
		return errors.Wrap(err, "uninstalling ingest pipelines failed")
	}
	return nil
}

// CanRunPerDataStream returns whether this test runner can run on individual
// data streams within the package.
func (r *runner) CanRunPerDataStream() bool {
	return true
}

func (r *runner) run() ([]testrunner.TestResult, error) {
	testCaseFiles, err := r.listTestCaseFiles()
	if err != nil {
		return nil, errors.Wrap(err, "listing test case definitions failed")
	}

	dataStreamPath, found, err := packages.FindDataStreamRootForPath(r.options.TestFolder.Path)
	if err != nil {
		return nil, errors.Wrap(err, "locating data_stream root failed")
	}
	if !found {
		return nil, errors.New("data stream root not found")
	}

	var entryPipeline string
	entryPipeline, r.pipelines, err = installIngestPipelines(r.options.API, dataStreamPath)
	if err != nil {
		return nil, errors.Wrap(err, "installing ingest pipelines failed")
	}

	results := make([]testrunner.TestResult, 0)
	for _, testCaseFile := range testCaseFiles {
		tr := testrunner.TestResult{
			TestType:   TestType,
			Package:    r.options.TestFolder.Package,
			DataStream: r.options.TestFolder.DataStream,
		}
		startTime := time.Now()

		tc, err := r.loadTestCaseFile(testCaseFile)
		if err != nil {
			err := errors.Wrap(err, "loading test case failed")
			tr.ErrorMsg = err.Error()
			results = append(results, tr)
			continue
		}
		tr.Name = tc.name

		if tc.config.Skip != nil {
			logger.Warnf("skipping %s test for %s/%s: %s (details: %s)",
				TestType, r.options.TestFolder.Package, r.options.TestFolder.DataStream,
				tc.config.Skip.Reason, tc.config.Skip.Link.String())

			tr.Skipped = tc.config.Skip
			results = append(results, tr)
			continue
		}

		result, err := simulatePipelineProcessing(r.options.API, entryPipeline, tc)
		if err != nil {
			err := errors.Wrap(err, "simulating pipeline processing failed")
			tr.ErrorMsg = err.Error()
			results = append(results, tr)
			continue
		}

		tr.TimeElapsed = time.Since(startTime)
		fieldsValidator, err := fields.CreateValidatorForDataStream(dataStreamPath,
			fields.WithNumericKeywordFields(tc.config.NumericKeywordFields),
			// explicitly enabled for pipeline tests only
			// since system tests can have dynamic public IPs
			fields.WithEnabledAllowedIPCheck(),
		)
		if err != nil {
			return nil, errors.Wrapf(err, "creating fields validator for data stream failed (path: %s, test case file: %s)", dataStreamPath, testCaseFile)
		}

		err = r.verifyResults(testCaseFile, tc.config, result, fieldsValidator)
		if e, ok := err.(testrunner.ErrTestCaseFailed); ok {
			tr.FailureMsg = e.Error()
			tr.FailureDetails = e.Details

			results = append(results, tr)
			continue
		}
		if err != nil {
			err := errors.Wrap(err, "verifying test result failed")
			tr.ErrorMsg = err.Error()
			results = append(results, tr)
			continue
		}

		results = append(results, tr)
	}
	return results, nil
}

func (r *runner) listTestCaseFiles() ([]string, error) {
	fis, err := os.ReadDir(r.options.TestFolder.Path)
	if err != nil {
		return nil, errors.Wrapf(err, "reading pipeline tests failed (path: %s)", r.options.TestFolder.Path)
	}

	var files []string
	for _, fi := range fis {
		if strings.HasSuffix(fi.Name(), expectedTestResultSuffix) ||
			strings.HasSuffix(fi.Name(), configTestSuffixYAML) {
			continue
		}
		files = append(files, fi.Name())
	}
	return files, nil
}

func (r *runner) loadTestCaseFile(testCaseFile string) (*testCase, error) {
	testCasePath := filepath.Join(r.options.TestFolder.Path, testCaseFile)
	testCaseData, err := os.ReadFile(testCasePath)
	if err != nil {
		return nil, errors.Wrapf(err, "reading input file failed (testCasePath: %s)", testCasePath)
	}

	config, err := readConfigForTestCase(testCasePath)
	if err != nil {
		return nil, errors.Wrapf(err, "reading config for test case failed (testCasePath: %s)", testCasePath)
	}

	if config.Skip != nil {
		return &testCase{
			name:   testCaseFile,
			config: config,
		}, nil
	}

	ext := filepath.Ext(testCaseFile)

	var entries []json.RawMessage
	switch ext {
	case ".json":
		entries, err = readTestCaseEntriesForEvents(testCaseData)
		if err != nil {
			return nil, errors.Wrapf(err, "reading test case entries for events failed (testCasePath: %s)", testCasePath)
		}
	case ".log":
		entries, err = readTestCaseEntriesForRawInput(testCaseData, config)
		if err != nil {
			return nil, errors.Wrapf(err, "creating test case entries for raw input failed (testCasePath: %s)", testCasePath)
		}
	default:
		return nil, fmt.Errorf("unsupported extension for test case file (ext: %s)", ext)
	}

	tc, err := createTestCase(testCaseFile, entries, config)
	if err != nil {
		return nil, errors.Wrapf(err, "can't create test case (testCasePath: %s)", testCasePath)
	}
	return tc, nil
}

func (r *runner) verifyResults(testCaseFile string, config *testConfig, result *testResult, fieldsValidator *fields.Validator) error {
	testCasePath := filepath.Join(r.options.TestFolder.Path, testCaseFile)

	if r.options.GenerateTestResult {
		err := writeTestResult(testCasePath, result)
		if err != nil {
			return errors.Wrap(err, "writing test result failed")
		}
	}

	err := compareResults(testCasePath, config, result)
	if _, ok := err.(testrunner.ErrTestCaseFailed); ok {
		return err
	}
	if err != nil {
		return errors.Wrap(err, "comparing test results failed")
	}

	result = stripEmptyTestResults(result)

	err = verifyDynamicFields(result, config)
	if err != nil {
		return err
	}

	err = verifyFieldsInTestResult(result, fieldsValidator)
	if err != nil {
		return err
	}
	return nil
}

// stripEmptyTestResults function removes events which are nils. These nils can represent
// documents processed by a pipeline which potentially used a "drop" processor (to drop the event at all).
func stripEmptyTestResults(result *testResult) *testResult {
	var tr testResult
	for _, event := range result.events {
		if event == nil {
			continue
		}
		tr.events = append(tr.events, event)
	}
	return &tr
}

func verifyDynamicFields(result *testResult, config *testConfig) error {
	if config == nil || config.DynamicFields == nil {
		return nil
	}

	var multiErr multierror.Error
	for _, event := range result.events {
		var m common.MapStr
		err := json.Unmarshal(event, &m)
		if err != nil {
			return errors.Wrap(err, "can't unmarshal event")
		}

		for key, pattern := range config.DynamicFields {
			val, err := m.GetValue(key)
			if err != nil && err != common.ErrKeyNotFound {
				return errors.Wrap(err, "can't remove dynamic field")
			}

			valStr, ok := val.(string)
			if !ok {
				continue // regular expressions can be verify only string values
			}

			matched, err := regexp.MatchString(pattern, valStr)
			if err != nil {
				return errors.Wrap(err, "pattern matching for dynamic field failed")
			}

			if !matched {
				multiErr = append(multiErr, fmt.Errorf("dynamic field \"%s\" doesn't match the pattern (%s): %s",
					key, pattern, valStr))
			}
		}
	}

	if len(multiErr) > 0 {
		return testrunner.ErrTestCaseFailed{
			Reason:  "one or more problems with dynamic fields found in documents",
			Details: multiErr.Unique().Error(),
		}
	}
	return nil
}

func verifyFieldsInTestResult(result *testResult, fieldsValidator *fields.Validator) error {
	var multiErr multierror.Error
	for _, event := range result.events {
		err := checkErrorMessage(event)
		if err != nil {
			multiErr = append(multiErr, err)
			continue // all fields can be wrong, no need validate them
		}

		errs := fieldsValidator.ValidateDocumentBody(event)
		if errs != nil {
			multiErr = append(multiErr, errs...)
		}
	}

	if len(multiErr) > 0 {
		return testrunner.ErrTestCaseFailed{
			Reason:  "one or more problems with fields found in documents",
			Details: multiErr.Unique().Error(),
		}
	}
	return nil
}

func checkErrorMessage(event json.RawMessage) error {
	var pipelineError = struct {
		Error struct {
			Message string
		}
	}{}
	err := json.Unmarshal(event, &pipelineError)
	if err != nil {
		return errors.Wrap(err, "can't unmarshal event to check pipeline error")
	}

	if pipelineError.Error.Message != "" {
		return fmt.Errorf("unexpected pipeline error: %s", pipelineError.Error.Message)
	}
	return nil
}

func init() {
	testrunner.RegisterRunner(&runner{})
}
