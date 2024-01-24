// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package static

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/elastic/elastic-package/internal/fields"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/testrunner"
)

const sampleEventJSON = "sample_event.json"

type runner struct {
	options testrunner.TestOptions
}

// Ensures that runner implements testrunner.TestRunner interface
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
		return result.WithError(fmt.Errorf("unable to load asset loading test config file: %w", err))
	}

	if testConfig != nil && testConfig.Skip != nil {
		logger.Warnf("skipping %s test for %s: %s (details: %s)",
			TestType, r.options.TestFolder.Package,
			testConfig.Skip.Reason, testConfig.Skip.Link.String())
		return result.WithSkip(testConfig.Skip)
	}

	pkgManifest, err := packages.ReadPackageManifestFromPackageRoot(r.options.PackageRootPath)
	if err != nil {
		return result.WithError(fmt.Errorf("failed to read manifest: %w", err))
	}

	return r.verifySampleEvent(pkgManifest), nil
}

func (r runner) verifySampleEvent(pkgManifest *packages.PackageManifest) []testrunner.TestResult {
	resultComposer := testrunner.NewResultComposer(testrunner.TestResult{
		Name:       "Verify " + sampleEventJSON,
		TestType:   TestType,
		Package:    r.options.TestFolder.Package,
		DataStream: r.options.TestFolder.DataStream,
	})

	sampleEventPath, found, err := r.getSampleEventPath()
	if err != nil {
		results, _ := resultComposer.WithError(err)
		return results
	}
	if !found {
		// Nothing to do.
		return []testrunner.TestResult{}
	}

	expectedDatasets, err := r.getExpectedDatasets(pkgManifest)
	if err != nil {
		results, _ := resultComposer.WithError(err)
		return results
	}
	fieldsValidator, err := fields.CreateValidatorForDirectory(filepath.Dir(sampleEventPath),
		fields.WithSpecVersion(pkgManifest.SpecVersion),
		fields.WithDefaultNumericConversion(),
		fields.WithExpectedDatasets(expectedDatasets),
		fields.WithEnabledImportAllECSSChema(true),
	)
	if err != nil {
		results, _ := resultComposer.WithError(fmt.Errorf("creating fields validator for data stream failed: %w", err))
		return results
	}

	content, err := os.ReadFile(sampleEventPath)
	if err != nil {
		results, _ := resultComposer.WithError(fmt.Errorf("can't read file: %w", err))
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

func (r runner) getSampleEventPath() (string, bool, error) {
	var sampleEventPath string
	if r.options.TestFolder.DataStream != "" {
		sampleEventPath = filepath.Join(
			r.options.PackageRootPath,
			"data_stream",
			r.options.TestFolder.DataStream,
			sampleEventJSON)
	} else {
		sampleEventPath = filepath.Join(r.options.PackageRootPath, sampleEventJSON)
	}
	_, err := os.Stat(sampleEventPath)
	if errors.Is(err, os.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("stat file failed: %w", err)
	}
	return sampleEventPath, true, nil
}

func (r runner) getExpectedDatasets(pkgManifest *packages.PackageManifest) ([]string, error) {
	dsName := r.options.TestFolder.DataStream
	if dsName == "" {
		// TODO: This should return the package name plus the policy name, but we don't know
		// what policy created this event, so we cannot reliably know it here. Skip the check
		// by now.
		return nil, nil
	}

	dataStreamManifest, err := packages.ReadDataStreamManifestFromPackageRoot(r.options.PackageRootPath, dsName)
	if err != nil {
		return nil, fmt.Errorf("failed to read data stream manifest: %w", err)
	}
	if ds := dataStreamManifest.Dataset; ds != "" {
		return []string{ds}, nil
	}
	return []string{pkgManifest.Name + "." + dsName}, nil
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
