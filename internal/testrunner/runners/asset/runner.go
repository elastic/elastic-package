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
	packageRootPath  string
	kibanaClient     *kibana.Client
	globalTestConfig testrunner.GlobalRunnerTestConfig
	withCoverage     bool
	coverageType     string
	repoRoot         *os.Root
}

type AssetTestRunnerOptions struct {
	PackageRootPath  string
	KibanaClient     *kibana.Client
	GlobalTestConfig testrunner.GlobalRunnerTestConfig
	WithCoverage     bool
	CoverageType     string
	RepoRoot         *os.Root
}

func NewAssetTestRunner(options AssetTestRunnerOptions) *runner {
	runner := runner{
		packageRootPath:  options.PackageRootPath,
		kibanaClient:     options.KibanaClient,
		globalTestConfig: options.GlobalTestConfig,
		withCoverage:     options.WithCoverage,
		coverageType:     options.CoverageType,
		repoRoot:         options.RepoRoot,
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
	_, pkg := filepath.Split(r.packageRootPath)
	testers := []testrunner.Tester{
		NewAssetTester(AssetTesterOptions{
			PackageRootPath:  r.packageRootPath,
			KibanaClient:     r.kibanaClient,
			TestFolder:       testrunner.TestFolder{Package: pkg},
			GlobalTestConfig: r.globalTestConfig,
			WithCoverage:     r.withCoverage,
			CoverageType:     r.coverageType,
			RepoRoot:         r.repoRoot,
		}),
	}
	return testers, nil
}
