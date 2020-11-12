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

	"github.com/elastic/elastic-package/internal/kibana/ingestmanager"
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
	stackSettings   testrunner.StackSettings
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

// Run runs the asset loading tests
func (r runner) Run(options testrunner.TestOptions) ([]testrunner.TestResult, error) {
	r.testFolder = options.TestFolder
	r.packageRootPath = options.PackageRootPath
	r.stackSettings = testrunner.GetStackSettingsFromEnv()
	r.esClient = options.ESClient

	return r.run()
}

func (r *runner) run() ([]testrunner.TestResult, error) {
	result := testrunner.TestResult{
		TestType:   TestType,
		Package:    r.testFolder.Package,
		DataStream: r.testFolder.DataStream,
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
	im, err := ingestmanager.NewClient(r.stackSettings.Kibana.Host, r.stackSettings.Elasticsearch.Username, r.stackSettings.Elasticsearch.Password)
	if err != nil {
		return resultsWith(result, errors.Wrap(err, "could not create ingest manager client"))
	}

	logger.Debug("installing package...")
	assets, err := im.InstallPackage(*pkgManifest)
	if err != nil {
		return resultsWith(result, errors.Wrap(err, "could not install package"))
	}
	r.removePackageHandler = func() error {
		logger.Debug("removing package...")
		if _, err := im.RemovePackage(*pkgManifest); err != nil {
			return errors.Wrap(err, "error cleaning up package")
		}
		return nil
	}

	// TODO: Verify that data stream assets are loaded as expected
	fmt.Println(assets)
	// index templates
	// kibana saved objects

	return resultsWith(result, nil)
}

func (r *runner) TearDown() error {
	if r.removePackageHandler != nil {
		if err := r.removePackageHandler(); err != nil {
			return err
		}
	}

	return nil
}
