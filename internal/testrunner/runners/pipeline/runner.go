// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package pipeline

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"gopkg.in/yaml.v3"

	"github.com/elastic/elastic-package/internal/common"
	"github.com/elastic/elastic-package/internal/elasticsearch/ingest"
	"github.com/elastic/elastic-package/internal/environment"
	"github.com/elastic/elastic-package/internal/fields"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/multierror"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/signal"
	"github.com/elastic/elastic-package/internal/stack"
	"github.com/elastic/elastic-package/internal/testrunner"
)

const (
	// TestType defining pipeline tests
	TestType testrunner.TestType = "pipeline"
)

var (
	serverlessEnableCompareResults = environment.WithElasticPackagePrefix("SERVERLESS_PIPELINE_TEST_ENABLE_COMPARE_RESULTS")
)

type runner struct {
	options   testrunner.TestOptions
	pipelines []ingest.Pipeline

	runCompareResults bool
}

type IngestPipelineReroute struct {
	Description      string                               `yaml:"description"`
	Processors       []map[string]ingest.RerouteProcessor `yaml:"processors"`
	AdditionalFields map[string]interface{}               `yaml:",inline"`
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

	stackConfig, err := stack.LoadConfig(r.options.Profile)
	if err != nil {
		return nil, err
	}

	r.runCompareResults = true
	if stackConfig.Provider == stack.ProviderServerless {
		r.runCompareResults = false

		v, ok := os.LookupEnv(serverlessEnableCompareResults)
		if ok && strings.ToLower(v) != "false" {
			r.runCompareResults = true
		}
	}

	return r.run()
}

// TearDown shuts down the pipeline test runner.
func (r *runner) TearDown() error {
	if r.options.DeferCleanup > 0 {
		logger.Debugf("Waiting for %s before cleanup...", r.options.DeferCleanup)
		signal.Sleep(r.options.DeferCleanup)
	}

	if err := ingest.UninstallPipelines(r.options.API, r.pipelines); err != nil {
		return fmt.Errorf("uninstalling ingest pipelines failed: %w", err)
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
		return nil, fmt.Errorf("listing test case definitions failed: %w", err)
	}

	dataStreamPath, found, err := packages.FindDataStreamRootForPath(r.options.TestFolder.Path)
	if err != nil {
		return nil, fmt.Errorf("locating data_stream root failed: %w", err)
	}
	if !found {
		return nil, errors.New("data stream root not found")
	}

	var entryPipeline string
	entryPipeline, r.pipelines, err = ingest.InstallDataStreamPipelines(r.options.API, dataStreamPath)
	if err != nil {
		return nil, fmt.Errorf("installing ingest pipelines failed: %w", err)
	}

	pkgManifest, err := packages.ReadPackageManifestFromPackageRoot(r.options.PackageRootPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest: %w", err)
	}

	dsManifest, err := packages.ReadDataStreamManifestFromPackageRoot(r.options.PackageRootPath, r.options.TestFolder.DataStream)
	if err != nil {
		return nil, fmt.Errorf("failed to read data stream manifest: %w", err)
	}

	// when reroute processors are used, expectedDatasets should be set depends on the processor config
	var expectedDatasets []string
	for _, pipeline := range r.pipelines {
		var esIngestPipeline IngestPipelineReroute
		err = yaml.Unmarshal(pipeline.Content, &esIngestPipeline)
		if err != nil {
			return nil, fmt.Errorf("unmarshalling ingest pipeline content failed: %w", err)
		}
		for _, processor := range esIngestPipeline.Processors {
			if reroute, ok := processor["reroute"]; ok {
				expectedDatasets = append(expectedDatasets, reroute.Dataset...)
			}
		}
	}

	if len(expectedDatasets) == 0 {
		expectedDataset := dsManifest.Dataset
		if expectedDataset == "" {
			expectedDataset = pkgManifest.Name + "." + r.options.TestFolder.DataStream
		}
		expectedDatasets = []string{expectedDataset}
	}

	results := make([]testrunner.TestResult, 0)
	for _, testCaseFile := range testCaseFiles {
		tr := testrunner.TestResult{
			TestType:   TestType,
			Package:    r.options.TestFolder.Package,
			DataStream: r.options.TestFolder.DataStream,
		}
		startTime := time.Now()

		// TODO: Add tests to cover regressive use of json.Unmarshal in loadTestCaseFile.
		// See https://github.com/elastic/elastic-package/pull/717.
		tc, err := r.loadTestCaseFile(testCaseFile)
		if err != nil {
			err := fmt.Errorf("loading test case failed: %w", err)
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

		simulateDataStream := dsManifest.Type + "-" + r.options.TestFolder.Package + "." + r.options.TestFolder.DataStream + "-default"
		processedEvents, err := ingest.SimulatePipeline(r.options.API, entryPipeline, tc.events, simulateDataStream)
		if err != nil {
			err := fmt.Errorf("simulating pipeline processing failed: %w", err)
			tr.ErrorMsg = err.Error()
			results = append(results, tr)
			continue
		}

		result := &testResult{events: processedEvents}

		tr.TimeElapsed = time.Since(startTime)
		fieldsValidator, err := fields.CreateValidatorForDirectory(dataStreamPath,
			fields.WithSpecVersion(pkgManifest.SpecVersion),
			fields.WithNumericKeywordFields(tc.config.NumericKeywordFields),
			// explicitly enabled for pipeline tests only
			// since system tests can have dynamic public IPs
			fields.WithEnabledAllowedIPCheck(),
			fields.WithExpectedDatasets(expectedDatasets),
			fields.WithEnabledImportAllECSSChema(true),
		)
		if err != nil {
			return nil, fmt.Errorf("creating fields validator for data stream failed (path: %s, test case file: %s): %w", dataStreamPath, testCaseFile, err)
		}

		// TODO: Add tests to cover regressive use of json.Unmarshal in verifyResults.
		// See https://github.com/elastic/elastic-package/pull/717.
		err = r.verifyResults(testCaseFile, tc.config, result, fieldsValidator)
		if e, ok := err.(testrunner.ErrTestCaseFailed); ok {
			tr.FailureMsg = e.Error()
			tr.FailureDetails = e.Details

			results = append(results, tr)
			continue
		}
		if err != nil {
			err := fmt.Errorf("verifying test result failed: %w", err)
			tr.ErrorMsg = err.Error()
			results = append(results, tr)
			continue
		}

		if r.options.WithCoverage {
			tr.Coverage, err = GetPipelineCoverage(r.options, r.pipelines)
			if err != nil {
				return nil, fmt.Errorf("error calculating pipeline coverage: %w", err)
			}
		}
		results = append(results, tr)
	}

	return results, nil
}

func (r *runner) listTestCaseFiles() ([]string, error) {
	fis, err := os.ReadDir(r.options.TestFolder.Path)
	if err != nil {
		return nil, fmt.Errorf("reading pipeline tests failed (path: %s): %w", r.options.TestFolder.Path, err)
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
		return nil, fmt.Errorf("reading input file failed (testCasePath: %s): %w", testCasePath, err)
	}

	config, err := readConfigForTestCase(testCasePath)
	if err != nil {
		return nil, fmt.Errorf("reading config for test case failed (testCasePath: %s): %w", testCasePath, err)
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
			return nil, fmt.Errorf("reading test case entries for events failed (testCasePath: %s): %w", testCasePath, err)
		}
	case ".log":
		entries, err = readTestCaseEntriesForRawInput(testCaseData, config)
		if err != nil {
			return nil, fmt.Errorf("creating test case entries for raw input failed (testCasePath: %s): %w", testCasePath, err)
		}
	default:
		return nil, fmt.Errorf("unsupported extension for test case file (ext: %s)", ext)
	}

	tc, err := createTestCase(testCaseFile, entries, config)
	if err != nil {
		return nil, fmt.Errorf("can't create test case (testCasePath: %s): %w", testCasePath, err)
	}
	return tc, nil
}

func (r *runner) verifyResults(testCaseFile string, config *testConfig, result *testResult, fieldsValidator *fields.Validator) error {
	testCasePath := filepath.Join(r.options.TestFolder.Path, testCaseFile)

	manifest, err := packages.ReadPackageManifestFromPackageRoot(r.options.PackageRootPath)
	if err != nil {
		return fmt.Errorf("failed to read package manifest: %w", err)
	}
	specVersion, err := semver.NewVersion(manifest.SpecVersion)
	if err != nil {
		return fmt.Errorf("failed to parse package format version %q: %w", manifest.SpecVersion, err)
	}

	if r.options.GenerateTestResult {
		// TODO: Add tests to cover regressive use of json.Unmarshal in writeTestResult.
		// See https://github.com/elastic/elastic-package/pull/717.
		err := writeTestResult(testCasePath, result, *specVersion)
		if err != nil {
			return fmt.Errorf("writing test result failed: %w", err)
		}
	}

	// TODO: currently GeoIP related fields are being removed when the serverless provider is used.
	if r.runCompareResults {
		err = compareResults(testCasePath, config, result, *specVersion)
		if _, ok := err.(testrunner.ErrTestCaseFailed); ok {
			return err
		}
		if err != nil {
			return fmt.Errorf("comparing test results failed: %w", err)
		}
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
		err := jsonUnmarshalUsingNumber(event, &m)
		if err != nil {
			return fmt.Errorf("can't unmarshal event: %w", err)
		}

		for key, pattern := range config.DynamicFields {
			val, err := m.GetValue(key)
			if err != nil && err != common.ErrKeyNotFound {
				return fmt.Errorf("can't remove dynamic field: %w", err)
			}

			valStr, ok := val.(string)
			if !ok {
				continue // regular expressions can be verify only string values
			}

			matched, err := regexp.MatchString(pattern, valStr)
			if err != nil {
				return fmt.Errorf("pattern matching for dynamic field failed: %w", err)
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
	var pipelineError struct {
		Error struct {
			Message interface{}
		}
	}
	err := jsonUnmarshalUsingNumber(event, &pipelineError)
	if err != nil {
		return fmt.Errorf("can't unmarshal event to check pipeline error: %#q: %w", event, err)
	}

	switch m := pipelineError.Error.Message.(type) {
	case nil:
		return nil
	case string, []string:
		return fmt.Errorf("unexpected pipeline error: %s", m)
	case []interface{}:
		for i, v := range m {
			switch v.(type) {
			case string:
				break
			default:
				return fmt.Errorf("unexpected pipeline error (unexpected error.message type %T at position %d): %v", v, i, m)
			}
		}
		return fmt.Errorf("unexpected pipeline error: %s", m)
	default:
		return fmt.Errorf("unexpected pipeline error (unexpected error.message type %T): %[1]v", m)
	}
}

func init() {
	testrunner.RegisterRunner(&runner{})
}
