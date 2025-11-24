// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package policy

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/elastic/go-resource"

	"github.com/elastic/elastic-package/internal/kibana"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/resources"
	"github.com/elastic/elastic-package/internal/testrunner"
)

type tester struct {
	testFolder         testrunner.TestFolder
	packageRoot        string
	kibanaClient       *kibana.Client
	testPath           string
	generateTestResult bool
	globalTestConfig   testrunner.GlobalRunnerTestConfig
	withCoverage       bool
	coverageType       string

	resourcesManager *resources.Manager
}

// Ensures that runner implements testrunner.Tester interface
var _ testrunner.Tester = new(tester)

type PolicyTesterOptions struct {
	TestFolder         testrunner.TestFolder
	TestPath           string
	KibanaClient       *kibana.Client
	PackageRoot        string
	GenerateTestResult bool
	GlobalTestConfig   testrunner.GlobalRunnerTestConfig
	WithCoverage       bool
	CoverageType       string
}

func NewPolicyTester(options PolicyTesterOptions) *tester {
	tester := tester{
		kibanaClient:       options.KibanaClient,
		testFolder:         options.TestFolder,
		packageRoot:        options.PackageRoot,
		generateTestResult: options.GenerateTestResult,
		testPath:           options.TestPath,
		globalTestConfig:   options.GlobalTestConfig,
		withCoverage:       options.WithCoverage,
		coverageType:       options.CoverageType,
	}
	tester.resourcesManager = resources.NewManager()
	tester.resourcesManager.RegisterProvider(resources.DefaultKibanaProviderName, &resources.KibanaProvider{Client: tester.kibanaClient})
	return &tester
}

func (r *tester) Type() testrunner.TestType {
	return TestType
}

func (r *tester) String() string {
	return string(TestType)
}

// Parallel indicates if this tester can run in parallel or not.
func (r tester) Parallel() bool {
	// Not supported yet parallel tests even if it is indicated in the global config r.globalTestConfig
	return false
}

func (r *tester) Run(ctx context.Context) ([]testrunner.TestResult, error) {
	var results []testrunner.TestResult

	result, err := r.runTest(ctx, r.resourcesManager, r.testPath)
	if err != nil {
		logger.Error(err)
	}

	results = append(results, result...)
	return results, nil
}

func (r *tester) runTest(ctx context.Context, manager *resources.Manager, testPath string) ([]testrunner.TestResult, error) {
	result := testrunner.NewResultComposer(testrunner.TestResult{
		TestType:   TestType,
		Name:       filepath.Base(testPath),
		Package:    r.testFolder.Package,
		DataStream: r.testFolder.DataStream,
	})

	testConfig, err := readTestConfig(testPath)
	if err != nil {
		return result.WithErrorf("failed to read test config from %s: %w", testPath, err)
	}

	testName := testNameFromPath(testPath)

	if skip := testrunner.AnySkipConfig(testConfig.Skip, r.globalTestConfig.Skip); skip != nil {
		logger.Warnf("skipping %s test for %s/%s: %s (details: %s)",
			TestType, r.testFolder.Package, r.testFolder.DataStream,
			skip.Reason, skip.Link)
		return result.WithSkip(skip)
	}

	policy := resources.FleetAgentPolicy{
		Name:      testName,
		Namespace: "ep",
		PackagePolicies: []resources.FleetPackagePolicy{
			{
				Name:           testName + "-" + r.testFolder.Package,
				PackageRoot:    r.packageRoot,
				DataStreamName: r.testFolder.DataStream,
				InputName:      testConfig.Input,
				Vars:           testConfig.Vars,
				DataStreamVars: testConfig.DataStream.Vars,
			},
		},
	}
	resources := resource.Resources{&policy}
	_, testErr := manager.ApplyCtx(ctx, resources)
	if testErr == nil {
		if r.generateTestResult {
			testErr = dumpExpectedAgentPolicy(ctx, r.kibanaClient, testPath, policy.ID)
		} else {
			testErr = assertExpectedAgentPolicy(ctx, r.kibanaClient, testPath, policy.ID)
		}
	}

	// Cleanup
	policy.Absent = true
	_, err = manager.ApplyCtx(ctx, resources)
	if err != nil {
		if testErr != nil {
			return result.WithErrorf("cleanup failed with %w after test failed: %w", err, testErr)
		}
		return result.WithErrorf("cleanup failed: %w", err)
	}

	if r.withCoverage {
		coverage, err := generateCoverageReport(result.CoveragePackageName(), r.packageRoot, r.testFolder.DataStream, r.coverageType)
		if err != nil {
			return result.WithErrorf("coverage report generation failed: %w", err)
		}
		result = result.WithCoverage(coverage)
	}

	if testErr != nil {
		return result.WithError(testErr)
	}
	return result.WithSuccess()
}

// generateCoverageReport generates a coverage report that includes the manifests and template files in the package or data stream.
// TODO: For manifests, mark as covered only the variables used.
// TODO: For templates, mark as covered only the parts used, but this requires introspection in handlebars.
func generateCoverageReport(pkgName, rootPath, dataStream, coverageType string) (testrunner.CoverageReport, error) {
	dsPattern := "*"
	if dataStream != "" {
		dsPattern = dataStream
	}

	// This list of patterns includes patterns for all types of packages. It should not be a problem if some path doesn't exist.
	patterns := []string{
		filepath.Join(rootPath, "manifest.yml"),
		filepath.Join(rootPath, "agent", "input", "*.yml.hbs"),
		filepath.Join(rootPath, "data_stream", dsPattern, "manifest.yml"),
		filepath.Join(rootPath, "data_stream", dsPattern, "agent", "stream", "*.yml.hbs"),
	}

	return testrunner.GenerateBaseFileCoverageReportGlob(pkgName, patterns, coverageType, true)
}

func testNameFromPath(path string) string {
	ext := filepath.Ext(path)
	return strings.TrimSuffix(filepath.Base(path), ext)
}

func (r *tester) TearDown(ctx context.Context) error {
	return nil
}
