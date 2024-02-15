// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package wait

import (
	"context"
	"time"
)

// UntilTrue waits till the context is cancelled or the given function returns an error or true.
func UntilTrue(ctx context.Context, fn func(ctx context.Context) (bool, error), timeout time.Duration) (bool, error) {
	timeoutTimer := time.NewTimer(timeout)
	defer timeoutTimer.Stop()

	retryTicker := time.NewTicker(1 * time.Second)
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
