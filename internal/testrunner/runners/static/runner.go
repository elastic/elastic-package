// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package static

import (
	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/testrunner"
)

type runner struct {
	options testrunner.TestOptions
}

var _ testrunner.TestRunner = new(runner)

func init() {
	testrunner.RegisterRunner(&runner{})
}

const (
	// TestType defining asset loading tests
	TestType testrunner.TestType = "static"
)

func (r runner) Type() testrunner.TestType {
	return TestType
}

func (r runner) String() string {
	return "static files"
}

func (r runner) Run(options testrunner.TestOptions) ([]testrunner.TestResult, error) {
	r.options = options
	return r.run()
}

func (r runner) run()  ([]testrunner.TestResult, error) {
	result := testrunner.NewResultComposer(testrunner.TestResult{
		TestType: TestType,
		Package:  r.options.TestFolder.Package,
		DataStream: r.options.TestFolder.DataStream,
	})

	testConfig, err := newConfig(r.options.TestFolder.Path)
	if err != nil {
		return result.WithError(errors.Wrap(err, "unable to load asset loading test config file"))
	}

	if testConfig != nil && testConfig.Skip != nil {
		logger.Warnf("skipping %s test for %s: %s (details: %s)",
			TestType, r.options.TestFolder.Package,
			testConfig.Skip.Reason, testConfig.Skip.Link.String())
		return result.WithSkip(testConfig.Skip)
	}

	var results []testrunner.TestResult
	results = append(results, r.verifySampleEvent()...)
	return results, nil
}

func (r runner) verifySampleEvent() []testrunner.TestResult {
	result := testrunner.NewResultComposer(testrunner.TestResult{
		TestType: TestType,
		Package:  r.options.TestFolder.Package,
		DataStream: r.options.TestFolder.DataStream,
	})
	rErr, _ := result.WithError(errors.New("foobar"))
	return rErr
}

func (r runner) TearDown() error {
	return nil // it's a static test runner, no state is stored
}

func (r runner) CanRunPerDataStream() bool {
	return true
}

func (r *runner) TestFolderRequired() bool {
	return false
}