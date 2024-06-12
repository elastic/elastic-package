// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package pipeline

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/elastic/elastic-package/internal/elasticsearch"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/profile"
	"github.com/elastic/elastic-package/internal/testrunner"
)

const (
	// TestType defining pipeline tests
	TestType testrunner.TestType = "pipeline"
)

type runner struct {
	packageRootPath string
	profile         *profile.Profile
	esAPI           *elasticsearch.API
	dataStreams     []string

	failOnMissingTests bool
	generateTestResult bool

	withCoverage bool
	coverageType string
	deferCleanup time.Duration
}

type PipelineTestRunnerOptions struct {
	Profile            *profile.Profile
	PackageRootPath    string
	API                *elasticsearch.API
	DataStreams        []string
	FailOnMissingTests bool
	GenerateTestResult bool
	WithCoverage       bool
	CoverageType       string
	DeferCleanup       time.Duration
}

func NewPipelineTestRunner(options PipelineTestRunnerOptions) *runner {
	runner := runner{
		profile:            options.Profile,
		packageRootPath:    options.PackageRootPath,
		esAPI:              options.API,
		dataStreams:        options.DataStreams,
		failOnMissingTests: options.FailOnMissingTests,
		generateTestResult: options.GenerateTestResult,
		withCoverage:       options.WithCoverage,
		coverageType:       options.CoverageType,
		deferCleanup:       options.DeferCleanup,
	}
	return &runner
}

// Ensures that runner implements testrunner.TestRunner interface
var _ testrunner.TestRunner = new(runner)

// SetupRunner prepares global resources required by the test runner.
func (r *runner) SetupRunner(ctx context.Context) error {
	return nil
}

// TearDownRunner cleans up any global test runner resources. It must be called
// after the test runner has finished executing all its tests.
func (r *runner) TearDownRunner(ctx context.Context) error {
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

	// TODO: Return a tester per each configuration file defined in the data stream.
	var testers []testrunner.Tester
	for _, t := range folders {
		t, err := NewPipelineTester(PipelineTesterOptions{
			TestFolder:         t,
			PackageRootPath:    r.packageRootPath,
			GenerateTestResult: r.generateTestResult,
			WithCoverage:       r.withCoverage,
			CoverageType:       r.coverageType,
			DeferCleanup:       r.deferCleanup,
			Profile:            r.profile,
			API:                r.esAPI,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create pipeline tester: %w", err)
		}
		testers = append(testers, t)
	}
	return testers, nil
}

func (r *runner) Type() testrunner.TestType {
	return TestType
}
