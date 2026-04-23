// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package wait

import (
	"context"
	"time"
)

// PollBudget returns how many polls spaced by period are needed to cover window, rounded up
// (ceil(window/period)).
// If window or period is zero or negative, PollBudget returns 1 so callers always get a positive
// budget without dividing by zero. Otherwise the result is at least 1.
func PollBudget(window, period time.Duration) int {
	if period <= 0 || window <= 0 {
		return 1
	}
	n := int((window + period - 1) / period)
	if n < 1 {
		return 1
	}
	return n
}

// UntilTrue waits till the context is cancelled or the given function returns an error or true.
func UntilTrue(ctx context.Context, fn func(ctx context.Context) (bool, error), period, timeout time.Duration) (bool, error) {
	timeoutTimer := time.NewTimer(timeout)
	defer timeoutTimer.Stop()

	retryTicker := time.NewTicker(period)
	defer retryTicker.Stop()

	for {
		result, err := fn(ctx)
		if err != nil {
			return false, err
		}
		if result {
			return true, nil
		}

		select {
		case <-retryTicker.C:
			continue
		case <-ctx.Done():
			return false, ctx.Err()
		case <-timeoutTimer.C:
			return false, nil
		}
	}
}
