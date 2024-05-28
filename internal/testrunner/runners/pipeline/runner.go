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
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"gopkg.in/yaml.v3"

	"github.com/elastic/elastic-package/internal/common"
	"github.com/elastic/elastic-package/internal/elasticsearch/ingest"
	"github.com/elastic/elastic-package/internal/environment"
	"github.com/elastic/elastic-package/internal/fields"
	"github.com/elastic/elastic-package/internal/formatter"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/multierror"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/stack"
	"github.com/elastic/elastic-package/internal/testrunner"
)

const (
	// TestType defining pipeline tests
	TestType testrunner.TestType = "pipeline"
)

var serverlessDisableCompareResults = environment.WithElasticPackagePrefix("SERVERLESS_PIPELINE_TEST_DISABLE_COMPARE_RESULTS")

type runner struct {
	logger *slog.Logger

	options   testrunner.TestOptions
	pipelines []ingest.Pipeline

	runCompareResults bool

	provider stack.Provider
}

type IngestPipelineReroute struct {
	Description      string                               `yaml:"description"`
	Processors       []map[string]ingest.RerouteProcessor `yaml:"processors"`
	AdditionalFields map[string]interface{}               `yaml:",inline"`
}

func init() {
	testrunner.RegisterRunner(&runner{})
	testrunner.RegisterRunnerFactory(TestType, func(l *slog.Logger) testrunner.TestRunner {
		log := logger.Logger
		if l != nil {
			log = l
		}
		log = log.With(slog.String("testrunner", "pipeline"))
		return &runner{logger: log}
	})
}

// Ensures that runner implements testrunner.TestRunner interface
var _ testrunner.TestRunner = new(runner)

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
func (r *runner) Run(ctx context.Context, options testrunner.TestOptions) ([]testrunner.TestResult, error) {
	r.options = options

	stackConfig, err := stack.LoadConfig(r.options.Profile)
	if err != nil {
		return nil, err
	}

	provider, err := stack.BuildProvider(stackConfig.Provider, r.options.Profile)
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

	return r.run(ctx)
}

// TearDown shuts down the pipeline test runner.
func (r *runner) TearDown(ctx context.Context) error {
	if r.options.DeferCleanup > 0 {
		r.logger.Debug("Waiting before cleanup...", slog.Duration("duration", r.options.DeferCleanup))
		select {
		case <-time.After(r.options.DeferCleanup):
		case <-ctx.Done():
		}
	}

	if err := ingest.UninstallPipelines(ctx, r.options.API, r.pipelines); err != nil {
		return fmt.Errorf("uninstalling ingest pipelines failed: %w", err)
	}
	return nil
}

// CanRunPerDataStream returns whether this test runner can run on individual
// data streams within the package.
func (r *runner) CanRunPerDataStream() bool {
	return true
}

// CanRunSetupTeardownIndependent returns whether this test runner can run setup or
// teardown process independent.
func (r *runner) CanRunSetupTeardownIndependent() bool {
	return false
}

func (r *runner) run(ctx context.Context) ([]testrunner.TestResult, error) {
	testCaseFiles, err := r.listTestCaseFiles()
	if err != nil {
		return nil, fmt.Errorf("listing test case definitions failed: %w", err)
	}

	if r.options.API == nil {
		return nil, errors.New("missing Elasticsearch client")
	}

	dataStreamPath, found, err := packages.FindDataStreamRootForPath(r.options.TestFolder.Path)
	if err != nil {
		return nil, fmt.Errorf("locating data_stream root failed: %w", err)
	}
	if !found {
		return nil, errors.New("data stream root not found")
	}

	startTime := time.Now()
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
		validatorOptions := []fields.ValidatorOption{
			fields.WithSpecVersion(pkgManifest.SpecVersion),
			// explicitly enabled for pipeline tests only
			// since system tests can have dynamic public IPs
			fields.WithEnabledAllowedIPCheck(),
			fields.WithExpectedDatasets(expectedDatasets),
			fields.WithEnabledImportAllECSSChema(true),
		}
		result, err := r.runTestCase(ctx, testCaseFile, dataStreamPath, dsManifest.Type, entryPipeline, validatorOptions)
		if err != nil {
			return nil, err
		}
		results = append(results, result)
	}

	esLogs, err := r.checkElasticsearchLogs(ctx, startTime)
	if err != nil {
		return nil, err
	}
	results = append(results, esLogs...)

	return results, nil
}

func (r *runner) checkElasticsearchLogs(ctx context.Context, startTesting time.Time) ([]testrunner.TestResult, error) {
	startTime := time.Now()

	testingTime := startTesting.Truncate(time.Second)

	dumpOptions := stack.DumpOptions{
		Profile:  r.options.Profile,
		Services: []string{"elasticsearch"},
		Since:    testingTime,
	}
	dump, err := r.provider.Dump(ctx, dumpOptions)
	var notImplementedErr *stack.ErrNotImplemented
	if errors.As(err, &notImplementedErr) {
		r.logger.Debug("Not checking Elasticsearch logs", slog.Any("error", err))
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
		r.logger.Warn("elasticsearch warning", slog.String("message", log.Message))

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
		Name:        "(ingest pipeline warnings)",
		Package:     r.options.TestFolder.Package,
		DataStream:  r.options.TestFolder.DataStream,
		TimeElapsed: time.Since(startTime),
	}

	if totalProcessorWarnings := len(processorRelatedWarnings); totalProcessorWarnings > 0 {
		tr.FailureMsg = fmt.Sprintf("detected ingest pipeline warnings: %d", totalProcessorWarnings)
		tr.FailureDetails = strings.Join(processorRelatedWarnings, "\n")
	}

	return []testrunner.TestResult{tr}, nil

}

func (r *runner) runTestCase(ctx context.Context, testCaseFile string, dsPath string, dsType string, pipeline string, validatorOptions []fields.ValidatorOption) (testrunner.TestResult, error) {
	tr := testrunner.TestResult{
		TestType:   TestType,
		Package:    r.options.TestFolder.Package,
		DataStream: r.options.TestFolder.DataStream,
	}
	startTime := time.Now()

	tc, err := loadTestCaseFile(r.options.TestFolder.Path, testCaseFile)
	if err != nil {
		err := fmt.Errorf("loading test case failed: %w", err)
		tr.ErrorMsg = err.Error()
		return tr, nil
	}
	tr.Name = tc.name

	if tc.config.Skip != nil {
		r.logger.Warn("skipping test",
			slog.String("testType", string(TestType)),
			slog.String("data-stream", r.options.TestFolder.DataStream),
			slog.String("reason", tc.config.Skip.Reason),
			slog.String("details", tc.config.Skip.Link.String()),
		)

		tr.Skipped = tc.config.Skip
		return tr, nil
	}

	simulateDataStream := dsType + "-" + r.options.TestFolder.Package + "." + r.options.TestFolder.DataStream + "-default"
	processedEvents, err := ingest.SimulatePipeline(ctx, r.options.API, pipeline, tc.events, simulateDataStream)
	if err != nil {
		err := fmt.Errorf("simulating pipeline processing failed: %w", err)
		tr.ErrorMsg = err.Error()
		return tr, nil
	}

	result := &testResult{events: processedEvents}

	tr.TimeElapsed = time.Since(startTime)
	validatorOptions = append(slices.Clone(validatorOptions),
		fields.WithNumericKeywordFields(tc.config.NumericKeywordFields),
	)
	fieldsValidator, err := fields.CreateValidatorForDirectory(dsPath, validatorOptions...)
	if err != nil {
		return tr, fmt.Errorf("creating fields validator for data stream failed (path: %s, test case file: %s): %w", dsPath, testCaseFile, err)
	}

	err = r.verifyResults(testCaseFile, tc.config, result, fieldsValidator)
	if e, ok := err.(testrunner.ErrTestCaseFailed); ok {
		tr.FailureMsg = e.Error()
		tr.FailureDetails = e.Details
		return tr, nil
	}
	if err != nil {
		err := fmt.Errorf("verifying test result failed: %w", err)
		tr.ErrorMsg = err.Error()
		return tr, nil
	}

	if r.options.WithCoverage {
		tr.Coverage, err = GetPipelineCoverage(r.options, r.pipelines)
		if err != nil {
			return tr, fmt.Errorf("error calculating pipeline coverage: %w", err)
		}
	}

	return tr, nil
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
