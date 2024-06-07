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
	"github.com/elastic/elastic-package/internal/resources"
	"github.com/elastic/elastic-package/internal/testrunner"
)

const (
	TestType testrunner.TestType = "policy"
)

type runner struct {
	testFolder         testrunner.TestFolder
	packageRootPath    string
	generateTestResult bool
	kibanaClient       *kibana.Client

	resourcesManager *resources.Manager
	cleanup          func(context.Context) error
}

// Ensures that runner implements testrunner.Tester interface
var _ testrunner.Tester = new(runner)

// Ensures that runner implements testrunner.TestRunner interface
var _ testrunner.TestRunner = new(runner)

type PolicyRunnerOptions struct {
	KibanaClient    *kibana.Client
	PackageRootPath string
}

type PolicyTestRunnerOptions struct {
	TestFolder         testrunner.TestFolder
	KibanaClient       *kibana.Client
	PackageRootPath    string
	GenerateTestResult bool
}

func NewPolicyRunner(options PolicyRunnerOptions) *runner {
	runner := runner{
		kibanaClient:    options.KibanaClient,
		packageRootPath: options.PackageRootPath,
	}

	runner.resourcesManager = resources.NewManager()
	runner.resourcesManager.RegisterProvider(resources.DefaultKibanaProviderName, &resources.KibanaProvider{Client: runner.kibanaClient})
	return &runner
}

func NewTestPolicyRunner(options PolicyTestRunnerOptions) *runner {
	runner := runner{
		kibanaClient:       options.KibanaClient,
		testFolder:         options.TestFolder,
		packageRootPath:    options.PackageRootPath,
		generateTestResult: options.GenerateTestResult,
	}
	runner.resourcesManager = resources.NewManager()
	runner.resourcesManager.RegisterProvider(resources.DefaultKibanaProviderName, &resources.KibanaProvider{Client: runner.kibanaClient})
	return &runner
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

func (r *runner) Type() testrunner.TestType {
	return TestType
}

func (r *runner) String() string {
	return string(TestType)
}

func (r *runner) Run(ctx context.Context) ([]testrunner.TestResult, error) {
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

func (r *runner) runTest(ctx context.Context, manager *resources.Manager, testPath string) ([]testrunner.TestResult, error) {
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

func (r *runner) TearDown(ctx context.Context) error {
	return nil
}
