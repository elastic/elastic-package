// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package asset

import (
	"context"
	"os"
	"path/filepath"

	"github.com/elastic/elastic-package/internal/kibana"
	"github.com/elastic/elastic-package/internal/testrunner"
)

const (
	// TestType defining asset loading tests
	TestType testrunner.TestType = "asset"
)

type runner struct {
	packageRoot      string
	kibanaClient     *kibana.Client
	globalTestConfig testrunner.GlobalRunnerTestConfig
	withCoverage     bool
	coverageType     string
	repositoryRoot   *os.Root
}

type AssetTestRunnerOptions struct {
	PackageRoot      string
	KibanaClient     *kibana.Client
	GlobalTestConfig testrunner.GlobalRunnerTestConfig
	WithCoverage     bool
	CoverageType     string
	RepositoryRoot   *os.Root
}

func NewAssetTestRunner(options AssetTestRunnerOptions) *runner {
	runner := runner{
		packageRoot:      options.PackageRoot,
		kibanaClient:     options.KibanaClient,
		globalTestConfig: options.GlobalTestConfig,
		withCoverage:     options.WithCoverage,
		coverageType:     options.CoverageType,
		repositoryRoot:   options.RepositoryRoot,
	}
	return &runner
}

// Ensures that runner implements testrunner.TestRunner interface
var _ testrunner.TestRunner = new(runner)

// Type returns the type of test that can be run by this test runner.
func (r *runner) Type() testrunner.TestType {
	return TestType
}

func (r *runner) SetupRunner(ctx context.Context) error {
	return nil
}

func (r *runner) TearDownRunner(ctx context.Context) error {
	return nil
}

func (r *runner) GetTests(ctx context.Context) ([]testrunner.Tester, error) {
	_, pkg := filepath.Split(r.packageRoot)
	testers := []testrunner.Tester{
		NewAssetTester(AssetTesterOptions{
			PackageRoot:      r.packageRoot,
			KibanaClient:     r.kibanaClient,
			TestFolder:       testrunner.TestFolder{Package: pkg},
			GlobalTestConfig: r.globalTestConfig,
			WithCoverage:     r.withCoverage,
			CoverageType:     r.coverageType,
			RepositoryRoot:   r.repositoryRoot,
		}),
	}
	return testers, nil
}
