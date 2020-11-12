// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package pipeline

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/common"
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
	options testrunner.TestOptions
}

// Run runs the pipeline tests defined under the given folder
func Run(options testrunner.TestOptions) ([]testrunner.TestResult, error) {
	r := runner{options}
	return r.run()
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

	entryPipeline, pipelineIDs, err := installIngestPipelines(r.options.ESClient, dataStreamPath)
	if err != nil {
		return nil, errors.Wrap(err, "installing ingest pipelines failed")
	}
	defer func() {
		err := uninstallIngestPipelines(r.options.ESClient, pipelineIDs)
		if err != nil {
			logger.Warnf("uninstalling ingest pipelines failed: %v", err)
		}
	}()

	fieldsValidator, err := fields.CreateValidatorForDataStream(dataStreamPath)
	if err != nil {
		return nil, errors.Wrapf(err, "creating fields validator for data stream failed (path: %s)", dataStreamPath)
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

		result, err := simulatePipelineProcessing(r.options.ESClient, entryPipeline, tc)
		if err != nil {
			err := errors.Wrap(err, "simulating pipeline processing failed")
			tr.ErrorMsg = err.Error()
			results = append(results, tr)
			continue
		}

		tr.TimeElapsed = time.Now().Sub(startTime)
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
	fis, err := ioutil.ReadDir(r.options.TestFolder.Path)
	if err != nil {
		return nil, errors.Wrapf(err, "reading pipeline tests failed (path: %s)", r.options.TestFolder.Path)
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

func (r *runner) loadTestCaseFile(testCaseFile string) (*testCase, error) {
	testCasePath := filepath.Join(r.options.TestFolder.Path, testCaseFile)
	testCaseData, err := ioutil.ReadFile(testCasePath)
	if err != nil {
		return nil, errors.Wrapf(err, "reading input file failed (testCasePath: %s)", testCasePath)
	}

	var tc *testCase
	ext := filepath.Ext(testCaseFile)
	switch ext {
	case ".json":
		tc, err = createTestCaseForEvents(testCaseFile, testCaseData)
		if err != nil {
			return nil, errors.Wrapf(err, "creating test case for events failed (testCasePath: %s)", testCasePath)
		}
	case ".log":
		config, err := readConfigForTestCase(testCasePath)
		if err != nil {
			return nil, errors.Wrapf(err, "reading config for test case failed (testCasePath: %s)", testCasePath)
		}
		tc, err = createTestCaseForRawInput(testCaseFile, testCaseData, config)
		if err != nil {
			return nil, errors.Wrapf(err, "creating test case for events failed (testCasePath: %s)", testCasePath)
		}
	default:
		return nil, fmt.Errorf("unsupported extension for test case file (ext: %s)", ext)
	}
	return tc, nil
}

func (r *runner) verifyResults(testCaseFile string, config *testConfig, result *testResult, fieldsValidator *fields.Validator) error {
	testCasePath := filepath.Join(r.options.TestFolder.Path, testCaseFile)

	resultWithoutDynamicFields, err := adjustTestResult(result, config)
	if err != nil {
		return errors.Wrap(err, "can't adjust test result")
	}

	if r.options.GenerateTestResult {
		err := writeTestResult(testCasePath, resultWithoutDynamicFields)
		if err != nil {
			return errors.Wrap(err, "writing test result failed")
		}
	}

	err = compareResults(testCasePath, resultWithoutDynamicFields)
	if _, ok := err.(testrunner.ErrTestCaseFailed); ok {
		return err
	}
	if err != nil {
		return errors.Wrap(err, "comparing test results failed")
	}

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
				multiErr = append(multiErr, fmt.Errorf("dynamic field \"%s\" doesn't match the pattern \"%s\": %s",
					key, pattern, valStr))
			}
		}
	}

	if len(multiErr) > 0 {
		return testrunner.ErrTestCaseFailed{
			Reason:  "one or more problems with dynamic fields found in documents",
			Details: multiErr.Error(),
		}
	}
	return nil
}

func verifyFieldsInTestResult(result *testResult, fieldsValidator *fields.Validator) error {
	var multiErr multierror.Error
	for _, event := range result.events {
		errs := fieldsValidator.ValidateDocumentBody(event)
		if errs != nil {
			multiErr = append(multiErr, errs...)
		}
	}

	if len(multiErr) > 0 {
		multiErr = multiErr.Unique()
		return testrunner.ErrTestCaseFailed{
			Reason:  "one or more problems with fields found in documents",
			Details: multiErr.Error(),
		}
	}
	return nil
}

func adjustTestResult(result *testResult, config *testConfig) (*testResult, error) {
	if config == nil || config.DynamicFields == nil {
		return result, nil
	}

	// Strip dynamic fields from test result
	var stripped testResult
	for _, event := range result.events {
		var m common.MapStr
		err := json.Unmarshal(event, &m)
		if err != nil {
			return nil, errors.Wrap(err, "can't unmarshal event")
		}

		for key := range config.DynamicFields {
			err := m.Delete(key)
			if err != nil && err != common.ErrKeyNotFound {
				return nil, errors.Wrap(err, "can't remove dynamic field")
			}
		}

		b, err := json.Marshal(&m)
		if err != nil {
			return nil, errors.Wrap(err, "can't marshal event")
		}
		stripped.events = append(stripped.events, b)
	}
	return &stripped, nil
}

func init() {
	testrunner.RegisterRunner(TestType, Run)
}
