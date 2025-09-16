// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package pipeline

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"gopkg.in/yaml.v3"

	"github.com/elastic/elastic-package/internal/common"
	"github.com/elastic/elastic-package/internal/elasticsearch"
	"github.com/elastic/elastic-package/internal/elasticsearch/ingest"
	"github.com/elastic/elastic-package/internal/environment"
	"github.com/elastic/elastic-package/internal/fields"
	"github.com/elastic/elastic-package/internal/formatter"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/multierror"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/profile"
	"github.com/elastic/elastic-package/internal/stack"
	"github.com/elastic/elastic-package/internal/testrunner"
)

var serverlessDisableCompareResults = environment.WithElasticPackagePrefix("SERVERLESS_PIPELINE_TEST_DISABLE_COMPARE_RESULTS")

type tester struct {
	profile            *profile.Profile
	deferCleanup       time.Duration
	esAPI              *elasticsearch.API
	packageRootPath    string
	testFolder         testrunner.TestFolder
	generateTestResult bool
	withCoverage       bool
	coverageType       string
	globalTestConfig   testrunner.GlobalRunnerTestConfig

	testCaseFile string

	pipelines []ingest.Pipeline

	runCompareResults bool

	provider stack.Provider
}

type PipelineTesterOptions struct {
	Profile            *profile.Profile
	DeferCleanup       time.Duration
	API                *elasticsearch.API
	PackageRootPath    string
	TestFolder         testrunner.TestFolder
	GenerateTestResult bool
	WithCoverage       bool
	CoverageType       string
	TestCaseFile       string
	GlobalTestConfig   testrunner.GlobalRunnerTestConfig
}

func NewPipelineTester(options PipelineTesterOptions) (*tester, error) {
	if options.API == nil {
		return nil, errors.New("missing Elasticsearch client")
	}

	r := tester{
		profile:            options.Profile,
		packageRootPath:    options.PackageRootPath,
		esAPI:              options.API,
		deferCleanup:       options.DeferCleanup,
		testFolder:         options.TestFolder,
		testCaseFile:       options.TestCaseFile,
		generateTestResult: options.GenerateTestResult,
		withCoverage:       options.WithCoverage,
		coverageType:       options.CoverageType,
		globalTestConfig:   options.GlobalTestConfig,
	}

	stackConfig, err := stack.LoadConfig(r.profile)
	if err != nil {
		return nil, err
	}

	provider, err := stack.BuildProvider(stackConfig.Provider, r.profile)
	if err != nil {
		return nil, fmt.Errorf("failed to build stack provider: %w", err)
	}
	r.provider = provider

	r.runCompareResults = true
	if stackConfig.Provider == stack.ProviderServerless {
		r.runCompareResults = true

		v, ok := os.LookupEnv(serverlessDisableCompareResults)
		if ok && strings.ToLower(v) == "true" {
			r.runCompareResults = false
		}
	}

	return &r, nil
}

type IngestPipelineReroute struct {
	Description      string                               `yaml:"description"`
	Processors       []map[string]ingest.RerouteProcessor `yaml:"processors"`
	AdditionalFields map[string]interface{}               `yaml:",inline"`
}

// Ensures that runner implements testrunner.Tester interface
var _ testrunner.Tester = new(tester)

// Type returns the type of test that can be run by this test runner.
func (r *tester) Type() testrunner.TestType {
	return TestType
}

// String returns the human-friendly name of the test runner.
func (r *tester) String() string {
	return "pipeline"
}

// Parallel indicates if this tester can run in parallel or not.
func (r tester) Parallel() bool {
	// Not supported yet parallel tests even if it is indicated in the global config r.globalTestConfig
	return false
}

// Run runs the pipeline tests defined under the given folder
func (r *tester) Run(ctx context.Context) ([]testrunner.TestResult, error) {
	return r.run(ctx)
}

// TearDown shuts down the pipeline test runner.
func (r *tester) TearDown(ctx context.Context) error {
	if r.deferCleanup > 0 {
		logger.Debugf("Waiting for %s before cleanup...", r.deferCleanup)
		select {
		case <-time.After(r.deferCleanup):
		case <-ctx.Done():
		}
	}

	if err := ingest.UninstallPipelines(ctx, r.esAPI, r.pipelines); err != nil {
		return fmt.Errorf("uninstalling ingest pipelines failed: %w", err)
	}
	return nil
}

func (r *tester) run(ctx context.Context) ([]testrunner.TestResult, error) {
	dataStreamPath, found, err := packages.FindDataStreamRootForPath(r.testFolder.Path)
	if err != nil {
		return nil, fmt.Errorf("locating data_stream root failed: %w", err)
	}
	if !found {
		return nil, errors.New("data stream root not found")
	}

	startTesting := time.Now()
	var entryPipeline string
	entryPipeline, r.pipelines, err = ingest.InstallDataStreamPipelines(ctx, r.esAPI, dataStreamPath)
	if err != nil {
		return nil, fmt.Errorf("installing ingest pipelines failed: %w", err)
	}

	pkgManifest, err := packages.ReadPackageManifestFromPackageRoot(r.packageRootPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest: %w", err)
	}

	dsManifest, err := packages.ReadDataStreamManifestFromPackageRoot(r.packageRootPath, r.testFolder.DataStream)
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
			expectedDataset = pkgManifest.Name + "." + r.testFolder.DataStream
		}
		expectedDatasets = []string{expectedDataset}
	}

	results := make([]testrunner.TestResult, 0)
	validatorOptions := []fields.ValidatorOption{
		fields.WithSpecVersion(pkgManifest.SpecVersion),
		// explicitly enabled for pipeline tests only
		// since system tests can have dynamic public IPs
		fields.WithEnabledAllowedIPCheck(),
		fields.WithExpectedDatasets(expectedDatasets),
		fields.WithEnabledImportAllECSSChema(true),
	}
	result, err := r.runTestCase(ctx, r.testCaseFile, dataStreamPath, dsManifest.Type, entryPipeline, validatorOptions)
	if err != nil {
		return nil, err
	}
	results = append(results, result...)

	esLogs, err := r.checkElasticsearchLogs(ctx, startTesting)
	if err != nil {
		return nil, err
	}
	results = append(results, esLogs...)

	return results, nil
}

func (r *tester) checkElasticsearchLogs(ctx context.Context, startTesting time.Time) ([]testrunner.TestResult, error) {
	startTime := time.Now()

	testingTime := startTesting.Truncate(time.Second)

	statusOptions := stack.Options{
		Profile: r.profile,
	}
	_, err := r.provider.Status(ctx, statusOptions)
	if err != nil {
		logger.Debugf("Not checking Elasticsearch logs: %s", err)
		return nil, nil
	}

	dumpOptions := stack.DumpOptions{
		Profile:  r.profile,
		Services: []string{"elasticsearch"},
		Since:    testingTime,
	}
	dump, err := r.provider.Dump(ctx, dumpOptions)
	var notImplementedErr *stack.ErrNotImplemented
	if errors.As(err, &notImplementedErr) || errors.Is(err, stack.ErrUnavailableStack) {
		logger.Debugf("Not checking Elasticsearch logs: %s", err)
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("error at getting the logs of elasticsearch: %w", err)
	}

	if len(dump) != 1 || dump[0].ServiceName != "elasticsearch" {
		return nil, errors.New("expected elasticsearch logs in dump")
	}
	elasticsearchLogs := dump[0].Logs

	seenWarnings := make(map[string]any)
	var processorRelatedWarnings []string
	err = stack.ParseLogsFromReader(bytes.NewReader(elasticsearchLogs), stack.ParseLogsOptions{
		StartTime: testingTime,
	}, func(log stack.LogLine) error {
		if log.LogLevel != "WARN" {
			return nil
		}

		if _, exists := seenWarnings[log.Message]; exists {
			return nil
		}

		seenWarnings[log.Message] = struct{}{}
		logger.Warnf("elasticsearch warning: %s", log.Message)

		// trying to catch warnings only related to processors but this is best-effort
		if strings.Contains(strings.ToLower(log.Logger), "processor") {
			processorRelatedWarnings = append(processorRelatedWarnings, log.Message)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("error at parsing logs of elasticseach: %w", err)
	}

	tr := testrunner.TestResult{
		TestType:    TestType,
		Name:        fmt.Sprintf("(ingest pipeline warnings %s)", r.testCaseFile),
		Package:     r.testFolder.Package,
		DataStream:  r.testFolder.DataStream,
		TimeElapsed: time.Since(startTime),
	}

	if totalProcessorWarnings := len(processorRelatedWarnings); totalProcessorWarnings > 0 {
		tr.FailureMsg = fmt.Sprintf("detected ingest pipeline warnings: %d", totalProcessorWarnings)
		tr.FailureDetails = strings.Join(processorRelatedWarnings, "\n")
	}

	return []testrunner.TestResult{tr}, nil
}

func (r *tester) runTestCase(ctx context.Context, testCaseFile string, dsPath string, dsType string, pipeline string, validatorOptions []fields.ValidatorOption) ([]testrunner.TestResult, error) {
	rc := testrunner.NewResultComposer(testrunner.TestResult{
		TestType:   TestType,
		Package:    r.testFolder.Package,
		DataStream: r.testFolder.DataStream,
	})
	startTime := time.Now()

	tc, err := loadTestCaseFile(r.testFolder.Path, testCaseFile)
	if err != nil {
		results, _ := rc.WithErrorf("loading test case failed: %w", err)
		return results, nil
	}
	rc.Name = tc.name

	if skip := testrunner.AnySkipConfig(tc.config.Skip, r.globalTestConfig.Skip); skip != nil {
		logger.Warnf("skipping %s test for %s/%s: %s (details: %s)",
			TestType, r.testFolder.Package, r.testFolder.DataStream,
			skip.Reason, skip.Link)
		results, _ := rc.WithSkip(skip)
		return results, nil
	}

	simulateDataStream := dsType + "-" + r.testFolder.Package + "." + r.testFolder.DataStream + "-default"
	processedEvents, err := ingest.SimulatePipeline(ctx, r.esAPI, pipeline, tc.events, simulateDataStream)
	if err != nil {
		results, _ := rc.WithErrorf("simulating pipeline processing failed: %w", err)
		return results, nil
	}

	result := &testResult{events: processedEvents}

	rc.TimeElapsed = time.Since(startTime)
	validatorOptions = append(slices.Clone(validatorOptions),
		fields.WithNumericKeywordFields(tc.config.NumericKeywordFields),
		fields.WithStringNumberFields(tc.config.StringNumberFields),
	)
	fieldsValidator, err := fields.CreateValidatorForDirectory(dsPath, validatorOptions...)
	if err != nil {
		return rc.WithErrorf("creating fields validator for data stream failed (path: %s, test case file: %s): %w", dsPath, testCaseFile, err)
	}

	err = r.verifyResults(testCaseFile, tc.config, result, fieldsValidator)
	if err != nil {
		results, _ := rc.WithErrorf("verifying test result failed: %w", err)
		return results, nil
	}

	if r.withCoverage {
		options := PipelineTesterOptions{
			TestFolder:      r.testFolder,
			API:             r.esAPI,
			PackageRootPath: r.packageRootPath,
			CoverageType:    r.coverageType,
		}
		rc.Coverage, err = getPipelineCoverage(rc.CoveragePackageName(), options, r.pipelines)
		if err != nil {
			return rc.WithErrorf("error calculating pipeline coverage: %w", err)
		}
	}

	return rc.WithSuccess()
}

func loadTestCaseFile(testFolderPath, testCaseFile string) (*testCase, error) {
	testCasePath := filepath.Join(testFolderPath, testCaseFile)
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

func (r *tester) verifyResults(testCaseFile string, config *testConfig, result *testResult, fieldsValidator *fields.Validator) error {
	testCasePath := filepath.Join(r.testFolder.Path, testCaseFile)

	manifest, err := packages.ReadPackageManifestFromPackageRoot(r.packageRootPath)
	if err != nil {
		return fmt.Errorf("failed to read package manifest: %w", err)
	}
	specVersion, err := semver.NewVersion(manifest.SpecVersion)
	if err != nil {
		return fmt.Errorf("failed to parse package format version %q: %w", manifest.SpecVersion, err)
	}

	if r.generateTestResult {
		err := writeTestResult(testCasePath, result, *specVersion)
		if err != nil {
			return fmt.Errorf("writing test result failed: %w", err)
		}
	}

	// TODO: temporary workaround until other approach for deterministic geoip in serverless can be implemented.
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
		err := formatter.JSONUnmarshalUsingNumber(event, &m)
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
	err := formatter.JSONUnmarshalUsingNumber(event, &pipelineError)
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
