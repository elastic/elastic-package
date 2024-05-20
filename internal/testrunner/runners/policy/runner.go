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

	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/resources"
	"github.com/elastic/elastic-package/internal/testrunner"
)

const (
	TestType testrunner.TestType = "policy"
)

func init() {
	testrunner.RegisterRunner(&runner{})
}

type runner struct{}

func (r *runner) Type() testrunner.TestType {
	return TestType
}

func (r *runner) String() string {
	return string(TestType)
}

func (r *runner) Run(ctx context.Context, options testrunner.TestOptions) ([]testrunner.TestResult, error) {
	manager := resources.NewManager()
	manager.RegisterProvider(resources.DefaultKibanaProviderName, &resources.KibanaProvider{
		Client: options.KibanaClient,
	})

	cleanup, err := r.setupSuite(ctx, manager, options)
	if err != nil {
		return nil, fmt.Errorf("failed to setup test runner: %w", err)
	}

	var results []testrunner.TestResult
	tests, err := filepath.Glob(filepath.Join(options.TestFolder.Path, "test-*.yml"))
	if err != nil {
		return nil, fmt.Errorf("failed to look for test files in %s: %w", options.TestFolder.Path, err)
	}
	for _, test := range tests {
		result, err := r.runTest(ctx, manager, options, test)
		if err != nil {
			logger.Error(err)
		}
		results = append(results, result...)
	}

	err = cleanup(context.WithoutCancel(ctx))
	if err != nil {
		return nil, fmt.Errorf("failed to clean up test runner: %w", err)
	}

	return results, nil
}

func (r *runner) runTest(ctx context.Context, manager *resources.Manager, options testrunner.TestOptions, testPath string) ([]testrunner.TestResult, error) {
	result := testrunner.NewResultComposer(testrunner.TestResult{
		TestType:   TestType,
		Name:       filepath.Base(testPath),
		Package:    options.TestFolder.Package,
		DataStream: options.TestFolder.DataStream,
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
				Name:           testName + "-" + options.TestFolder.Package,
				RootPath:       options.PackageRootPath,
				DataStreamName: options.TestFolder.DataStream,
				Vars:           testConfig.Vars,
				DataStreamVars: testConfig.DataStream.Vars,
			},
		},
	}
	resources := resource.Resources{&policy}
	_, testErr := manager.ApplyCtx(ctx, resources)
	if err == nil {
		if options.GenerateTestResult {
			testErr = dumpExpectedAgentPolicy(ctx, options, testPath, policy.ID)
		} else {
			testErr = assertExpectedAgentPolicy(ctx, options, testPath, policy.ID)
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

func (r *runner) setupSuite(ctx context.Context, manager *resources.Manager, options testrunner.TestOptions) (cleanup func(ctx context.Context) error, err error) {
	packageResource := resources.FleetPackage{
		RootPath: options.PackageRootPath,
	}
	setupResources := resources.Resources{
		&packageResource,
	}

	cleanup = func(ctx context.Context) error {
		packageResource.Absent = true
		_, err := manager.ApplyCtx(ctx, setupResources)
		return err
	}

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

func (r *runner) CanRunPerDataStream() bool {
	return true
}

func (r *runner) TestFolderRequired() bool {
	return true
}

func (r *runner) CanRunSetupTeardownIndependent() bool {
	return false
}
