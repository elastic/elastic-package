// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package system

import (
	"fmt"
	"time"

	"github.com/elastic/elastic-package/internal/benchrunner"
	"github.com/elastic/elastic-package/internal/benchrunner/reporters"
)

const (
	testRunMaxID = 99999
	testRunMinID = 10000
)

const (
	// BenchType defining system tests
	BenchType benchrunner.Type = "system"

	// ServiceLogsAgentDir is folder path where log files produced by the service
	// are stored on the Agent container's filesystem.
	ServiceLogsAgentDir = "/tmp/service_logs"

	waitForDataDefaultTimeout = 10 * time.Minute
)

type runner struct {
	options  Options
	scenario *scenario
}

func NewSystemBenchmark(opts Options) benchrunner.Runner {
	return &runner{options: opts}
}

// Type returns the type of test that can be run by this test runner.
func (r *runner) Type() benchrunner.Type {
	return BenchType
}

// String returns the human-friendly name of the test runner.
func (r *runner) String() string {
	return string(BenchType)
}

// Run runs the system tests defined under the given folder
func (r *runner) Run() (reporters.Reportable, error) {
	return r.run()
}

func (r *runner) SetUp() error {
	scenario, err := readConfig(r.options.PackageRootPath, r.options.BenchName)
	if err != nil {
		return err
	}
	r.scenario = scenario
	fmt.Println(r.scenario)
	return nil
}

// TearDown method doesn't perform any global action as the "tear down" is executed per test case.
func (r *runner) TearDown() error {
	return nil
}

func (r *runner) run() (report reporters.Reportable, err error) {
	return nil, nil
}
