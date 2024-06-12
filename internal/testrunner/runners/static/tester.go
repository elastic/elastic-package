// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package static

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/elastic/elastic-package/internal/benchrunner/runners/stream"
	"github.com/elastic/elastic-package/internal/fields"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/signal"
	"github.com/elastic/elastic-package/internal/testrunner"
)

type tester struct {
	testFolder      testrunner.TestFolder
	packageRootPath string
}
type StaticTesterOptions struct {
	TestFolder      testrunner.TestFolder
	PackageRootPath string
}

func NewStaticTester(options StaticTesterOptions) *tester {
	runner := tester{
		testFolder:      options.TestFolder,
		packageRootPath: options.PackageRootPath,
	}
	return &runner
}

// Ensures that runner implements testrunner.Tester interface
var _ testrunner.Tester = new(tester)

func (r tester) Type() testrunner.TestType {
	return TestType
}

func (r tester) String() string {
	return "static files"
}

// Parallel indicates if this tester can run in parallel or not.
func (r tester) Parallel() bool {
	return false
}

func (r tester) Run(ctx context.Context) ([]testrunner.TestResult, error) {
	return r.run(ctx)
}

func (r tester) run(ctx context.Context) ([]testrunner.TestResult, error) {
	result := testrunner.NewResultComposer(testrunner.TestResult{
		TestType:   TestType,
		Package:    r.testFolder.Package,
		DataStream: r.testFolder.DataStream,
	})

	testConfig, err := newConfig(r.testFolder.Path)
	if err != nil {
		return result.WithError(fmt.Errorf("unable to load asset loading test config file: %w", err))
	}

	if testConfig != nil && testConfig.Skip != nil {
		logger.Warnf("skipping %s test for %s: %s (details: %s)",
			TestType, r.testFolder.Package,
			testConfig.Skip.Reason, testConfig.Skip.Link.String())
		return result.WithSkip(testConfig.Skip)
	}

	pkgManifest, err := packages.ReadPackageManifestFromPackageRoot(r.packageRootPath)
	if err != nil {
		return result.WithError(fmt.Errorf("failed to read manifest: %w", err))
	}

	// join together results from verifyStreamConfig and verifySampleEvent
	return append(r.verifyStreamConfig(ctx, r.packageRootPath), r.verifySampleEvent(pkgManifest)...), nil
}

func (r tester) verifyStreamConfig(ctx context.Context, packageRootPath string) []testrunner.TestResult {
	resultComposer := testrunner.NewResultComposer(testrunner.TestResult{
		Name:       "Verify benchmark config",
		TestType:   TestType,
		Package:    r.testFolder.Package,
		DataStream: r.testFolder.DataStream,
	})

	withOpts := []stream.OptionFunc{
		stream.WithPackageRootPath(packageRootPath),
	}

	ctx, stop := signal.Enable(ctx, logger.Info)
	defer stop()

	hasBenchmark, err := stream.StaticValidation(ctx, stream.NewOptions(withOpts...), r.testFolder.DataStream)
	if err != nil {
		results, _ := resultComposer.WithError(err)
		return results
	}

	if !hasBenchmark {
		return []testrunner.TestResult{}
	}

	results, _ := resultComposer.WithSuccess()
	return results
}

func (r tester) verifySampleEvent(pkgManifest *packages.PackageManifest) []testrunner.TestResult {
	resultComposer := testrunner.NewResultComposer(testrunner.TestResult{
		Name:       "Verify " + sampleEventJSON,
		TestType:   TestType,
		Package:    r.testFolder.Package,
		DataStream: r.testFolder.DataStream,
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

func (r tester) getSampleEventPath() (string, bool, error) {
	var sampleEventPath string
	if r.testFolder.DataStream != "" {
		sampleEventPath = filepath.Join(
			r.packageRootPath,
			"data_stream",
			r.testFolder.DataStream,
			sampleEventJSON)
	} else {
		sampleEventPath = filepath.Join(r.packageRootPath, sampleEventJSON)
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

func (r tester) getExpectedDatasets(pkgManifest *packages.PackageManifest) ([]string, error) {
	dsName := r.testFolder.DataStream
	if dsName == "" {
		// TODO: This should return the package name plus the policy name, but we don't know
		// what policy created this event, so we cannot reliably know it here. Skip the check
		// by now.
		return nil, nil
	}

	dataStreamManifest, err := packages.ReadDataStreamManifestFromPackageRoot(r.packageRootPath, dsName)
	if err != nil {
		return nil, fmt.Errorf("failed to read data stream manifest: %w", err)
	}
	if ds := dataStreamManifest.Dataset; ds != "" {
		return []string{ds}, nil
	}
	return []string{pkgManifest.Name + "." + dsName}, nil
}

func (r tester) TearDown(ctx context.Context) error {
	return nil // it's a static test runner, no state is stored
}
