// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package asset

import (
	es "github.com/elastic/go-elasticsearch/v7"

	"github.com/elastic/elastic-package/internal/testrunner"
)

func init() {
	testrunner.RegisterRunner(TestType, Run)
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
}

// Run runs the asset loading tests
func Run(options testrunner.TestOptions) ([]testrunner.TestResult, error) {
	r := runner{
		testFolder:      options.TestFolder,
		packageRootPath: options.PackageRootPath,
		stackSettings:   testrunner.GetStackSettingsFromEnv(),
		esClient:        options.ESClient,
	}
	defer r.tearDown()
	return r.run()
}

func (r *runner) run() ([]testrunner.TestResult, error) {
	result := testrunner.TestResult{
		TestType:   TestType,
		Package:    r.testFolder.Package,
		DataStream: r.testFolder.DataStream,
	}
	return []testrunner.TestResult{result}, nil
}

func (r *runner) tearDown() {
}
