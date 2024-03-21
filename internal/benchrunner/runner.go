// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package benchrunner

import (
	"context"
	"errors"
	"fmt"

	"github.com/elastic/elastic-package/internal/benchrunner/reporters"
	"github.com/elastic/elastic-package/internal/logger"
)

// Type represents the various supported benchmark types
type Type string

type Runner interface {
	SetUp(context.Context) error
	Run(context.Context) (reporters.Reportable, error)
	TearDown(context.Context) error
}

// Run method delegates execution to the benchmark runner.
func Run(ctx context.Context, runner Runner) (reporters.Reportable, error) {
	if runner == nil {
		return nil, errors.New("a runner is required")
	}

	defer func() {
		tdErr := runner.TearDown(ctx)
		if tdErr != nil {
			logger.Errorf("could not teardown benchmark runner: %v", tdErr)
		}
	}()

	if err := runner.SetUp(ctx); err != nil {
		return nil, fmt.Errorf("could not set up benchmark runner: %w", err)
	}

	report, err := runner.Run(ctx)
	if err != nil {
		return nil, fmt.Errorf("could not complete benchmark run: %w", err)
	}

	return report, nil
}
