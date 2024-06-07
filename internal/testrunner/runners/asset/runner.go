// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package asset

import (
	"context"

	"github.com/elastic/elastic-package/internal/testrunner"
)

const (
	// TestType defining asset loading tests
	TestType testrunner.TestType = "asset"
)

type runner struct {
	packageRootPath string
}

type AssetTestRunnerOptions struct {
	PackageRootPath string
}

func NewAssetTestRunner(options AssetTestRunnerOptions) *runner {
	runner := runner{
		packageRootPath: options.PackageRootPath,
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

func (r *runner) GetTests(ctx context.Context) ([]testrunner.TestFolder, error) {
	tests := []testrunner.TestFolder{{Package: r.packageRootPath}}
	return tests, nil
}

func (r *runner) Type() testrunner.TestType {
	return TestType
}
