// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package asset

import (
	"context"

	"github.com/elastic/elastic-package/internal/kibana"
	"github.com/elastic/elastic-package/internal/testrunner"
)

const (
	// TestType defining asset loading tests
	TestType testrunner.TestType = "asset"
)

type runner struct {
	packageRootPath string
	kibanaClient    *kibana.Client
}

type AssetTestRunnerOptions struct {
	PackageRootPath string
	KibanaClient    *kibana.Client
}

func NewAssetTestRunner(options AssetTestRunnerOptions) *runner {
	runner := runner{
		packageRootPath: options.PackageRootPath,
		kibanaClient:    options.KibanaClient,
	}
	return &runner
}

// Ensures that runner implements testrunner.TestRunner interface
var _ testrunner.TestRunner = new(runner)

func (r *runner) SetupRunner(ctx context.Context) error {
	return nil
}

func (r *runner) TearDownRunner(ctx context.Context) error {
	return nil
}

func (r *runner) GetTests(ctx context.Context) ([]testrunner.Tester, error) {
	testers := []testrunner.Tester{
		NewAssetTester(AssetTesterOptions{
			PackageRootPath: r.packageRootPath,
			KibanaClient:    r.kibanaClient,
			TestFolder:      testrunner.TestFolder{Package: r.packageRootPath},
		}),
	}
	return testers, nil
}

func (r *runner) Type() testrunner.TestType {
	return TestType
}
