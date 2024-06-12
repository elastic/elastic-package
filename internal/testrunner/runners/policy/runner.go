// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package policy

import (
	"context"
	"fmt"
	"strings"

	"github.com/elastic/elastic-package/internal/kibana"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/resources"
	"github.com/elastic/elastic-package/internal/testrunner"
)

const (
	TestType testrunner.TestType = "policy"
)

type runner struct {
	packageRootPath string
	kibanaClient    *kibana.Client

	dataStreams        []string
	failOnMissingTests bool
	generateTestResult bool

	resourcesManager *resources.Manager
	cleanup          func(context.Context) error
}

// Ensures that runner implements testrunner.TestRunner interface
var _ testrunner.TestRunner = new(runner)

type PolicyTestRunnerOptions struct {
	KibanaClient       *kibana.Client
	PackageRootPath    string
	DataStreams        []string
	FailOnMissingTests bool
	GenerateTestResult bool
}

func NewPolicyTestRunner(options PolicyTestRunnerOptions) *runner {
	runner := runner{
		packageRootPath:    options.PackageRootPath,
		kibanaClient:       options.KibanaClient,
		dataStreams:        options.DataStreams,
		failOnMissingTests: options.FailOnMissingTests,
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

func (r *runner) GetTests(ctx context.Context) ([]testrunner.Tester, error) {
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

	// TODO: Return a tester per each configuration file defined in the data stream folder
	var testers []testrunner.Tester
	for _, t := range folders {
		testers = append(testers, NewPolicyTester(PolicyTesterOptions{
			PackageRootPath:    r.packageRootPath,
			TestFolder:         t,
			KibanaClient:       r.kibanaClient,
			GenerateTestResult: r.generateTestResult,
		}))
	}
	return testers, nil
}

func (r *runner) Type() testrunner.TestType {
	return TestType
}
