// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package benchrunner

import (
	"errors"
	"fmt"

	"github.com/elastic/elastic-package/internal/benchrunner/reporters"
)

// Type represents the various supported benchmark types
type Type string

type Runner interface {
	SetUp() error
	Run() (reporters.Reportable, error)
	TearDown() error
}

// Run method delegates execution to the benchmark runner.
func Run(runner Runner) (reporters.Reportable, error) {
	if runner == nil {
		return nil, errors.New("a runner is required")
	}

	if err := runner.SetUp(); err != nil {
		return nil, fmt.Errorf("could not set up benchmark runner: %w", err)
	}

	report, err := runner.Run()
	tdErr := runner.TearDown()

	if err != nil {
		return nil, fmt.Errorf("could not complete benchmark run: %w", err)
	}

	if tdErr != nil {
		return report, fmt.Errorf("could not teardown benchmark runner: %w", tdErr)
	}

	return report, nil
}
