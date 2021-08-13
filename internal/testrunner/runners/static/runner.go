// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package static

import (
	"os"
	"path/filepath"

	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/fields"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/testrunner"
)

const sampleEventJSON = "sample_event.json"

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

func (r runner) run() ([]testrunner.TestResult, error) {
	result := testrunner.NewResultComposer(testrunner.TestResult{
		TestType:   TestType,
		Package:    r.options.TestFolder.Package,
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
	dataStreamPath := filepath.Join(r.options.PackageRootPath, "data_stream", r.options.TestFolder.DataStream)
	sampleEventPath := filepath.Join(dataStreamPath, sampleEventJSON)
	_, err := os.Stat(sampleEventPath)
	if errors.Is(err, os.ErrNotExist) {
		return []testrunner.TestResult{} // nothing to succeed, nothing to skip
	}

	resultComposer := testrunner.NewResultComposer(testrunner.TestResult{
		Name:       "Verify " + sampleEventJSON,
		TestType:   TestType,
		Package:    r.options.TestFolder.Package,
		DataStream: r.options.TestFolder.DataStream,
	})

	if err != nil {
		results, _ := resultComposer.WithError(errors.Wrap(err, "stat file failed"))
		return results
	}

	fieldsValidator, err := fields.CreateValidatorForDataStream(
		dataStreamPath,
		fields.WithDefaultNumericConversion())
	if err != nil {
		results, _ := resultComposer.WithError(errors.Wrap(err, "creating fields validator for data stream failed"))
		return results
	}

	content, err := os.ReadFile(sampleEventPath)
	if err != nil {
		results, _ := resultComposer.WithError(errors.Wrap(err, "can't read file"))
		return results
	}

	multiErr := fieldsValidator.ValidateDocumentBody(content)
	if len(multiErr) > 0 {
		results, _ := resultComposer.WithError(testrunner.ErrTestCaseFailed{
			Reason:  "one or more errors found in document",
			Details: multiErr.Error(),
		})
		return results
	}

	results, _ := resultComposer.WithSuccess()
	return results
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
