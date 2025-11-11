// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package static

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/testrunner"
)

const (
	// TestType defining asset loading tests
	TestType testrunner.TestType = "static"

	sampleEventJSON = "sample_event.json"
)

type runner struct {
	workDir            string
	packageRootPath    string
	failOnMissingTests bool
	dataStreams        []string
	globalTestConfig   testrunner.GlobalRunnerTestConfig
	withCoverage       bool
	coverageType       string
}

type StaticTestRunnerOptions struct {
	WorkDir            string
	PackageRootPath    string
	FailOnMissingTests bool
	DataStreams        []string
	GlobalTestConfig   testrunner.GlobalRunnerTestConfig
	WithCoverage       bool
	CoverageType       string
}

func NewStaticTestRunner(options StaticTestRunnerOptions) *runner {
	runner := runner{
		workDir:            options.WorkDir,
		packageRootPath:    options.PackageRootPath,
		failOnMissingTests: options.FailOnMissingTests,
		dataStreams:        options.DataStreams,
		globalTestConfig:   options.GlobalTestConfig,
		withCoverage:       options.WithCoverage,
		coverageType:       options.CoverageType,
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
	var tests []testrunner.TestFolder
	manifest, err := packages.ReadPackageManifestFromPackageRoot(r.packageRootPath)
	if err != nil {
		return nil, fmt.Errorf("reading package manifest failed (path: %s): %w", r.packageRootPath, err)
	}

	hasDataStreams, err := testrunner.PackageHasDataStreams(manifest)
	if err != nil {
		return nil, fmt.Errorf("cannot determine if package has data streams: %w", err)
	}

	if hasDataStreams {
		tests, err = testrunner.AssumeTestFolders(r.packageRootPath, r.dataStreams, r.Type())
		if err != nil {
			return nil, fmt.Errorf("unable to assume test folder paths: %w", err)
		}

		if r.failOnMissingTests && len(tests) == 0 {
			if len(r.dataStreams) > 0 {
				return nil, fmt.Errorf("no %s tests found for %s data stream(s)", r.Type(), strings.Join(r.dataStreams, ","))
			}
			return nil, fmt.Errorf("no %s tests found", r.Type())
		}
	} else {
		_, pkg := filepath.Split(r.packageRootPath)
		tests = []testrunner.TestFolder{
			{
				Package: pkg,
			},
		}
	}

	var testers []testrunner.Tester
	for _, t := range tests {
		testers = append(testers, NewStaticTester(StaticTesterOptions{
			WorkDir:          r.workDir,
			PackageRootPath:  r.packageRootPath,
			TestFolder:       t,
			GlobalTestConfig: r.globalTestConfig,
			WithCoverage:     r.withCoverage,
			CoverageType:     r.coverageType,
		}))
	}
	return testers, nil
}

func (r *runner) Type() testrunner.TestType {
	return TestType
}
