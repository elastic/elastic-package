// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package static

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/elastic/elastic-package/internal/benchrunner/runners/stream"
	"github.com/elastic/elastic-package/internal/fields"
	"github.com/elastic/elastic-package/internal/files"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/signal"
	"github.com/elastic/elastic-package/internal/testrunner"
)

type tester struct {
	testFolder       testrunner.TestFolder
	packageRoot      string
	globalTestConfig testrunner.GlobalRunnerTestConfig
	withCoverage     bool
	coverageType     string
	schemaURLs       fields.SchemaURLs
}

type StaticTesterOptions struct {
	TestFolder       testrunner.TestFolder
	PackageRoot      string
	GlobalTestConfig testrunner.GlobalRunnerTestConfig
	WithCoverage     bool
	CoverageType     string
	SchemaURLs       fields.SchemaURLs
}

func NewStaticTester(options StaticTesterOptions) *tester {
	runner := tester{
		testFolder:       options.TestFolder,
		packageRoot:      options.PackageRoot,
		globalTestConfig: options.GlobalTestConfig,
		withCoverage:     options.WithCoverage,
		coverageType:     options.CoverageType,
		schemaURLs:       options.SchemaURLs,
	}
	return &runner
}

// Ensures that runner implements testrunner.Tester interface
var _ testrunner.Tester = new(tester)

func (r tester) Type() testrunner.TestType {
	return TestType
}

func (r tester) String() string {
	return "static files"
}

// Parallel indicates if this tester can run in parallel or not.
func (r tester) Parallel() bool {
	// Not supported yet parallel tests even if it is indicated in the global config r.globalTestConfig
	return false
}

func (r tester) Run(ctx context.Context) ([]testrunner.TestResult, error) {
	return r.run(ctx)
}

func (r tester) run(ctx context.Context) ([]testrunner.TestResult, error) {
	result := testrunner.NewResultComposer(testrunner.TestResult{
		TestType:   TestType,
		Package:    r.testFolder.Package,
		DataStream: r.testFolder.DataStream,
	})

	testConfig, err := newConfig(r.testFolder.Path)
	if err != nil {
		return result.WithError(fmt.Errorf("unable to load asset loading test config file: %w", err))
	}

	skipConfigs := []*testrunner.SkipConfig{r.globalTestConfig.Skip}
	if testConfig != nil {
		skipConfigs = append(skipConfigs, testConfig.Skip)
	}

	if skip := testrunner.AnySkipConfig(skipConfigs...); skip != nil {
		logger.Warnf("skipping %s test for %s/%s: %s (details: %s)",
			TestType, r.testFolder.Package, r.testFolder.DataStream,
			skip.Reason, skip.Link)
		return result.WithSkip(skip)
	}

	pkgManifest, err := packages.ReadPackageManifestFromPackageRoot(r.packageRoot)
	if err != nil {
		return result.WithError(fmt.Errorf("failed to read manifest: %w", err))
	}

	// join together results from verifyStreamConfig and verifySampleEvents
	return append(r.verifyStreamConfig(ctx, r.packageRoot), r.verifySampleEvents(pkgManifest)...), nil
}

func (r tester) verifyStreamConfig(ctx context.Context, packageRoot string) []testrunner.TestResult {
	resultComposer := testrunner.NewResultComposer(testrunner.TestResult{
		Name:       "Verify benchmark config",
		TestType:   TestType,
		Package:    r.testFolder.Package,
		DataStream: r.testFolder.DataStream,
	})

	withOpts := []stream.OptionFunc{
		stream.WithPackageRoot(packageRoot),
	}

	ctx, stop := signal.Enable(ctx, logger.Info)
	defer stop()

	hasBenchmark, err := stream.StaticValidation(ctx, stream.NewOptions(withOpts...), r.testFolder.DataStream)
	if err != nil {
		results, _ := resultComposer.WithError(err)
		return results
	}

	if !hasBenchmark {
		return []testrunner.TestResult{}
	}

	results, _ := resultComposer.WithSuccess()
	return results
}

func (r tester) verifySampleEvents(pkgManifest *packages.PackageManifest) []testrunner.TestResult {
	defaultResultComposer := testrunner.NewResultComposer(testrunner.TestResult{
		Name:       "Verify " + sampleEventJSON,
		TestType:   TestType,
		Package:    r.testFolder.Package,
		DataStream: r.testFolder.DataStream,
	})

	sampleEventPaths, err := r.getSampleEventPaths()
	if err != nil {
		results, _ := defaultResultComposer.WithError(err)
		return results
	}
	if len(sampleEventPaths) == 0 {
		// Nothing to do.
		return []testrunner.TestResult{}
	}

	expectedDatasets, err := r.getExpectedDatasets(pkgManifest)
	if err != nil {
		results, _ := defaultResultComposer.WithError(err)
		return results
	}
	repositoryRoot, err := files.FindRepositoryRootFrom(r.packageRoot)
	if err != nil {
		results, _ := defaultResultComposer.WithErrorf("cannot find repository root from %s: %w", r.packageRoot, err)
		return results
	}
	defer repositoryRoot.Close()

	var allResults []testrunner.TestResult
	for _, sampleEventPath := range sampleEventPaths {
		results := r.verifySampleEvent(sampleEventPath, pkgManifest, expectedDatasets, repositoryRoot)
		allResults = append(allResults, results...)
	}
	return allResults
}

func (r tester) verifySampleEvent(sampleEventPath string, pkgManifest *packages.PackageManifest, expectedDatasets []string, repositoryRoot *os.Root) []testrunner.TestResult {
	resultComposer := testrunner.NewResultComposer(testrunner.TestResult{
		Name:       "Verify " + filepath.Base(sampleEventPath),
		TestType:   TestType,
		Package:    r.testFolder.Package,
		DataStream: r.testFolder.DataStream,
	})

	if r.withCoverage {
		coverage, err := testrunner.GenerateBaseFileCoverageReport(resultComposer.CoveragePackageName(), sampleEventPath, r.coverageType, true)
		if err != nil {
			results, _ := resultComposer.WithErrorf("coverage report generation failed: %w", err)
			return results
		}
		resultComposer = resultComposer.WithCoverage(coverage)
	}

	fieldsDir := filepath.Join(filepath.Dir(sampleEventPath), "fields")
	fieldsValidator, err := fields.CreateValidator(repositoryRoot, r.packageRoot, fieldsDir,
		fields.WithSpecVersion(pkgManifest.SpecVersion),
		fields.WithDefaultNumericConversion(),
		fields.WithExpectedDatasets(expectedDatasets),
		fields.WithEnabledImportAllECSSChema(true),
		fields.WithOTelValidation(isTestUsingOTelCollectorInput(pkgManifest)),
		fields.WithSchemaURLs(r.schemaURLs),
	)
	if err != nil {
		results, _ := resultComposer.WithError(fmt.Errorf("creating fields validator for data stream failed: %w", err))
		return results
	}

	content, err := os.ReadFile(sampleEventPath)
	if err != nil {
		results, _ := resultComposer.WithError(fmt.Errorf("can't read file: %w", err))
		return results
	}

	multiErr := fieldsValidator.ValidateDocumentBody(content)
	if len(multiErr) > 0 {
		results, _ := resultComposer.WithError(testrunner.ErrTestCaseFailed{
			Reason:  "one or more errors found in document",
			Details: multiErr.Unique().Error(),
		})
		return results
	}

	results, _ := resultComposer.WithSuccess()
	return results
}

func (r tester) getSampleEventPaths() ([]string, error) {
	var dir string
	if r.testFolder.DataStream != "" {
		dir = filepath.Join(r.packageRoot, "data_stream", r.testFolder.DataStream)
	} else {
		dir = r.packageRoot
	}
	matches, err := filepath.Glob(filepath.Join(dir, sampleEventGlob))
	if err != nil {
		return nil, fmt.Errorf("globbing for sample event files failed: %w", err)
	}
	return matches, nil
}

func (r tester) getExpectedDatasets(pkgManifest *packages.PackageManifest) ([]string, error) {
	dsName := r.testFolder.DataStream
	if dsName == "" {
		// TODO: This should return the package name plus the policy name, but we don't know
		// what policy created this event, so we cannot reliably know it here. Skip the check
		// by now.
		return nil, nil
	}

	dataStreamManifest, err := packages.ReadDataStreamManifestFromPackageRoot(r.packageRoot, dsName)
	if err != nil {
		return nil, fmt.Errorf("failed to read data stream manifest: %w", err)
	}
	if ds := dataStreamManifest.Dataset; ds != "" {
		return []string{ds}, nil
	}
	return []string{pkgManifest.Name + "." + dsName}, nil
}

func (r tester) TearDown(ctx context.Context) error {
	return nil // it's a static test runner, no state is stored
}

func isTestUsingOTelCollectorInput(manifest *packages.PackageManifest) bool {
	if manifest.Type != "input" {
		return false
	}

	// We are not testing an specific policy template here, assume this is an OTel package
	// if at least one policy template has an "otelcol" input.
	if !slices.ContainsFunc(manifest.PolicyTemplates, func(t packages.PolicyTemplate) bool {
		return t.Input == "otelcol"
	}) {
		return false
	}

	return true
}
