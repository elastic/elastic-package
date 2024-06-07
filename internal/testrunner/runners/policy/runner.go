// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package policy

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/elastic/go-resource"

	"github.com/elastic/elastic-package/internal/kibana"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/profile"
	"github.com/elastic/elastic-package/internal/resources"
	"github.com/elastic/elastic-package/internal/testrunner"
)

const (
	TestType testrunner.TestType = "policy"
)

type tester struct {
	testFolder         testrunner.TestFolder
	packageRootPath    string
	generateTestResult bool
	kibanaClient       *kibana.Client

	resourcesManager *resources.Manager
	cleanup          func(context.Context) error
}

type runner struct {
	packageRootPath string
	kibanaClient    *kibana.Client
	profile         *profile.Profile

	dataStreams        []string
	failOnMissingTests bool

	resourcesManager *resources.Manager
	cleanup          func(context.Context) error
}

// Ensures that runner implements testrunner.Tester interface
var _ testrunner.Tester = new(tester)

// Ensures that runner implements testrunner.TestRunner interface
var _ testrunner.TestRunner = new(runner)

type PolicyTestRunnerOptions struct {
	KibanaClient       *kibana.Client
	PackageRootPath    string
	Profile            *profile.Profile
	DataStreams        []string
	FailOnMissingTests bool
}

type PolicyTesterOptions struct {
	TestFolder         testrunner.TestFolder
	KibanaClient       *kibana.Client
	PackageRootPath    string
	GenerateTestResult bool
}

func NewPolicyTestRunner(options PolicyTestRunnerOptions) *runner {
	runner := runner{
		kibanaClient:       options.KibanaClient,
		packageRootPath:    options.PackageRootPath,
		profile:            options.Profile,
		dataStreams:        options.DataStreams,
		failOnMissingTests: options.FailOnMissingTests,
	}

	runner.resourcesManager = resources.NewManager()
	runner.resourcesManager.RegisterProvider(resources.DefaultKibanaProviderName, &resources.KibanaProvider{Client: runner.kibanaClient})
	return &runner
}

func NewPolicyTester(options PolicyTesterOptions) *tester {
	tester := tester{
		kibanaClient:       options.KibanaClient,
		testFolder:         options.TestFolder,
		packageRootPath:    options.PackageRootPath,
		generateTestResult: options.GenerateTestResult,
	}
	tester.resourcesManager = resources.NewManager()
	tester.resourcesManager.RegisterProvider(resources.DefaultKibanaProviderName, &resources.KibanaProvider{Client: tester.kibanaClient})
	return &tester
}

// SetupRunner prepares global resources required by the test runner.
func (r *runner) SetupRunner(ctx context.Context) error {
	cleanup, err := r.setupSuite(ctx, r.resourcesManager)
	if err != nil {
		return fmt.Errorf("failed to setup test runner: %w", err)
	}
	r.cleanup = cleanup

	return nil
}

// TearDownRunner cleans up any global test runner resources. It must be called
// after the test runner has finished executing all its tests.
func (r *runner) TearDownRunner(ctx context.Context) error {
	logger.Debug("Uninstalling package...")
	err := r.cleanup(context.WithoutCancel(ctx))
	if err != nil {
		return fmt.Errorf("failed to clean up test runner: %w", err)
	}
	return nil
}

func (r *runner) GetTests(ctx context.Context) ([]testrunner.TestFolder, error) {
	var folders []testrunner.TestFolder
	manifest, err := packages.ReadPackageManifestFromPackageRoot(r.packageRootPath)
	if err != nil {
		return nil, fmt.Errorf("reading package manifest failed (path: %s): %w", r.packageRootPath, err)
	}

	hasDataStreams, err := testrunner.PackageHasDataStreams(manifest)
	if err != nil {
		return nil, fmt.Errorf("cannot determine if package has data streams: %w", err)
	}

	if hasDataStreams {
		var dataStreams []string
		if len(r.dataStreams) > 0 {
			dataStreams = r.dataStreams
		}

		folders, err = testrunner.FindTestFolders(r.packageRootPath, dataStreams, r.Type())
		if err != nil {
			return nil, fmt.Errorf("unable to determine test folder paths: %w", err)
		}

		if r.failOnMissingTests && len(folders) == 0 {
			if len(dataStreams) > 0 {
				return nil, fmt.Errorf("no %s tests found for %s data stream(s)", r.Type(), strings.Join(dataStreams, ","))
			}
			return nil, fmt.Errorf("no %s tests found", r.Type())
		}
	} else {
		folders, err = testrunner.FindTestFolders(r.packageRootPath, nil, r.Type())
		if err != nil {
			return nil, fmt.Errorf("unable to determine test folder paths: %w", err)
		}
		if r.failOnMissingTests && len(folders) == 0 {
			return nil, fmt.Errorf("no %s tests found", r.Type())
		}
	}

	return folders, nil
}

func (r *runner) Type() testrunner.TestType {
	return TestType
}

func (r *tester) Type() testrunner.TestType {
	return TestType
}

func (r *tester) String() string {
	return string(TestType)
}

func (r *tester) Run(ctx context.Context) ([]testrunner.TestResult, error) {
	var results []testrunner.TestResult
	tests, err := filepath.Glob(filepath.Join(r.testFolder.Path, "test-*.yml"))
	if err != nil {
		return nil, fmt.Errorf("failed to look for test files in %s: %w", r.testFolder.Path, err)
	}
	for _, test := range tests {
		result, err := r.runTest(ctx, r.resourcesManager, test)
		if err != nil {
			logger.Error(err)
		}
		results = append(results, result...)
	}

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
	policy := resources.FleetAgentPolicy{
		Name:      testName,
		Namespace: "ep",
		PackagePolicies: []resources.FleetPackagePolicy{
			{
				Name:           testName + "-" + r.testFolder.Package,
				RootPath:       r.packageRootPath,
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

	if testErr != nil {
		return result.WithError(testErr)
	}
	return result.WithSuccess()
}

func testNameFromPath(path string) string {
	ext := filepath.Ext(path)
	return strings.TrimSuffix(filepath.Base(path), ext)
}

func (r *runner) setupSuite(ctx context.Context, manager *resources.Manager) (cleanup func(ctx context.Context) error, err error) {
	packageResource := resources.FleetPackage{
		RootPath: r.packageRootPath,
	}
	setupResources := resources.Resources{
		&packageResource,
	}

	cleanup = func(ctx context.Context) error {
		packageResource.Absent = true
		_, err := manager.ApplyCtx(ctx, setupResources)
		return err
	}

	logger.Debugf("Installing package...")
	_, err = manager.ApplyCtx(ctx, setupResources)
	if err != nil {
		if ctx.Err() == nil {
			cleanupErr := cleanup(ctx)
			if cleanupErr != nil {
				return nil, fmt.Errorf("setup failed: %w (with cleanup error: %w)", err, cleanupErr)
			}
		}
		return nil, fmt.Errorf("setup failed: %w", err)
	}

	return cleanup, err
}

func (r *tester) TearDown(ctx context.Context) error {
	return nil
}
