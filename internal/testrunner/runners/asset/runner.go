// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package asset

import (
	"fmt"
	"path/filepath"
	"time"

	es "github.com/elastic/go-elasticsearch/v7"
	"github.com/pkg/errors"

	"github.com/elastic/elastic-package/internal/kibana"
	"github.com/elastic/elastic-package/internal/logger"
	"github.com/elastic/elastic-package/internal/packages"
	"github.com/elastic/elastic-package/internal/testrunner"
)

func init() {
	testrunner.RegisterRunner(&runner{})
}

const (
	// TestType defining asset loading tests
	TestType testrunner.TestType = "asset"
)

type runner struct {
	testFolder      testrunner.TestFolder
	packageRootPath string
	esClient        *es.Client

	// Execution order of following handlers is defined in runner.tearDown() method.
	removePackageHandler func() error
}

// Type returns the type of test that can be run by this test runner.
func (r *runner) Type() testrunner.TestType {
	return TestType
}

// String returns the name of the test runner.
func (r runner) String() string {
	return "asset loading"
}

// CanRunPerDataStream returns whether this test runner can run on individual
// data streams within the package.
func (r runner) CanRunPerDataStream() bool {
	return false
}

// Run runs the asset loading tests
func (r runner) Run(options testrunner.TestOptions) ([]testrunner.TestResult, error) {
	r.testFolder = options.TestFolder
	r.packageRootPath = options.PackageRootPath
	r.esClient = options.ESClient

	return r.run()
}

func (r *runner) run() ([]testrunner.TestResult, error) {
	result := testrunner.TestResult{
		TestType: TestType,
		Package:  r.testFolder.Package,
	}

	startTime := time.Now()
	resultsWith := func(tr testrunner.TestResult, err error) ([]testrunner.TestResult, error) {
		tr.TimeElapsed = time.Now().Sub(startTime)
		if err == nil {
			return []testrunner.TestResult{tr}, nil
		}

		if tcf, ok := err.(testrunner.ErrTestCaseFailed); ok {
			tr.FailureMsg = tcf.Reason
			tr.FailureDetails = tcf.Details
			return []testrunner.TestResult{tr}, nil
		}

		tr.ErrorMsg = err.Error()
		return []testrunner.TestResult{tr}, err
	}

	pkgManifest, err := packages.ReadPackageManifest(filepath.Join(r.packageRootPath, packages.PackageManifestFile))
	if err != nil {
		return resultsWith(result, errors.Wrap(err, "reading package manifest failed"))
	}

	// Install package
	kib, err := kibana.NewClient()
	if err != nil {
		return resultsWith(result, errors.Wrap(err, "could not create kibana client"))
	}

	logger.Debug("installing package...")
	actualAssets, err := kib.InstallPackage(*pkgManifest)
	if err != nil {
		return resultsWith(result, errors.Wrap(err, "could not install package"))
	}
	r.removePackageHandler = func() error {
		logger.Debug("removing package...")
		if _, err := kib.RemovePackage(*pkgManifest); err != nil {
			return errors.Wrap(err, "error cleaning up package")
		}
		return nil
	}

	expectedAssets, err := packages.LoadPackageAssets(r.packageRootPath)
	if err != nil {
		return resultsWith(result, errors.Wrap(err, "could not load expected package assets"))
	}

	results := make([]testrunner.TestResult, 0, len(expectedAssets))
	for _, e := range expectedAssets {
		result := testrunner.TestResult{
			Name:        fmt.Sprintf("%s %s is loaded", e.Type, e.ID),
			Package:     pkgManifest.Name,
			DataStream:  e.DataStream,
			TestType:    TestType,
			TimeElapsed: time.Now().Sub(startTime),
		}

		if !findActualAsset(actualAssets, e) {
			result.FailureMsg = "could not find expected asset"
			result.FailureDetails = fmt.Sprintf("could not find expected asset with ID = %s and type = %s. Assets loaded = %v", e.ID, e.Type, actualAssets)
		}

		results = append(results, result)
		startTime = time.Now()
	}

	return results, nil
}

func (r *runner) TearDown() error {
	if r.removePackageHandler != nil {
		if err := r.removePackageHandler(); err != nil {
			return err
		}
	}

	return nil
}

func findActualAsset(actualAssets []packages.Asset, expectedAsset packages.Asset) bool {
	for _, a := range actualAssets {
		if a.Type == expectedAsset.Type && a.ID == expectedAsset.ID {
			return true
		}
	}

	return false
}
